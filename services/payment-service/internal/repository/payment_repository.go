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
var ErrIdempotencyConflict = errors.New("idempotency key reused with different request")

type PaymentRepository interface {
	Create(ctx context.Context, payment model.Payment) (model.Payment, bool, error)
	GetByID(ctx context.Context, id string) (model.Payment, error)
	UpdateStatus(ctx context.Context, id string, status model.PaymentStatus) (model.Payment, error)
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

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.Payment{}, false, fmt.Errorf("begin idempotent create transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertQuery = `
INSERT INTO payments (
		id,
		amount_cents,
		currency,
		status,
		description,
		external_reference,
		idempotency_key,
		idempotency_request_hash
	)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id, amount_cents, currency, status, description, external_reference, idempotency_key, idempotency_request_hash, created_at, updated_at`

	createdPayment, err := scanPayment(tx.QueryRow(ctx, insertQuery,
		payment.ID,
		payment.AmountCents,
		payment.Currency,
		payment.Status,
		nullableString(payment.Description),
		nullableString(payment.ExternalReference),
		payment.IdempotencyKey,
		payment.IdempotencyHash,
	), nil)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return model.Payment{}, false, fmt.Errorf("commit idempotent create transaction: %w", err)
		}

		return createdPayment, true, nil
	}
	if !errors.Is(err, ErrPaymentNotFound) {
		return model.Payment{}, false, fmt.Errorf("create payment: %w", err)
	}

	const replayQuery = `
SELECT id, amount_cents, currency, status, description, external_reference, idempotency_key, idempotency_request_hash, created_at, updated_at
FROM payments
WHERE idempotency_key = $1`

	existingPayment, err := scanPayment(tx.QueryRow(ctx, replayQuery, payment.IdempotencyKey), nil)
	if err != nil {
		return model.Payment{}, false, fmt.Errorf("find idempotent payment: %w", err)
	}

	if existingPayment.IdempotencyHash != payment.IdempotencyHash {
		return model.Payment{}, false, ErrIdempotencyConflict
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Payment{}, false, fmt.Errorf("commit idempotent replay transaction: %w", err)
	}

	return existingPayment, false, nil
}

func (r *PostgresPaymentRepository) GetByID(ctx context.Context, id string) (model.Payment, error) {
	const query = `
SELECT id, amount_cents, currency, status, description, external_reference, idempotency_key, idempotency_request_hash, created_at, updated_at
FROM payments
WHERE id = $1`

	payment, err := scanPayment(r.pool.QueryRow(ctx, query, id), nil)
	if err != nil {
		return model.Payment{}, fmt.Errorf("get payment by id: %w", err)
	}

	return payment, nil
}

func (r *PostgresPaymentRepository) UpdateStatus(ctx context.Context, id string, status model.PaymentStatus) (model.Payment, error) {
	const query = `
UPDATE payments
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING id, amount_cents, currency, status, description, external_reference, idempotency_key, idempotency_request_hash, created_at, updated_at`

	payment, err := scanPayment(r.pool.QueryRow(ctx, query, id, status), nil)
	if err != nil {
		return model.Payment{}, fmt.Errorf("update payment status: %w", err)
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
	idempotency_key,
	idempotency_request_hash
)
VALUES ($1, $2, $3, $4, $5, $6, NULL, NULL)
RETURNING id, amount_cents, currency, status, description, external_reference, idempotency_key, idempotency_request_hash, created_at, updated_at`

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
	var idempotencyHash *string

	dest := []any{
		&payment.ID,
		&payment.AmountCents,
		&payment.Currency,
		&payment.Status,
		&description,
		&externalReference,
		&idempotencyKey,
		&idempotencyHash,
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
	payment.IdempotencyHash = derefString(idempotencyHash)

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
