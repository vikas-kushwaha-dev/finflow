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

Push-Location $projectRoot
try {
    foreach ($service in $services) {
        Write-Step "testing $service"
        Push-Location $service
        try {
            go test ./...

            Write-Step "building commands in $service"
            go build ./cmd/...
        }
        finally {
            Pop-Location
        }
    }

    Write-Step "validating Docker Compose configuration"
    docker compose config --quiet

    Write-Step "all checks passed"
}
finally {
    Pop-Location
}
