[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateScript({ $_ -eq "DISRUPT_LOCAL_FINFLOW" })]
    [string]$ConfirmDisruption,

    [ValidateSet("postgres-network", "kafka-network", "payment-restart", "ledger-consumer-restart", "all")]
    [string]$Scenario = "all",

    [ValidateRange(1, 30)]
    [int]$DisruptionSeconds = 5,
    [string]$GatewayUrl = "http://localhost:8088",
    [string]$PaymentServiceUrl = "http://localhost:8080",
    [string]$LedgerServiceUrl = "http://localhost:8081",
    [string]$ApiKey = "local-dev-api-key-change-me"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

foreach ($url in @($GatewayUrl, $PaymentServiceUrl, $LedgerServiceUrl)) {
    $uri = [Uri]$url
    if ($uri.Scheme -notin @("http", "https") -or $uri.Host -notin @("localhost", "127.0.0.1", "::1")) {
        throw "Docker resilience tests are restricted to local URLs"
    }
}
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "docker is required"
}
& docker info --format "{{.ServerVersion}}" | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Docker is not available"
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$evidenceDirectory = Join-Path $projectRoot "dist/resilience"
New-Item -ItemType Directory -Force $evidenceDirectory | Out-Null
$runID = [Guid]::NewGuid().ToString("N")
$evidencePath = Join-Path $evidenceDirectory "docker-$Scenario-$runID.json"
$evidence = [System.Collections.Generic.List[object]]::new()
$pausedContainers = [System.Collections.Generic.HashSet[string]]::new()
$failure = $null

function Add-Evidence {
    param([string]$Step, [string]$Outcome, [string]$Detail)
    $script:evidence.Add([ordered]@{
        timestamp_utc = (Get-Date).ToUniversalTime().ToString("o")
        step = $Step
        outcome = $Outcome
        detail = $Detail
    })
}

function Get-Status {
    param([string]$Url)
    try {
        return [int](Invoke-WebRequest -Method Get -Uri $Url -TimeoutSec 3 -SkipHttpErrorCheck).StatusCode
    }
    catch {
        return 0
    }
}

function Wait-Status {
    param(
        [string]$Name,
        [string]$Url,
        [scriptblock]$Accept,
        [int]$Attempts = 30
    )
    for ($attempt = 1; $attempt -le $Attempts; $attempt++) {
        $status = Get-Status $Url
        if (& $Accept $status) {
            Add-Evidence $Name "passed" "HTTP status $status"
            return $status
        }
        Start-Sleep -Seconds 1
    }
    throw "$Name did not reach the expected state at $Url"
}

function Assert-ContainerReady {
    param([string]$Container)
    $state = (& docker inspect --format "{{.State.Running}}:{{.State.Paused}}" $Container 2>$null).Trim()
    if ($LASTEXITCODE -ne 0 -or $state -ne "true:false") {
        throw "$Container must be running and unpaused before the drill"
    }
}

function Pause-Container {
    param([string]$Container)
    Assert-ContainerReady $Container
    & docker pause $Container | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "could not pause $Container" }
    [void]$script:pausedContainers.Add($Container)
    Add-Evidence "inject" "passed" "paused $Container"
}

function Resume-Container {
    param([string]$Container)
    if ($script:pausedContainers.Contains($Container)) {
        & docker unpause $Container | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "could not unpause $Container" }
        [void]$script:pausedContainers.Remove($Container)
        Add-Evidence "recover" "passed" "unpaused $Container"
    }
}

function New-TestPayment {
    $idempotencyKey = "resilience-$runID-$([Guid]::NewGuid().ToString('N'))"
    $headers = @{"X-API-Key" = $ApiKey; "Idempotency-Key" = $idempotencyKey}
    $body = @{amount_cents = 1299; currency = "USD"; description = "Resilience drill"; external_reference = $idempotencyKey} | ConvertTo-Json
    $payment = Invoke-RestMethod -Method Post -Uri "$GatewayUrl/api/v1/payments" -Headers $headers -ContentType "application/json" -Body $body -TimeoutSec 10
    if (-not $payment.id) { throw "payment response did not include an id" }
    Add-Evidence "create-payment" "passed" "payment accepted"
    return $payment.id
}

