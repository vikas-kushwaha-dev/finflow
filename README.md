# FinFlow

FinFlow is a fintech/payments learning project built incrementally with production-style boundaries.

Milestone 1 contains a Go payment service backed by PostgreSQL. Kafka and Kubernetes are intentionally not implemented yet; they come after the first service is working and tested.

## What exists now

- Go payment service in `services/payment-service`
- Chi HTTP router
- PostgreSQL connection via `pgxpool`
- Docker Compose PostgreSQL setup
- Dockerized payment service
- one-shot migration container
- SQL migration runner for the `payments` table
- Ledger service foundation
- Ledger HTTP API
- API gateway service
- Double-entry ledger rules
- Ledger account and entry migrations
- Kafka local infrastructure
- payment outbox table
- outbox publisher command
- payment event schemas for `payment.created` and `payment.status_changed`
- ledger payment event consumer
- idempotent ledger event consumption
- centralized gateway authentication, request logging, and rate limiting
- internal service-token authentication from gateway to services
- Repository, service, handler, model, config, and database packages
- `GET /health`
- `GET /ready`
- `GET /metrics`
- `GET /api/v1/ledger/balances`
- `GET /api/v1/ledger/payments/{payment_id}/entries`
- `POST /api/v1/payments`
- `GET /api/v1/payments/{id}`
- `PATCH /api/v1/payments/{id}/status`
- request validation
- API key authentication for payment endpoints
- request body size limit for payment endpoints
- basic HTTP security headers
- payment status transition rules
- optional idempotency support through the `Idempotency-Key` header
- idempotency request hashing to reject conflicting retries
- JSON error responses with request IDs
- structured server logs
- startup configuration validation
- readiness checks for PostgreSQL
- lightweight JSON runtime metrics
- PostgreSQL integration test entry point
- unit tests for service and handler behavior
- GitHub Actions CI for tests, command builds, and Docker Compose validation
- Kubernetes manifests for APIs, workers, configuration, and migrations
- tagged release automation with image scanning and digest-pinned manifests
- Kafka TLS, mutual TLS, and SASL authentication support

## Requirements

- Go 1.26+
- Docker Desktop

## Run locally

From the project root:

```bash
copy .env.example .env
docker compose up --build -d
```

The local example API key is `local-dev-api-key-change-me`. Change `API_KEY` and `INTERNAL_SERVICE_TOKEN` before using this outside local development.

Then in another terminal:

```bash
curl http://localhost:8080/health
```

Readiness checks confirm the service can reach PostgreSQL:

```bash
curl http://localhost:8080/ready
```

Runtime metrics are exposed as JSON:

```bash
curl http://localhost:8080/metrics
```

The gateway is available on port `8088` and is the preferred client-facing entry point:

```bash
curl http://localhost:8088/health
```

The ledger service exposes internal health/readiness on port `8081`, while ledger API reads are available through the gateway.

Kafka is included in Docker Compose for local event publishing. The payment service records events in `outbox_events`, the outbox publisher sends them to the `finflow.payment.events` topic, and the ledger consumer creates balanced ledger entries from `payment.created` events.

Local Docker Compose uses explicit plaintext Kafka settings. Kubernetes workers require TLS and SCRAM authentication, with credentials and CA certificates supplied through Kubernetes Secrets.

Create a payment:

```bash
curl -X POST http://localhost:8088/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-API-Key: local-dev-api-key-change-me" \
  -H "Idempotency-Key: demo-key-001" \
  -d "{\"amount_cents\":1299,\"currency\":\"USD\",\"description\":\"Test payment\"}"
```

Send the same request with the same `Idempotency-Key` to receive the original payment instead of creating a duplicate.

If the same `Idempotency-Key` is reused with different payment details, the API returns `409 Conflict`.

Retrieve a payment:

```bash
curl http://localhost:8088/api/v1/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8 \
  -H "X-API-Key: local-dev-api-key-change-me"
```

Update payment status:

```bash
curl -X PATCH http://localhost:8088/api/v1/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8/status \
  -H "Content-Type: application/json" \
  -H "X-API-Key: local-dev-api-key-change-me" \
  -d "{\"status\":\"succeeded\"}"
```

List ledger balances:

```bash
curl http://localhost:8088/api/v1/ledger/balances \
  -H "X-API-Key: local-dev-api-key-change-me"
```

List ledger entries for a payment:

```bash
curl http://localhost:8088/api/v1/ledger/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8/entries \
  -H "X-API-Key: local-dev-api-key-change-me"
```

## Test

Run the same fast checks used by CI from the project root:

```powershell
.\scripts\check.ps1
```

From `services/payment-service`:

```bash
go test ./...
```

From `services/ledger-service`:

```bash
go test ./...
```

From `services/gateway-service`:

```bash
go test ./...
```

Run PostgreSQL integration tests after starting Docker Compose:

```bash
$env:INTEGRATION_DATABASE_URL="postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"
go test ./internal/repository
```

Run the Docker smoke test from the project root:

```powershell
.\scripts\smoke.ps1
```

