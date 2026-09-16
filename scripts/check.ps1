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
