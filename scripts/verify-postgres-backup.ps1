[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$BackupPath,

    [string]$PostgresImage = "postgres:18-alpine"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Assert-LastExitCode {
    param([string]$Message)

    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Invoke-VerificationQuery {
    param(
        [string]$Name,
        [string]$Sql,
        [string]$Expected
    )

    $actual = (& docker exec $script:containerName psql --username=postgres --dbname=finflow_restore_verify --no-psqlrc --tuples-only --no-align --command=$Sql).Trim()
    Assert-LastExitCode "verification query failed: $Name"
    if ($actual -ne $Expected) {
        throw "$Name verification expected '$Expected' but returned '$actual'"
    }
    Write-Host "Verified: $Name"
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "docker is required and was not found on PATH"
}

$resolvedBackup = (Resolve-Path -LiteralPath $BackupPath).Path
$manifestPath = "$resolvedBackup.manifest.json"
if (Test-Path -LiteralPath $manifestPath) {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    $actualHash = (Get-FileHash -LiteralPath $resolvedBackup -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $manifest.sha256) {
        throw "backup checksum does not match its manifest"
    }
    Write-Host "Verified: SHA-256 checksum"
}
else {
    Write-Warning "No manifest was found; archive checksum verification was skipped"
}

& docker info --format "{{.ServerVersion}}" | Out-Null
Assert-LastExitCode "Docker is not available"

$script:containerName = "finflow-restore-verify-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
$password = [Guid]::NewGuid().ToString("N")
$backupDirectory = Split-Path -Parent $resolvedBackup
$backupFile = Split-Path -Leaf $resolvedBackup
$started = $false

try {
    & docker run --detach --rm --name $script:containerName `
        --env "POSTGRES_PASSWORD=$password" `
        --env "POSTGRES_DB=finflow_restore_verify" `
        --mount "type=bind,source=$backupDirectory,target=/backup,readonly" `
        $PostgresImage | Out-Null
    Assert-LastExitCode "could not start the isolated restore container"
    $started = $true

    $ready = $false
    for ($attempt = 1; $attempt -le 30; $attempt++) {
        & docker exec $script:containerName pg_isready --username=postgres --dbname=finflow_restore_verify 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw "isolated PostgreSQL did not become ready"
    }

    & docker exec $script:containerName pg_restore --exit-on-error --no-owner --no-privileges `
        --username=postgres --dbname=finflow_restore_verify "/backup/$backupFile"
    Assert-LastExitCode "backup restore failed"

    Invoke-VerificationQuery "migration version" "SELECT COALESCE(max(version), 0) >= 8 FROM schema_migrations" "t"
    Invoke-VerificationQuery "valid payments" "SELECT count(*) FROM payments WHERE amount_cents <= 0 OR status NOT IN ('pending','succeeded','failed') OR currency !~ '^[A-Z]{3}$'" "0"
    Invoke-VerificationQuery "ledger account references" "SELECT count(*) FROM ledger_entries e LEFT JOIN ledger_accounts a ON a.id = e.account_id WHERE a.id IS NULL" "0"
    Invoke-VerificationQuery "balanced ledger transactions" "SELECT count(*) FROM (SELECT transaction_id, currency FROM ledger_entries GROUP BY transaction_id, currency HAVING sum(CASE WHEN direction='debit' THEN amount_cents ELSE 0 END) <> sum(CASE WHEN direction='credit' THEN amount_cents ELSE 0 END)) AS unbalanced" "0"

    Write-Host "Restore verification passed for $resolvedBackup"
}
finally {
    if ($started) {
        & docker rm --force $script:containerName 2>$null | Out-Null
    }
}
