[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("payment-pod", "ledger-pod", "outbox-pod", "ledger-consumer-pod", "payment-network")]
    [string]$Scenario,

    [Parameter(Mandatory = $true)]
    [string]$ConfirmContext,

    [Parameter(Mandatory = $true)]
    [ValidateScript({ $_ -eq "DISRUPT_FINFLOW" })]
    [string]$ConfirmDisruption,

    [string]$Namespace = "finflow",
    [ValidateRange(30, 600)]
    [int]$RecoveryTimeoutSeconds = 180
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) { throw "kubectl is required" }
$context = (& kubectl config current-context).Trim()
if ($LASTEXITCODE -ne 0 -or $context -ne $ConfirmContext) {
    throw "current kubectl context '$context' does not match ConfirmContext '$ConfirmContext'"
}
$partOf = (& kubectl get namespace $Namespace -o "jsonpath={.metadata.labels.app\.kubernetes\.io/part-of}").Trim()
if ($LASTEXITCODE -ne 0 -or $partOf -ne "finflow") {
    throw "namespace '$Namespace' is not labelled as a FinFlow namespace"
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$evidenceDirectory = Join-Path $projectRoot "dist/resilience"
New-Item -ItemType Directory -Force $evidenceDirectory | Out-Null
$runID = [Guid]::NewGuid().ToString("N")
$evidencePath = Join-Path $evidenceDirectory "kubernetes-$Scenario-$runID.json"
$events = [System.Collections.Generic.List[object]]::new()
$networkPolicy = "finflow-resilience-deny-egress"
$policyCreated = $false
$failure = $null

function Add-Evidence {
    param([string]$Step, [string]$Outcome, [string]$Detail)
    $script:events.Add([ordered]@{timestamp_utc = (Get-Date).ToUniversalTime().ToString("o"); step = $Step; outcome = $Outcome; detail = $Detail})
}

function Wait-Deployment {
    param([string]$Deployment)
    & kubectl rollout status "deployment/$Deployment" -n $Namespace --timeout "${RecoveryTimeoutSeconds}s"
    if ($LASTEXITCODE -ne 0) { throw "$Deployment did not recover" }
    Add-Evidence "recovery" "passed" "$Deployment rollout is available"
}

try {
    Add-Evidence "preflight" "passed" "context=$context namespace=$Namespace"
    $mapping = @{
        "payment-pod" = "payment-service"
        "ledger-pod" = "ledger-service"
        "outbox-pod" = "outbox-publisher"
        "ledger-consumer-pod" = "ledger-consumer"
    }
    if ($Scenario -eq "payment-network") {
        & kubectl get networkpolicy $networkPolicy -n $Namespace 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { throw "temporary network policy already exists; investigate before retrying" }
        $policy = [ordered]@{
            apiVersion = "networking.k8s.io/v1"
            kind = "NetworkPolicy"
            metadata = @{name = $networkPolicy; namespace = $Namespace; labels = @{"app.kubernetes.io/part-of" = "finflow"}}
            spec = @{podSelector = @{matchLabels = @{"app.kubernetes.io/name" = "payment-service"}}; policyTypes = @("Egress"); egress = @()}
        } | ConvertTo-Json -Depth 10
        $policy | & kubectl apply -f - | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "could not apply temporary network policy" }
        $policyCreated = $true
        Add-Evidence "inject" "passed" "denied payment-service egress"
        & kubectl wait pod -n $Namespace -l "app.kubernetes.io/name=payment-service" --for=condition=Ready=false --timeout "${RecoveryTimeoutSeconds}s" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "payment pods did not become unready; NetworkPolicy may not be enforced" }
        Add-Evidence "degradation" "passed" "payment pods became unready"
    }
    else {
        $deployment = $mapping[$Scenario]
        $pod = (& kubectl get pods -n $Namespace -l "app.kubernetes.io/name=$deployment" -o "jsonpath={.items[0].metadata.name}").Trim()
        if ($LASTEXITCODE -ne 0 -or -not $pod) { throw "no pod found for $deployment" }
        & kubectl delete pod $pod -n $Namespace --wait=false | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "could not delete $pod" }
        Add-Evidence "inject" "passed" "deleted pod $pod"
        & kubectl wait --for=delete "pod/$pod" -n $Namespace --timeout "${RecoveryTimeoutSeconds}s" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "$pod was not deleted within the recovery timeout" }
        & kubectl wait pod -n $Namespace -l "app.kubernetes.io/name=$deployment" --for=condition=Ready --timeout "${RecoveryTimeoutSeconds}s" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "replacement pod for $deployment did not become ready" }
        Wait-Deployment $deployment
    }
}
catch {
    $failure = $_
    Add-Evidence $Scenario "failed" $_.Exception.Message
}
finally {
    if ($policyCreated) {
        & kubectl delete networkpolicy $networkPolicy -n $Namespace --ignore-not-found | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Add-Evidence "cleanup" "passed" "removed temporary network policy"
            try { Wait-Deployment "payment-service" } catch {
                Add-Evidence "cleanup" "failed" $_.Exception.Message
                if ($null -eq $failure) { $failure = $_ }
            }
        }
        else {
            Add-Evidence "cleanup" "failed" "could not remove temporary network policy"
            if ($null -eq $failure) { $failure = "could not remove temporary network policy" }
        }
    }
    [ordered]@{run_id = $runID; scenario = $Scenario; context = $context; namespace = $Namespace; events = $events} |
        ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $evidencePath -Encoding utf8
    Write-Host "Resilience evidence: $evidencePath"
}

if ($null -ne $failure) { throw $failure }
Write-Host "Kubernetes resilience drill passed: $Scenario"
