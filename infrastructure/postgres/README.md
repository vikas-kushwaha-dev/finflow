# PostgreSQL Data Protection

This document defines FinFlow's database recovery targets, backup controls, retention policy, and recovery procedure. The values below are operating targets, not guarantees; production infrastructure must be measured against them with recurring restore drills.

## Recovery objectives

| Objective | Target | Required control |
| --- | --- | --- |
| Recovery point objective (RPO) | 5 minutes or less | Continuous WAL archiving and a current base backup, or equivalent managed PostgreSQL PITR |
| Recovery time objective (RTO) | 60 minutes or less | Automated restore, practiced runbook, current credentials, and sufficient restore capacity |
| Logical backup cadence | Daily | Custom-format `pg_dump`, encrypted and stored outside the database failure domain |
| Restore drill cadence | Weekly automated, quarterly operator exercise | Restore into an isolated database and run integrity checks |

A logical `pg_dump` is a portable, defense-in-depth backup. It cannot provide point-in-time recovery. Meeting the RPO requires continuous WAL archiving plus base backups, or a managed service with equivalent PITR capability.

## Backup controls

1. Enable data checksums when the PostgreSQL cluster is created and monitor storage integrity.
2. Configure continuous WAL archiving to encrypted, versioned object storage in another failure domain. Alert on archive lag and failed uploads.
3. Keep enough WAL and base backups to cover at least 35 days. Prevent operators and database credentials from deleting immutable backup copies.
4. Run `scripts/backup-postgres.ps1` daily for a custom-format logical backup. Store the archive and its SHA-256 manifest together.
5. Encrypt backups in transit and at rest. Restrict restore and delete permissions to separate operational roles.
6. Run `scripts/verify-postgres-backup.ps1` against the newest logical backup every week and retain the logs as recovery evidence.

The scripts intentionally omit ownership and grants so the archive can be restored under a controlled target role. They never include the connection URL or password in the manifest.

## Point-in-time recovery

1. Declare the incident and record the last known good timestamp. Stop application writes or isolate the damaged primary.
2. Preserve the failed cluster and its logs for investigation. Do not overwrite it with a restore.
3. Select a base backup that predates the target and verify that the WAL archive continuously covers the recovery interval.
4. Restore into a new PostgreSQL cluster in an isolated network. Configure the platform's recovery target timestamp and start recovery.
5. Confirm PostgreSQL reached the requested recovery point and promoted cleanly. Do not accept traffic yet.
6. Run the integrity queries used by `verify-postgres-backup.ps1`, check migration version, compare critical record counts, and reconcile payment and ledger totals against external settlement evidence.
7. Rotate database credentials, update application connection secrets, and use a controlled traffic cutover.
8. Monitor error rate, outbox lag, consumer lag, and ledger balance after cutover. Keep the previous cluster read-only until the incident owner approves disposal.
9. Record actual recovery point, data loss, recovery duration, evidence, and follow-up actions.

Managed PostgreSQL recovery commands differ by provider. Keep provider-specific commands in the protected operations system and test them quarterly.

## Logical restore procedure

Use a logical restore for portability checks, development recovery, or when PITR is unavailable and its older recovery point is accepted.

1. Verify the archive checksum against its manifest.
2. Create a new, empty target database. Never restore over the active production database.
3. Inspect the archive with `pg_restore --list` and restore it with `--exit-on-error --no-owner --no-privileges`.
4. Run `scripts/verify-postgres-backup.ps1` first. For a production recovery, repeat the same checks in the isolated target environment.
5. Reconcile payments and balanced ledger entries before cutover.

Treat every archive as trusted operational input: a restore executes SQL contained in that archive.

## Retention policy

| Data | Default | Automated deletion | Reason |
| --- | --- | --- | --- |
| `payments` | At least 7 years, adjusted for jurisdiction and contracts | No | Financial source record and reconciliation evidence |
| `ledger_accounts`, `ledger_entries` | At least 7 years, adjusted for jurisdiction and contracts | No | Immutable accounting history |
| Published `outbox_events` | 30 days | Yes | Operational delivery record after publication |
| `consumed_ledger_events` | 90 days minimum | Yes | Duplicate suppression across Kafka retention, replay, and incident windows |
| Completed `dead_letter_replays` | 365 days | Yes | Operator replay audit trail |
| Pending or processing operational rows | Until resolved | No | Deletion could lose work or conceal an active incident |

The consumed-event retention must always exceed Kafka topic retention plus the maximum replay window and incident-response buffer. Increase `CONSUMED_EVENT_RETENTION` before increasing those upstream windows.

The retention command defaults to report-only mode. Review its eligible counts first:

```powershell
docker compose --profile operations run --rm retention
```

To approve bounded deletion after review:

```powershell
$env:RETENTION_DRY_RUN = "false"
$env:RETENTION_CONFIRM = "DELETE_OPERATIONAL_DATA"
docker compose --profile operations run --rm retention
```

The command uses a PostgreSQL advisory lock to prevent overlapping runs and deletes in configurable batches. Schedule it during a low-traffic period and alert on failures. It never deletes payment or ledger records.

## PostgreSQL references

- [Backup and restore](https://www.postgresql.org/docs/current/backup.html)
- [Continuous archiving and point-in-time recovery](https://www.postgresql.org/docs/current/continuous-archiving.html)
- [`pg_restore`](https://www.postgresql.org/docs/current/app-pgrestore.html)
- [Data checksums](https://www.postgresql.org/docs/current/checksums.html)
