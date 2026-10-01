[CmdletBinding()]
param(
    [ValidateSet("payment-api", "event-throughput")]
    [string]$Scenario = "payment-api",

    [string]$BaseUrl = "http://localhost:8088",
    [string]$ApiKey = "local-dev-api-key-change-me",
    [ValidateRange(1, 1000)]
    [int]$VirtualUsers = 10,
    [string]$Duration = "1m",
    [switch]$AllowRemoteTarget
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$target = [Uri]$BaseUrl
if ($target.Scheme -notin @("http", "https")) {
    throw "BaseUrl must use HTTP or HTTPS"
}
$localHosts = @("localhost", "127.0.0.1", "::1")
if ($target.Host -notin $localHosts -and -not $AllowRemoteTarget) {
    throw "Remote load targets require -AllowRemoteTarget"
}
if ($Duration -notmatch '^\d+(ms|s|m|h)$') {
    throw "Duration must be a k6 duration such as 30s, 5m, or 1h"
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$testDirectory = Join-Path $projectRoot "tests/performance"
$scriptPath = Join-Path $testDirectory "$Scenario.js"
$resultDirectory = Join-Path $projectRoot "dist/performance"
New-Item -ItemType Directory -Force $resultDirectory | Out-Null
$resultName = "$Scenario-$((Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')).json"
$resultPath = Join-Path $resultDirectory $resultName

$environmentNames = @("BASE_URL", "API_KEY", "VUS", "DURATION")
$previousEnvironment = @{}
foreach ($name in $environmentNames) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}

try {
    $env:BASE_URL = $BaseUrl
    $env:API_KEY = $ApiKey
    $env:VUS = $VirtualUsers.ToString()
    $env:DURATION = $Duration

    if (Get-Command k6 -ErrorAction SilentlyContinue) {
        & k6 run --summary-export $resultPath $scriptPath
        if ($LASTEXITCODE -ne 0) {
            throw "k6 thresholds or checks failed"
        }
    }
    else {
        if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
            throw "Install k6 or Docker to run performance tests"
        }
        & docker info --format "{{.ServerVersion}}" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "Docker is not available and k6 is not installed"
        }

        $env:BASE_URL = $BaseUrl -replace '://localhost', '://host.docker.internal'
        & docker run --rm `
            --add-host "host.docker.internal:host-gateway" `
            --env BASE_URL `
            --env API_KEY `
            --env VUS `
            --env DURATION `
            --volume "${testDirectory}:/scripts:ro" `
            --volume "${resultDirectory}:/results" `
            grafana/k6:0.54.0 `
            run `
            --summary-export "/results/$resultName" `
            "/scripts/$Scenario.js"
        if ($LASTEXITCODE -ne 0) {
            throw "k6 thresholds or checks failed"
        }
    }
}
finally {
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], "Process")
    }
}

Write-Host "Performance report: $resultPath"
