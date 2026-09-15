param(
    [string]$GatewayUrl = "http://localhost:8088",
    [string]$PaymentServiceUrl = "http://localhost:8080",
    [string]$LedgerServiceUrl = "http://localhost:8081",
    [string]$ApiKey = "local-dev-api-key-change-me",
    [switch]$SkipComposeUp,
    [switch]$NoBuild,
    [int]$LedgerAttempts = 30,
    [int]$LedgerDelaySeconds = 2
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "[smoke] $Message"
}

function Invoke-Json {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [object]$Body = $null
    )

    $params = @{
        Method = $Method
        Uri = $Uri
        Headers = $Headers
        TimeoutSec = 10
    }

    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = ($Body | ConvertTo-Json -Depth 10)
    }

    Invoke-RestMethod @params
}

function Get-HttpStatus {
    param(
        [string]$Uri,
        [hashtable]$Headers = @{}
    )

    try {
        $response = Invoke-WebRequest -Method Get -Uri $Uri -Headers $Headers -TimeoutSec 10
        return [int]$response.StatusCode
    }
    catch {
        if ($_.Exception.Response) {
            return [int]$_.Exception.Response.StatusCode
        }
        throw
    }
}

function Wait-Healthy {
    param(
        [string]$Name,
        [string]$Uri,
        [hashtable]$Headers = @{},
        [int]$Attempts = 30,
        [int]$DelaySeconds = 2
    )

    for ($i = 1; $i -le $Attempts; $i++) {
        try {
            $status = Get-HttpStatus -Uri $Uri -Headers $Headers
            if ($status -ge 200 -and $status -lt 300) {
                Write-Step "$Name is ready"
                return
            }
        }
        catch {
            if ($i -eq $Attempts) {
                throw
            }
        }

        Start-Sleep -Seconds $DelaySeconds
    }

    throw "$Name did not become ready at $Uri"
}

function Assert-Status {
    param(
        [string]$Name,
        [int]$Actual,
        [int]$Expected
    )

    if ($Actual -ne $Expected) {
        throw "$Name returned status $Actual, expected $Expected"
    }

    Write-Step "$Name returned expected status $Expected"
}

function Assert-BalancedEntries {
    param([array]$Entries)

    if ($Entries.Count -lt 2) {
        throw "expected at least 2 ledger entries, got $($Entries.Count)"
    }

    $totals = @{}
    foreach ($entry in $Entries) {
        if (-not $totals.ContainsKey($entry.currency)) {
            $totals[$entry.currency] = 0
        }

        if ($entry.direction -eq "debit") {
            $totals[$entry.currency] += [int64]$entry.amount_cents
        }
        elseif ($entry.direction -eq "credit") {
            $totals[$entry.currency] -= [int64]$entry.amount_cents
        }
        else {
            throw "unexpected ledger entry direction: $($entry.direction)"
        }
    }

    foreach ($currency in $totals.Keys) {
        if ($totals[$currency] -ne 0) {
            throw "ledger entries are not balanced for $currency"
        }
    }

    Write-Step "ledger entries are balanced"
}

if (-not $SkipComposeUp) {
    Write-Step "starting Docker Compose stack"
    if ($NoBuild) {
        docker compose up -d
    }
    else {
        docker compose up --build -d
    }
}

$clientHeaders = @{
    "X-API-Key" = $ApiKey
}

Wait-Healthy -Name "gateway" -Uri "$GatewayUrl/health"
Wait-Healthy -Name "payment service" -Uri "$PaymentServiceUrl/ready"
Wait-Healthy -Name "ledger service" -Uri "$LedgerServiceUrl/ready"

$idempotencyKey = "smoke-$([guid]::NewGuid().ToString())"
$paymentRequest = @{
    amount_cents = 1299
    currency = "USD"
    description = "Smoke test payment"
    external_reference = $idempotencyKey
}

Write-Step "creating payment through gateway"
$payment = Invoke-Json -Method Post -Uri "$GatewayUrl/api/v1/payments" -Headers ($clientHeaders + @{
    "Idempotency-Key" = $idempotencyKey
}) -Body $paymentRequest

if (-not $payment.id) {
    throw "payment response did not include an id"
}
Write-Step "created payment $($payment.id)"

$replay = Invoke-Json -Method Post -Uri "$GatewayUrl/api/v1/payments" -Headers ($clientHeaders + @{
    "Idempotency-Key" = $idempotencyKey
}) -Body $paymentRequest
if ($replay.id -ne $payment.id) {
    throw "idempotent replay returned a different payment id"
}
Write-Step "idempotent replay returned original payment"

$entriesResponse = $null
for ($i = 1; $i -le $LedgerAttempts; $i++) {
    $entriesResponse = Invoke-Json -Method Get -Uri "$GatewayUrl/api/v1/ledger/payments/$($payment.id)/entries" -Headers $clientHeaders
    if ($entriesResponse.entries.Count -ge 2) {
        break
    }

    Start-Sleep -Seconds $LedgerDelaySeconds
}

if ($null -eq $entriesResponse -or $entriesResponse.entries.Count -lt 2) {
    throw "ledger entries were not created for payment $($payment.id)"
}

Assert-BalancedEntries -Entries $entriesResponse.entries

$balancesResponse = Invoke-Json -Method Get -Uri "$GatewayUrl/api/v1/ledger/balances" -Headers $clientHeaders
if ($null -eq $balancesResponse.balances) {
    throw "ledger balances response did not include balances"
}
Write-Step "ledger balances endpoint responded"

Assert-Status -Name "direct payment API without internal token" -Actual (Get-HttpStatus -Uri "$PaymentServiceUrl/api/v1/payments/$($payment.id)") -Expected 401
Assert-Status -Name "direct ledger API without internal token" -Actual (Get-HttpStatus -Uri "$LedgerServiceUrl/api/v1/ledger/balances") -Expected 401

Write-Step "smoke test completed successfully"
