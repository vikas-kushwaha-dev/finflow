# Performance and Capacity Engineering

This document defines FinFlow's initial service-level objectives, repeatable workload profiles, capacity assumptions, and scaling process. These are engineering targets. They are not measured production claims until a report from a production-like environment is reviewed and recorded.

## Initial objectives

| Indicator | Objective | Measurement |
| --- | --- | --- |
| HTTP availability | At least 99% successful requests over 30 days | Non-5xx gateway responses divided by all gateway responses |
| Create payment latency | p95 below 300 ms, p99 below 750 ms | Gateway `POST /api/v1/payments` |
| Read payment latency | p95 below 150 ms, p99 below 500 ms | Gateway `GET /api/v1/payments/{id}` |
| Database operation latency | p95 below 100 ms | Instrumented pgx operations by service and operation |
| Payment-to-ledger propagation | p95 below 5 seconds, p99 below 10 seconds | Payment acceptance until two ledger entries are readable |
| Ledger consumer lag | Below 100 messages for steady state | Maximum reported partition lag |
| Database pool utilization | Below 80% for steady state | Acquired connections divided by configured maximum |

The k6 thresholds encode the request and event objectives. Prometheus recording and alerting rules encode the continuously observable objectives. Revise a target only with benchmark evidence and a documented product decision.

## Workload profiles

Run only against local or dedicated non-production environments. The runner rejects remote targets unless `-AllowRemoteTarget` is explicitly supplied.

The normal local gateway allows 120 requests per minute per client. For a dedicated capacity run, set `GATEWAY_RATE_LIMIT_REQUESTS` to the intended test ceiling and recreate the gateway before running k6. Keep the normal limit for abuse-control testing, and never relax a shared or production gateway merely to make a benchmark pass.

| Profile | Command parameters | Purpose |
| --- | --- | --- |
| Smoke | `-VirtualUsers 2 -Duration 30s` | Validate scripts and environment |
| Baseline | `-VirtualUsers 10 -Duration 5m` | Establish repeatable latency and throughput |
| Capacity step | Increase users by 10 for each 10-minute run | Find the first breached SLO or saturated dependency |
| Soak | 70% of proven capacity for 2h | Detect leaks, pool exhaustion, backlog, and latency drift |

API workload:

```powershell
.\scripts\load-test.ps1 -Scenario payment-api -VirtualUsers 10 -Duration 5m
```

Event workload:

```powershell
.\scripts\load-test.ps1 -Scenario event-throughput -VirtualUsers 5 -Duration 5m
```

Each run writes a k6 summary under ignored `dist/performance/`. Record the commit, environment shape, database size, Kafka partition count, virtual users, request rate, latency percentiles, failure rate, pool utilization, CPU, memory, and consumer lag with any accepted baseline.

## PostgreSQL analysis

Capture representative plans after a baseline load has populated the database:

```powershell
.\scripts\analyze-postgres.ps1 -DatabaseUrl $env:DATABASE_URL
```

The script enables session-level read-only mode and a 30-second statement timeout. It runs `EXPLAIN ANALYZE` only for SELECT statements and writes the output to `dist/performance/`. Review sequential scans, row-estimate errors, sort spills, buffer reads, and execution time. Do not add an index from a single plan: confirm the query is important, compare realistic cardinality, and account for write amplification.

## Current capacity assumptions

- Kubernetes starts two replicas each for gateway, payment API, and ledger API; workers start with one replica each.
- Each database-backed process currently allows 10 pgx connections. At the default replica counts, payment API, ledger API, outbox publisher, and ledger consumer can hold up to 60 connections total.
- Allow at least 20 additional PostgreSQL connections for migrations, one-off operations, monitoring, and administration. A deployment using these defaults therefore needs at least 80 usable application connections, with provider-reserved connections considered separately.
- Kafka has three partitions. Useful ledger-consumer concurrency cannot exceed partition count, and ordering requirements may justify less concurrency.
- Kubernetes requests and limits are starting guardrails, not capacity proof. Load tests must establish throughput per pod on the actual runtime and database tier.

## Scaling decisions

1. Establish a stable baseline with no SLO breaches and capture all evidence listed above.
2. Identify the first saturated resource. Do not scale every tier together because that hides the constraint.
3. Scale gateway or API replicas when CPU is sustained above 70% of requested capacity and latency rises while database pool utilization remains healthy.
4. Scale workers only when lag grows and Kafka partitions permit more useful consumers. Increase partitions deliberately because it changes ordering and operational behavior.
5. Tune query shape and indexes before increasing PostgreSQL connections. More connections can worsen contention and memory pressure.
6. Increase pool size only when acquire pressure is sustained, database CPU and lock waits are healthy, and the database connection budget covers every replica plus reserve.
7. Repeat baseline and soak profiles after each material change. Keep the previous accepted result for comparison.

## Stop conditions

Stop a capacity test when error rate exceeds 5%, p99 latency exceeds 2 seconds for five minutes, database pool utilization exceeds 95%, consumer lag grows without recovery, database CPU or storage latency is saturated, or any integrity check fails. A stress test is not permission to continue harming a shared environment.
