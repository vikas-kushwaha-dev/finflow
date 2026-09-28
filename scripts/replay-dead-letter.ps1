param(
    [Parameter(Mandatory = $true)]
    [string]$EventId,

    [Parameter(Mandatory = $true)]
    [ValidateRange(0, [int]::MaxValue)]
    [int]$Partition,

    [Parameter(Mandatory = $true)]
    [ValidateRange(0, [long]::MaxValue)]
    [long]$Offset,

    [Parameter(Mandatory = $true)]
    [string]$Operator,

    [Parameter(Mandatory = $true)]
    [string]$Reason
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

docker compose run --rm `
    -e "REPLAY_EVENT_ID=$EventId" `
    -e "REPLAY_PARTITION=$Partition" `
    -e "REPLAY_OFFSET=$Offset" `
    -e "REPLAY_OPERATOR=$Operator" `
    -e "REPLAY_REASON=$Reason" `
    ledger-consumer /app/replay-dead-letter

if ($LASTEXITCODE -ne 0) {
    throw "dead-letter replay failed"
}
