[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$DatabaseUrl
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if (-not (Get-Command psql -ErrorAction SilentlyContinue)) {
    throw "psql is required and was not found on PATH"
}

$uri = [Uri]$DatabaseUrl
if ($uri.Scheme -notin @("postgres", "postgresql")) {
    throw "DatabaseUrl must use the postgres or postgresql scheme"
}
$userInfo = $uri.UserInfo.Split(":", 2)
if ($userInfo.Count -eq 0 -or [string]::IsNullOrWhiteSpace($userInfo[0])) {
    throw "DatabaseUrl must include a username"
}
$databaseName = [Uri]::UnescapeDataString($uri.AbsolutePath.TrimStart("/"))
if ([string]::IsNullOrWhiteSpace($databaseName)) {
    throw "DatabaseUrl must include a database name"
}

$environmentNames = @("PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSSLMODE", "PGOPTIONS")
$previousEnvironment = @{}
foreach ($name in $environmentNames) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$queryFile = Join-Path $projectRoot "tests/performance/postgres-query-plans.sql"
$resultDirectory = Join-Path $projectRoot "dist/performance"
New-Item -ItemType Directory -Force $resultDirectory | Out-Null
$resultPath = Join-Path $resultDirectory "postgres-plans-$((Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')).txt"

try {
    $env:PGHOST = $uri.Host
    $env:PGPORT = if ($uri.Port -gt 0) { $uri.Port.ToString() } else { "5432" }
    $env:PGUSER = [Uri]::UnescapeDataString($userInfo[0])
    $env:PGPASSWORD = if ($userInfo.Count -eq 2) { [Uri]::UnescapeDataString($userInfo[1]) } else { "" }
    $env:PGDATABASE = $databaseName
    $env:PGOPTIONS = "-c default_transaction_read_only=on -c statement_timeout=30000"

    $sslMode = $null
    foreach ($pair in $uri.Query.TrimStart("?").Split("&", [StringSplitOptions]::RemoveEmptyEntries)) {
        $parts = $pair.Split("=", 2)
        if ([Uri]::UnescapeDataString($parts[0]) -eq "sslmode" -and $parts.Count -eq 2) {
            $sslMode = [Uri]::UnescapeDataString($parts[1])
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($sslMode)) {
        $env:PGSSLMODE = $sslMode
    }
    else {
        Remove-Item Env:PGSSLMODE -ErrorAction SilentlyContinue
    }

    $output = (& psql --no-psqlrc --file=$queryFile 2>&1 | Out-String)
    $exitCode = $LASTEXITCODE
    $output | Set-Content -LiteralPath $resultPath -Encoding utf8
    if ($exitCode -ne 0) {
        throw "PostgreSQL query analysis failed; inspect $resultPath"
    }
    Write-Host $output
    Write-Host "Query plan report: $resultPath"
}
finally {
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], "Process")
    }
}