function Wait-LedgerEntries {
    param([string]$PaymentID)
    $headers = @{"X-API-Key" = $ApiKey}
    for ($attempt = 1; $attempt -le 45; $attempt++) {
        $response = Invoke-RestMethod -Method Get -Uri "$GatewayUrl/api/v1/ledger/payments/$PaymentID/entries" -Headers $headers -TimeoutSec 10
        if ($response.entries.Count -eq 2) {
            $debits = ($response.entries | Where-Object direction -eq "debit" | Measure-Object amount_cents -Sum).Sum
            $credits = ($response.entries | Where-Object direction -eq "credit" | Measure-Object amount_cents -Sum).Sum
            if ($debits -ne $credits) { throw "ledger entries are not balanced" }
            Add-Evidence "ledger-recovery" "passed" "two balanced entries became readable"
            return
        }
        Start-Sleep -Seconds 1
    }
    throw "ledger entries were not created after recovery"
}

function Invoke-PostgresNetworkDrill {
    Pause-Container "finflow-postgres"
    try {
        Wait-Status "payment remains live" "$PaymentServiceUrl/health" { param($status) $status -eq 200 } | Out-Null
        Wait-Status "payment reports dependency outage" "$PaymentServiceUrl/ready" { param($status) $status -eq 0 -or $status -eq 503 } | Out-Null
        Start-Sleep -Seconds $DisruptionSeconds
    }
    finally {
        Resume-Container "finflow-postgres"
    }
    Wait-Status "payment readiness recovers" "$PaymentServiceUrl/ready" { param($status) $status -eq 200 } | Out-Null
    Wait-Status "ledger readiness recovers" "$LedgerServiceUrl/ready" { param($status) $status -eq 200 } | Out-Null
    $paymentID = New-TestPayment
    Wait-LedgerEntries $paymentID
}

function Invoke-KafkaNetworkDrill {
    Pause-Container "finflow-kafka"
    try {
        $paymentID = New-TestPayment
        Add-Evidence "outbox-degradation" "passed" "payment accepted while Kafka was unavailable"
        Start-Sleep -Seconds $DisruptionSeconds
    }
    finally {
        Resume-Container "finflow-kafka"
    }
    Wait-LedgerEntries $paymentID
}

function Invoke-RestartDrill {
    param([string]$Container, [string]$ReadyUrl)
    Assert-ContainerReady $Container
    & docker restart --time 10 $Container | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "could not restart $Container" }
    Add-Evidence "inject" "passed" "restarted $Container"
    if ($ReadyUrl) {
        Wait-Status "$Container recovery" $ReadyUrl { param($status) $status -eq 200 } | Out-Null
    }
    else {
        Assert-ContainerReady $Container
        Add-Evidence "$Container recovery" "passed" "container is running"
    }
    $paymentID = New-TestPayment
    Wait-LedgerEntries $paymentID
}

try {
    foreach ($container in @("finflow-postgres", "finflow-kafka", "finflow-gateway-service", "finflow-payment-service", "finflow-ledger-service", "finflow-outbox-publisher", "finflow-ledger-consumer")) {
        Assert-ContainerReady $container
    }
    Wait-Status "gateway preflight" "$GatewayUrl/health" { param($status) $status -eq 200 } | Out-Null
    Wait-Status "payment preflight" "$PaymentServiceUrl/ready" { param($status) $status -eq 200 } | Out-Null
    Wait-Status "ledger preflight" "$LedgerServiceUrl/ready" { param($status) $status -eq 200 } | Out-Null

    $scenarios = if ($Scenario -eq "all") { @("postgres-network", "kafka-network", "payment-restart", "ledger-consumer-restart") } else { @($Scenario) }
    foreach ($current in $scenarios) {
        Add-Evidence $current "started" "fault injection started"
        switch ($current) {
            "postgres-network" { Invoke-PostgresNetworkDrill }
            "kafka-network" { Invoke-KafkaNetworkDrill }
            "payment-restart" { Invoke-RestartDrill "finflow-payment-service" "$PaymentServiceUrl/ready" }
            "ledger-consumer-restart" { Invoke-RestartDrill "finflow-ledger-consumer" "" }
        }
        Add-Evidence $current "passed" "steady state restored"
    }
}
catch {
    $failure = $_
    Add-Evidence $Scenario "failed" $_.Exception.Message
}
finally {
    foreach ($container in @($pausedContainers)) {
        try { Resume-Container $container } catch {
            Add-Evidence "cleanup" "failed" $_.Exception.Message
            if ($null -eq $failure) { $failure = $_ }
        }
    }
    [ordered]@{
        run_id = $runID
        scenario = $Scenario
        started_from_commit = (git -C $projectRoot rev-parse HEAD).Trim()
        events = $evidence
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $evidencePath -Encoding utf8
    Write-Host "Resilience evidence: $evidencePath"
}

if ($null -ne $failure) { throw $failure }
Write-Host "Resilience drill passed: $Scenario"
