# FinFlow observability

All APIs expose Prometheus metrics on their HTTP port at `/metrics`. The outbox publisher and ledger consumer expose `/metrics` and `/health` on `METRICS_ADDR`, which defaults to `:9090`.

`prometheus.yml` is a starting scrape configuration for the Docker Compose network. Kubernetes pod annotations in the deployment manifests expose the same targets to annotation-based Prometheus discovery. Import `alerts.yml` into Prometheus or your managed monitoring service and tune thresholds after observing normal production traffic.

## Suggested dashboard

Use these panels as the minimum payment-flow dashboard:

| Panel | PromQL |
| --- | --- |
| Request rate | `sum by (job, route) (rate(finflow_http_requests_total[5m]))` |
| HTTP 5xx ratio | `sum by (job) (rate(finflow_http_requests_total{status=~"5.."}[5m])) / sum by (job) (rate(finflow_http_requests_total[5m]))` |
| HTTP p95 latency | `histogram_quantile(0.95, sum by (job, le) (rate(finflow_http_request_duration_seconds_bucket[5m])))` |
| Database p95 latency | `histogram_quantile(0.95, sum by (job, operation, le) (rate(finflow_db_operation_duration_seconds_bucket[5m])))` |
| Kafka publish failures | `sum by (event_type) (rate(finflow_kafka_messages_published_total{result="error"}[5m]))` |
| Kafka consume failures | `sum by (event_type) (rate(finflow_kafka_messages_consumed_total{result="error"}[5m]))` |
| Consumer lag | `max by (topic, partition) (finflow_kafka_consumer_lag_messages)` |
| Consumer retries | `sum by (event_type) (rate(finflow_kafka_consumer_retries_total[5m]))` |
| Dead-letter outcomes | `sum by (event_type, result) (increase(finflow_kafka_dead_letters_total[1h]))` |

## Tracing

Set `OTEL_EXPORTER_OTLP_ENDPOINT` to an OTLP/HTTP collector endpoint, for example `http://otel-collector:4318`. When it is unset, trace export is disabled while W3C propagation remains active. HTTP context travels through the gateway and internal APIs; `traceparent` and `tracestate` are persisted in the transactional outbox, injected into Kafka headers, and continued by the ledger consumer. Baggage is propagated over synchronous HTTP but is deliberately not persisted.

Do not put payment IDs, idempotency keys, customer data, or raw SQL in metric labels. Use traces and structured logs for request-level investigation.

## First-response guide

1. For API errors, compare request rate, 5xx ratio, and p95 latency, then inspect traces for the affected route.
2. For publish failures, verify Kafka connectivity and TLS/SASL settings, then inspect outbox rows in `failed` state.
3. For consumer lag, compare consume failures with database latency before scaling consumers; partition count limits useful concurrency.
4. For database errors, check PostgreSQL availability and pool pressure before retrying or restarting workloads.
5. For dead letters, fix the underlying cause first, identify the exact topic partition and offset, and use `scripts/replay-dead-letter.ps1`. Never replay by copying payloads manually.