The smoke test starts Docker Compose, creates a payment through the gateway, waits for ledger entries, checks that the entries balance, and confirms direct payment/ledger API calls fail without the internal service token.

GitHub Actions runs unit tests and command builds for all three Go modules, validates Docker Compose, and renders the Kubernetes manifests on every push and pull request. The Docker smoke test remains a release check because it requires the full PostgreSQL, Kafka, gateway, payment, and ledger stack.

## Kubernetes

The deployment foundation is in `infrastructure/kubernetes`. It runs the three APIs and two workers in Kubernetes while treating PostgreSQL and Kafka as externally managed dependencies. See `infrastructure/kubernetes/README.md` for image, Secret, migration, and deployment instructions.

## Kafka security

Kafka producers and consumers support TLS 1.2+, custom CA trust, optional client certificates, and SASL mechanisms `plain`, `scram-sha-256`, and `scram-sha-512`. Set `KAFKA_REQUIRE_SECURE_TRANSPORT=true` for deployed workers so a missing TLS configuration fails at startup. SASL passwords are never included in validation errors or startup logs.

## Release

Push a semantic version tag such as `v1.0.0` to start the release workflow. Releases are gated by CI and container vulnerability scanning, publish multi-architecture images to GitHub Container Registry, and attach digest-pinned Kubernetes manifests to the GitHub release.

The release sequence runs the generated migration Job before applying the application manifests. After rollout, use `scripts/smoke.ps1 -SkipComposeUp -GatewayOnly` against the public gateway. Full instructions and rollback constraints are documented in `infrastructure/kubernetes/README.md`.

## API

Client API requests require an API key at the gateway. Send API requests through the gateway on `localhost:8088`:

```bash
X-API-Key: local-dev-api-key-change-me
```

You can also send:

```bash
Authorization: Bearer local-dev-api-key-change-me
```

The payment and ledger services require `X-Internal-Service-Token` for their `/api/v1` routes. The gateway sets this header when proxying requests, so clients should not call service ports directly.

Public endpoints:

- `GET /health`
- `GET /ready`
- `GET /metrics`

### `GET /health`

Returns service status.

### `GET /ready`

Returns `200 OK` when PostgreSQL is reachable and `503 Service Unavailable` when it is not.

### `GET /metrics`

Returns lightweight runtime counters:

```json
{
  "uptime_seconds": 120,
  "started_at": "2026-09-10T10:00:00Z",
  "total_requests": 42,
  "total_server_errors": 0
}
```

### `POST /api/v1/payments`

Request body:

```json
{
  "amount_cents": 1299,
  "currency": "USD",
  "description": "Test payment",
  "external_reference": "checkout_123"
}
```

Headers:

- `Idempotency-Key`: optional, recommended for client retry safety

Idempotency behavior:

- same key and same normalized request returns the original payment
- same key and different normalized request returns `409 Conflict`
- requests without an idempotency key always create a new payment

Success response:

```json
{
  "id": "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
  "amount_cents": 1299,
  "currency": "USD",
  "status": "pending",
  "description": "Test payment",
  "external_reference": "checkout_123",
  "idempotency_key": "demo-key-001",
  "created_at": "2026-08-29T13:00:00Z",
  "updated_at": "2026-08-29T13:00:00Z"
}
```

### `GET /api/v1/payments/{id}`

Returns one payment by UUID.

Missing payments return:

```json
{
  "error": "payment not found",
  "request_id": "example-request-id"
}
```

### `PATCH /api/v1/payments/{id}/status`

Updates a payment status.

Request body:

```json
{
  "status": "succeeded"
}
```

Allowed statuses:

- `pending`
- `succeeded`
- `failed`

Transition rules:

- `pending` can move to `succeeded`
- `pending` can move to `failed`
- repeating the current status is allowed
- terminal statuses cannot move to another status

Invalid transitions return:

```json
{
  "error": "invalid payment status transition",
  "request_id": "example-request-id"
}
```

### `GET /api/v1/ledger/balances`

Returns current ledger balances grouped by account and currency:

```json
{
  "balances": [
    {
      "account_id": "account-id",
      "account_name": "finflow_cash",
      "currency": "USD",
      "amount_cents": 1299,
      "as_of": "2026-09-14T12:00:00Z"
    }
  ]
}
```

### `GET /api/v1/ledger/payments/{payment_id}/entries`

Returns ledger entries created for one payment reference:

```json
{
  "entries": [
    {
      "id": "entry-id",
      "transaction_id": "transaction-id",
      "account_id": "account-id",
      "direction": "debit",
      "amount_cents": 1299,
      "currency": "USD",
      "reference_type": "payment",
      "reference_id": "payment-id",
      "created_at": "2026-09-14T12:00:00Z"
    }
  ]
}
```

## Next milestone

Milestone 20 should add production-grade observability:

- Prometheus-format metrics for all APIs and workers
- Kafka publish/consume success, failure, and lag signals
- database operation latency and error metrics
- OpenTelemetry trace propagation across HTTP and Kafka events
- dashboards and alerting guidance for payment and ledger health
