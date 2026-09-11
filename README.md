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
- Double-entry ledger rules
- Ledger account and entry migrations
- Kafka local infrastructure
- payment outbox table
- outbox publisher command
- payment event schemas for `payment.created` and `payment.status_changed`
- Repository, service, handler, model, config, and database packages
- `GET /health`
- `GET /ready`
- `GET /metrics`
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

## Requirements

- Go 1.26+
- Docker Desktop

## Run locally

From the project root:

```bash
copy .env.example .env
docker compose up --build -d
```

The local example API key is `local-dev-api-key-change-me`. Change `API_KEY` before using this outside local development.

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

Kafka is included in Docker Compose for local event publishing. The payment service records events in `outbox_events`, and the outbox publisher sends them to the `finflow.payment.events` topic.

Create a payment:

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-API-Key: local-dev-api-key-change-me" \
  -H "Idempotency-Key: demo-key-001" \
  -d "{\"amount_cents\":1299,\"currency\":\"USD\",\"description\":\"Test payment\"}"
```

Send the same request with the same `Idempotency-Key` to receive the original payment instead of creating a duplicate.

If the same `Idempotency-Key` is reused with different payment details, the API returns `409 Conflict`.

Retrieve a payment:

```bash
curl http://localhost:8080/api/v1/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8 \
  -H "X-API-Key: local-dev-api-key-change-me"
```

Update payment status:

```bash
curl -X PATCH http://localhost:8080/api/v1/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8/status \
  -H "Content-Type: application/json" \
  -H "X-API-Key: local-dev-api-key-change-me" \
  -d "{\"status\":\"succeeded\"}"
```

## Test

From `services/payment-service`:

```bash
go test ./...
```

From `services/ledger-service`:

```bash
go test ./...
```

Run PostgreSQL integration tests after starting Docker Compose:

```bash
$env:INTEGRATION_DATABASE_URL="postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"
go test ./internal/repository
```

## API

Payment endpoints require an API key:

```bash
X-API-Key: local-dev-api-key-change-me
```

You can also send:

```bash
Authorization: Bearer local-dev-api-key-change-me
```

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

## Next milestone

Milestone 11 should add the ledger event consumer:

- consume payment events in the ledger service
- make event handling idempotent
- store consumed event IDs
- create ledger entries from `payment.created`
- add retry/dead-letter behavior
