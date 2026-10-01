# FinFlow Scripts

## Local CI checks

Run from the project root:

```powershell
.\scripts\check.ps1
```

The script runs all Go tests, builds every service command, validates the Docker Compose configuration, and tests both development and release Kubernetes manifest rendering. These are the same checks enforced by GitHub Actions.

## Docker smoke test

Run from the project root:

```powershell
.\scripts\smoke.ps1
```

The script starts Docker Compose by default, creates a payment through the gateway, waits for ledger entries, checks the entries are balanced, and confirms direct service API calls are blocked without the internal service token.

To run against an already-started stack:

```powershell
.\scripts\smoke.ps1 -SkipComposeUp
```

To verify a deployed environment through its public gateway when service ports are intentionally private:

```powershell
.\scripts\smoke.ps1 `
  -SkipComposeUp `
  -GatewayOnly `
  -GatewayUrl "https://finflow.example.com" `
  -ApiKey "YOUR_RELEASE_API_KEY"
```

Gateway-only mode still creates and replays a payment and waits for balanced ledger entries. It skips direct service readiness and internal-port authentication checks.

If Docker image downloads are slow or fail, run the script again after Docker Desktop finishes pulling `postgres`, `apache/kafka`, `golang`, and `alpine` images.

The smoke test is a release check rather than a pull-request CI check because it starts PostgreSQL, Kafka, and every FinFlow service. Run it before a release or after changes to cross-service behavior, event delivery, Dockerfiles, or Docker Compose wiring.

## Dead-letter replay

After fixing the cause of a failed ledger event, replay one exact dead-letter record by its event ID, partition, and offset:

```powershell
.\scripts\replay-dead-letter.ps1 `
  -EventId "event-id" `
  -Partition 0 `
  -Offset 42 `
  -Operator "operator@example.com" `
  -Reason "Database constraint corrected"
```

The command verifies the requested event ID, allows only the configured payment-event source topic, and writes a durable audit record. A succeeded or in-progress event cannot be replayed again automatically. A failed replay may be retried.

## Release manifest rendering

`render-release-manifests.ps1` renders the Kubernetes application and migration resources with immutable container image digests. The release workflow calls it after publishing all three images. It can also be run locally with a registry path and three valid `sha256` digests.

## PostgreSQL backup

Create a custom-format logical backup and SHA-256 manifest with locally installed PostgreSQL client tools:

```powershell
.\scripts\backup-postgres.ps1 `
  -DatabaseUrl $env:DATABASE_URL
```

Backups default to the ignored `backups/` directory. The script validates that `pg_restore` can read the archive and records the server, migration, and client-tool versions without writing credentials to the manifest.

## Restore verification

Restore a backup into a disposable PostgreSQL container and verify its integrity:

```powershell
.\scripts\verify-postgres-backup.ps1 `
  -BackupPath .\backups\finflow-TIMESTAMP.dump
```

The source database is never contacted. The script verifies the checksum, restores into an isolated database, validates migration state and payment constraints, and confirms every ledger transaction is balanced.

## Data retention

Preview eligible operational records without deleting them:

```powershell
docker compose --profile operations run --rm retention
```

After reviewing the counts, a deletion run requires both safeguards:

```powershell
$env:RETENTION_DRY_RUN = "false"
$env:RETENTION_CONFIRM = "DELETE_OPERATIONAL_DATA"
docker compose --profile operations run --rm retention
```

See `infrastructure/postgres/README.md` for recovery objectives, PITR requirements, restore procedures, and the complete retention policy.

## Performance tests

Run the payment API workload with local k6 or the pinned Docker fallback:

```powershell
.\scripts\load-test.ps1 `
  -Scenario payment-api `
  -VirtualUsers 10 `
  -Duration 5m
```

Use `-Scenario event-throughput` to measure payment-to-ledger propagation. Remote targets are rejected unless `-AllowRemoteTarget` is supplied deliberately. Summaries are written under ignored `dist/performance/`.

Capture read-only PostgreSQL execution plans with:

```powershell
.\scripts\analyze-postgres.ps1 -DatabaseUrl $env:DATABASE_URL
```

See `infrastructure/performance/README.md` for workload profiles, SLOs, stop conditions, capacity assumptions, and scaling guidance.
