param(
    [Parameter(Mandatory)]
    [string]$Registry,
    [Parameter(Mandatory)]
    [string]$GatewayDigest,
    [Parameter(Mandatory)]
    [string]$PaymentDigest,
    [Parameter(Mandatory)]
    [string]$LedgerDigest,
    [string]$OutputDirectory = "dist/release"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$digestPattern = '^sha256:[a-f0-9]{64}$'

function Assert-Digest {
    param(
        [string]$Name,
        [string]$Digest
    )

    if ($Digest.Trim() -notmatch $digestPattern) {
        throw "$Name must be a sha256 container image digest"
    }
}

function Render-Kustomization {
    param([string]$Path)

    $rendered = & kubectl kustomize $Path
    if ($LASTEXITCODE -ne 0) {
        throw "failed to render Kubernetes manifests from $Path"
    }

    return ($rendered -join [Environment]::NewLine) + [Environment]::NewLine
}

$GatewayDigest = $GatewayDigest.Trim()
$PaymentDigest = $PaymentDigest.Trim()
$LedgerDigest = $LedgerDigest.Trim()
$Registry = $Registry.Trim().TrimEnd('/')

if ($Registry -notmatch '^[a-z0-9.-]+(?::[0-9]+)?(?:/[a-z0-9._-]+)*$') {
    throw "Registry must be a lowercase container registry path"
}

Assert-Digest -Name "GatewayDigest" -Digest $GatewayDigest
Assert-Digest -Name "PaymentDigest" -Digest $PaymentDigest
Assert-Digest -Name "LedgerDigest" -Digest $LedgerDigest

$application = Render-Kustomization -Path (Join-Path $projectRoot "infrastructure/kubernetes")
$migration = Render-Kustomization -Path (Join-Path $projectRoot "infrastructure/kubernetes/migration")

$imageReferences = [ordered]@{
    "finflow/gateway-service:dev" = "$Registry/finflow-gateway-service@$GatewayDigest"
    "finflow/payment-service:dev" = "$Registry/finflow-payment-service@$PaymentDigest"
    "finflow/ledger-service:dev" = "$Registry/finflow-ledger-service@$LedgerDigest"
}

foreach ($entry in $imageReferences.GetEnumerator()) {
    if (-not $application.Contains($entry.Key)) {
        throw "application manifests do not contain expected image $($entry.Key)"
    }
    $application = $application.Replace($entry.Key, $entry.Value)
}

$migrationImage = "finflow/payment-service:dev"
if (-not $migration.Contains($migrationImage)) {
    throw "migration manifests do not contain expected image $migrationImage"
}
$migration = $migration.Replace($migrationImage, $imageReferences[$migrationImage])

$resolvedOutputDirectory = if ([IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory
}
else {
    Join-Path $projectRoot $OutputDirectory
}

[IO.Directory]::CreateDirectory($resolvedOutputDirectory) | Out-Null
[IO.File]::WriteAllText((Join-Path $resolvedOutputDirectory "finflow-kubernetes.yaml"), $application)
[IO.File]::WriteAllText((Join-Path $resolvedOutputDirectory "finflow-migration.yaml"), $migration)

$metadata = [ordered]@{
    generated_at = [DateTimeOffset]::UtcNow.ToString("O")
    registry = $Registry
    images = [ordered]@{
        gateway_service = $imageReferences["finflow/gateway-service:dev"]
        payment_service = $imageReferences["finflow/payment-service:dev"]
        ledger_service = $imageReferences["finflow/ledger-service:dev"]
    }
}

[IO.File]::WriteAllText(
    (Join-Path $resolvedOutputDirectory "release-metadata.json"),
    ($metadata | ConvertTo-Json -Depth 5) + [Environment]::NewLine
)

Write-Host "[release] wrote digest-pinned manifests to $resolvedOutputDirectory"
