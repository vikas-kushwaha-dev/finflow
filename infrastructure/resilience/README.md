# Controlled Resilience Testing

These drills verify FinFlow's behavior during dependency loss and workload replacement. Run them only in local or dedicated non-production environments. They create test payments, interrupt services, and can temporarily remove pod readiness.

## Safety controls

- The Docker runner requires the literal confirmation `DISRUPT_LOCAL_FINFLOW`, accepts only loopback URLs, and limits dependency pauses to 30 seconds.
- Every paused container is tracked and unpaused in `finally`, including after an assertion fails.
- The Kubernetes runner requires the exact active context, a FinFlow-labelled namespace, and the literal confirmation `DISRUPT_FINFLOW`.
- The temporary NetworkPolicy has a fixed name; the runner refuses to start if that policy already exists and removes only the policy it created.
- Every run writes timestamped evidence under ignored `dist/resilience/` without API keys or database credentials.
- A cleanup failure fails the drill. Treat it as an incident and restore steady state manually before another experiment.

## Steady-state invariants

Before and after each experiment:

1. Gateway health returns `200`.
2. Payment and ledger readiness return `200` when PostgreSQL is available.
3. A payment can be accepted through the gateway.
4. Exactly two balanced ledger entries eventually become readable for that payment.
5. No dependency or application container remains paused.
6. No `finflow-resilience-deny-egress` NetworkPolicy remains after a Kubernetes network experiment.

## Docker experiments

Start the complete stack before running a drill:

```powershell
docker compose up --build -d
.\scripts\resilience-test.ps1 `
  -ConfirmDisruption DISRUPT_LOCAL_FINFLOW `
  -Scenario all
```

| Scenario | Injection | Hypothesis |
| --- | --- | --- |
| `postgres-network` | Pause PostgreSQL for up to 30 seconds | Payment stays live, reports not-ready, then recovers; payment and ledger flow succeeds afterward |
| `kafka-network` | Pause Kafka after preflight | Payment is accepted into PostgreSQL/outbox while Kafka is unavailable; ledger catches up after recovery |
| `payment-restart` | Gracefully restart the payment container | Readiness recovers and a new payment completes end to end |
| `ledger-consumer-restart` | Gracefully restart the consumer | The consumer resumes from committed offsets and a new payment reaches balanced ledger entries |

Run one experiment with a shorter interruption:

```powershell
.\scripts\resilience-test.ps1 `
  -ConfirmDisruption DISRUPT_LOCAL_FINFLOW `
  -Scenario kafka-network `
  -DisruptionSeconds 3
```

Docker pause is used as a bounded unresponsive-dependency simulation. It is not a complete substitute for latency, packet loss, partial partition, or managed-service failover testing.

## Kubernetes experiments

Use a dedicated non-production namespace and verify the current context before supplying it back to the runner:

```powershell
$context = kubectl config current-context
.\scripts\kubernetes-resilience-test.ps1 `
  -Scenario payment-pod `
  -ConfirmContext $context `
  -ConfirmDisruption DISRUPT_FINFLOW
```

Pod experiments delete one selected pod and wait for deletion, replacement readiness, and Deployment availability. Available scenarios are `payment-pod`, `ledger-pod`, `outbox-pod`, and `ledger-consumer-pod`.

The `payment-network` scenario applies a temporary deny-all egress NetworkPolicy to payment pods, verifies that they become unready, removes the policy, and waits for recovery. It requires a CNI that enforces Kubernetes NetworkPolicy. If pods do not become unready, the drill fails rather than claiming the partition worked.

## Abort conditions

Abort further experiments when any of these occurs:

- the active context or namespace is not the intended dedicated environment;
- an unrelated alert or production incident begins;
- a fault remains active beyond the configured limit;
- payment or ledger readiness does not recover within three minutes;
- balanced ledger entries do not appear after recovery;
- consumer lag continues growing after the dependency returns;
- a paused container or temporary NetworkPolicy cannot be removed;
- evidence indicates duplicate or unbalanced ledger entries.

## Manual recovery

Inspect state before changing it:

```powershell
docker inspect --format "{{.State.Running}} {{.State.Paused}}" finflow-postgres finflow-kafka
kubectl get networkpolicy finflow-resilience-deny-egress -n finflow
```

Remove only a fault created by the drill:

```powershell
docker unpause finflow-postgres
docker unpause finflow-kafka
kubectl delete networkpolicy finflow-resilience-deny-egress -n finflow
```

Then rerun `scripts/smoke.ps1 -SkipComposeUp`, inspect outbox and Kafka lag metrics, and reconcile the test payment's two ledger entries. Compare measured recovery time with the recovery objectives in `infrastructure/postgres/README.md`.

## Evidence review

For each accepted drill, retain the commit, environment, scenario, start and recovery timestamps, generated JSON evidence, relevant metrics, logs, traces, observed data loss, and follow-up action. A passed script proves only the encoded invariants; it does not prove recovery from every failure mode.
