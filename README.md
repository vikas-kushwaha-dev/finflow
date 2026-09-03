# FinFlow

FinFlow is a fintech/payments learning project built incrementally with production-style boundaries.

Milestone 1 contains a Go payment service backed by PostgreSQL. Kafka and Kubernetes are intentionally not implemented yet; they come after the first service is working and tested.

## What exists now

- Go payment service in `services/payment-service`
- Chi HTTP router
- PostgreSQL connection via `pgxpool`
- Docker Compose PostgreSQL setup
- SQL migration runner for the `payments` table
- Repository, service, handler, model, config, and database packages
- `GET /health`
- `POST /api/v1/payments`
- `GET /api/v1/payments/{id}`
- request validation
- optional idempotency support through the `Idempotency-Key` header
- JSON error responses with request IDs
- structured server logs
- PostgreSQL integration test entry point
- unit tests for service and handler behavior

## Requirements

- Go 1.26+
- Docker Desktop

## Run locally

From the project root:

```bash
copy .env.example .env
docker compose up -d
cd services/payment-service
go run ./cmd/migrate
go run ./cmd/api
```

Then in another terminal:

```bash
curl http://localhost:8080/health
```

Create a payment:

```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: demo-key-001" \
  -d "{\"amount_cents\":1299,\"currency\":\"USD\",\"description\":\"Test payment\"}"
```

Send the same request with the same `Idempotency-Key` to receive the original payment instead of creating a duplicate.

Retrieve a payment:

```bash
curl http://localhost:8080/api/v1/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8
```

## Test

From `services/payment-service`:

```bash
go test ./...
```

Run PostgreSQL integration tests after starting Docker Compose:

```bash
$env:INTEGRATION_DATABASE_URL="postgres://finflow:finflow@localhost:5432/finflow?sslmode=disable"
go test ./internal/repository
```

## API

### `GET /health`

Returns service status.

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

## Next milestone

Milestone 4 should add payment status lifecycle:

- `PATCH /api/v1/payments/{id}/status`
- valid payment status transition rules
- `updated_at` handling on status changes
- service and handler tests for status behavior
