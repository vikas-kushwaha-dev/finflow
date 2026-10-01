Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$services = @(
    "services/payment-service",
    "services/ledger-service",
    "services/gateway-service"
)

function Write-Step {
    param([string]$Message)
    Write-Host "[check] $Message"
}

function Assert-CommandSucceeded {
    param([string]$Message)

    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

Push-Location $projectRoot
try {
    Write-Step "parsing PowerShell operations scripts"
    foreach ($script in @(
        "scripts/backup-postgres.ps1",
        "scripts/verify-postgres-backup.ps1",
        "scripts/load-test.ps1",
        "scripts/analyze-postgres.ps1"
    )) {
        $tokens = $null
        $parseErrors = $null
        [System.Management.Automation.Language.Parser]::ParseFile(
            (Join-Path $projectRoot $script),
            [ref]$tokens,
            [ref]$parseErrors
        ) | Out-Null
        if ($parseErrors.Count -gt 0) {
            throw "PowerShell syntax validation failed for ${script}: $($parseErrors[0].Message)"
        }
    }

    Write-Step "validating performance workload definitions"
    foreach ($workload in @(
        "tests/performance/payment-api.js",
        "tests/performance/event-throughput.js"
    )) {
        $content = Get-Content -LiteralPath (Join-Path $projectRoot $workload) -Raw
        if ($content -notmatch "thresholds" -or $content -notmatch "http_req_failed") {
            throw "performance workload is missing failure thresholds: $workload"
        }
    }
    $queryPlanSQL = Get-Content -LiteralPath (Join-Path $projectRoot "tests/performance/postgres-query-plans.sql") -Raw
    if ($queryPlanSQL -notmatch "BEGIN TRANSACTION READ ONLY" -or $queryPlanSQL -match "(?im)^\s*(INSERT|UPDATE|DELETE|TRUNCATE)\s") {
        throw "PostgreSQL analysis workload must remain read-only"
    }

    foreach ($service in $services) {
        Write-Step "testing $service"
        Push-Location $service
        try {
            go test ./...
            Assert-CommandSucceeded "tests failed for $service"

            Write-Step "building commands in $service"
            go build ./cmd/...
            Assert-CommandSucceeded "command build failed for $service"
        }
        finally {
            Pop-Location
        }
    }

    Write-Step "validating Docker Compose configuration"
    docker compose config --quiet
    Assert-CommandSucceeded "Docker Compose validation failed"

    Write-Step "rendering Kubernetes manifests"
    kubectl kustomize infrastructure/kubernetes | Out-Null
    Assert-CommandSucceeded "Kubernetes application manifest rendering failed"
    kubectl kustomize infrastructure/kubernetes/migration | Out-Null
    Assert-CommandSucceeded "Kubernetes migration manifest rendering failed"

    Write-Step "validating release manifest rendering"
    $gatewayDigest = "sha256:" + ("a" * 64)
    $paymentDigest = "sha256:" + ("b" * 64)
    $ledgerDigest = "sha256:" + ("c" * 64)
    & "$PSScriptRoot/render-release-manifests.ps1" `
        -Registry "ghcr.io/vikas-kushwaha-dev" `
        -GatewayDigest $gatewayDigest `
        -PaymentDigest $paymentDigest `
        -LedgerDigest $ledgerDigest `
        -OutputDirectory "dist/check-release"

    if (Select-String -Path "dist/check-release/*.yaml" -Pattern "finflow/.+-service:dev" -Quiet) {
        throw "development image reference remained in release manifests"
    }

    Write-Step "all checks passed"
}
finally {
    Pop-Location
}
