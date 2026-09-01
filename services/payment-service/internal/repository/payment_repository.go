package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
)

var ErrPaymentNotFound = errors.New("payment not found")

type PaymentRepository interface {
	Create(ctx context.Context, payment model.Payment) (model.Payment, bool, error)
	GetByID(ctx context.Context, id string) (model.Payment, error)
}

type PostgresPaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPaymentRepository(pool *pgxpool.Pool) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{pool: pool}
}

func (r *PostgresPaymentRepository) Create(ctx context.Context, payment model.Payment) (model.Payment, bool, error) {
	if payment.IdempotencyKey == "" {
		return r.insert(ctx, payment)
	}

	const query = `
WITH inserted AS (
	INSERT INTO payments (
		id,
		amount_cents,
		currency,
		status,
		description,
		external_reference,
		idempotency_key
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (idempotency_key) DO NOTHING
	RETURNING id, amount_cents, currency, status, description, external_reference, idempotency_key, created_at, updated_at
)
SELECT id, amount_cents, currency, status, description, external_reference, idempotency_key, created_at, updated_at, true AS created
FROM inserted
UNION ALL
SELECT id, amount_cents, currency, status, description, external_reference, idempotency_key, created_at, updated_at, false AS created
FROM payments
WHERE idempotency_key = $7
LIMIT 1`

	var created bool
	createdPayment, err := scanPayment(r.pool.QueryRow(ctx, query,
		payment.ID,
		payment.AmountCents,
		payment.Currency,
		payment.Status,
		nullableString(payment.Description),
		nullableString(payment.ExternalReference),
		payment.IdempotencyKey,
	), &created)
	if err != nil {
		return model.Payment{}, false, fmt.Errorf("create payment: %w", err)
	}

	return createdPayment, created, nil
}

func (r *PostgresPaymentRepository) GetByID(ctx context.Context, id string) (model.Payment, error) {
	const query = `
SELECT id, amount_cents, currency, status, description, external_reference, idempotency_key, created_at, updated_at
FROM payments
WHERE id = $1`

	payment, err := scanPayment(r.pool.QueryRow(ctx, query, id), nil)
	if err != nil {
		return model.Payment{}, fmt.Errorf("get payment by id: %w", err)
	}

	return payment, nil
}

func (r *PostgresPaymentRepository) insert(ctx context.Context, payment model.Payment) (model.Payment, bool, error) {
	const query = `
INSERT INTO payments (
	id,
	amount_cents,
	currency,
	status,
	description,
	external_reference,
	idempotency_key
)
VALUES ($1, $2, $3, $4, $5, $6, NULL)
RETURNING id, amount_cents, currency, status, description, external_reference, idempotency_key, created_at, updated_at`

	createdPayment, err := scanPayment(r.pool.QueryRow(ctx, query,
		payment.ID,
		payment.AmountCents,
		payment.Currency,
		payment.Status,
		nullableString(payment.Description),
		nullableString(payment.ExternalReference),
	), nil)
	if err != nil {
		return model.Payment{}, false, fmt.Errorf("insert payment: %w", err)
	}

	return createdPayment, true, nil
}

func scanPayment(row pgx.Row, created *bool) (model.Payment, error) {
	var payment model.Payment
	var description *string
	var externalReference *string
	var idempotencyKey *string

	dest := []any{
		&payment.ID,
		&payment.AmountCents,
		&payment.Currency,
		&payment.Status,
		&description,
		&externalReference,
		&idempotencyKey,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	}

	if created != nil {
		dest = append(dest, created)
	}

	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Payment{}, ErrPaymentNotFound
		}
		return model.Payment{}, err
	}

	payment.Description = derefString(description)
	payment.ExternalReference = derefString(externalReference)
	payment.IdempotencyKey = derefString(idempotencyKey)

	return payment, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
