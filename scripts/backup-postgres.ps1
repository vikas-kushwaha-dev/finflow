[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$DatabaseUrl,

    [string]$OutputDirectory = "backups"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Assert-LastExitCode {
    param([string]$Message)

    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

foreach ($command in @("pg_dump", "pg_restore", "psql")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "$command is required and was not found on PATH"
    }
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

$projectRoot = Split-Path -Parent $PSScriptRoot
if ([IO.Path]::IsPathRooted($OutputDirectory)) {
    $resolvedOutput = [IO.Path]::GetFullPath($OutputDirectory)
}
else {
    $resolvedOutput = [IO.Path]::GetFullPath((Join-Path $projectRoot $OutputDirectory))
}
New-Item -ItemType Directory -Force $resolvedOutput | Out-Null

$timestamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$archivePath = Join-Path $resolvedOutput "finflow-$timestamp.dump"
$manifestPath = "$archivePath.manifest.json"

$environmentNames = @("PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE", "PGSSLMODE")
$previousEnvironment = @{}
foreach ($name in $environmentNames) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}

try {
    $env:PGHOST = $uri.Host
    $env:PGPORT = if ($uri.Port -gt 0) { $uri.Port.ToString() } else { "5432" }
    $env:PGUSER = [Uri]::UnescapeDataString($userInfo[0])
    $env:PGPASSWORD = if ($userInfo.Count -eq 2) { [Uri]::UnescapeDataString($userInfo[1]) } else { "" }
    $env:PGDATABASE = $databaseName

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

    & pg_dump --format=custom --compress=6 --no-owner --no-privileges --file=$archivePath
    Assert-LastExitCode "pg_dump failed"

    & pg_restore --list $archivePath | Out-Null
    Assert-LastExitCode "pg_restore could not read the generated archive"

    $serverVersion = (& psql --no-psqlrc --tuples-only --no-align --command="SHOW server_version").Trim()
    Assert-LastExitCode "could not read the PostgreSQL server version"
    $migrationVersion = (& psql --no-psqlrc --tuples-only --no-align --command="SELECT COALESCE(max(version), 0) FROM schema_migrations").Trim()
    Assert-LastExitCode "could not read the schema migration version"
    $pgDumpVersion = (& pg_dump --version).Trim()
    Assert-LastExitCode "could not read the pg_dump version"

    $archive = Get-Item -LiteralPath $archivePath
    $manifest = [ordered]@{
        format = "postgresql-custom"
        created_at_utc = (Get-Date).ToUniversalTime().ToString("o")
        database = $databaseName
        server_version = $serverVersion
        schema_migration_version = [long]$migrationVersion
        pg_dump_version = $pgDumpVersion
        archive_file = $archive.Name
        archive_bytes = $archive.Length
        sha256 = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    $manifest | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8

    Write-Host "Backup created: $archivePath"
    Write-Host "Manifest created: $manifestPath"
}
finally {
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], "Process")
    }
}
