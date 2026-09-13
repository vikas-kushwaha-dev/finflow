package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
)

type PostgresLedgerRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresLedgerRepository(pool *pgxpool.Pool) *PostgresLedgerRepository {
	return &PostgresLedgerRepository{pool: pool}
}

func (r *PostgresLedgerRepository) EnsureAccount(ctx context.Context, name string, currency string, normalBalance model.Direction) (model.Account, error) {
	const query = `
INSERT INTO ledger_accounts (id, name, currency, normal_balance)
VALUES ($1, $2, $3, $4)
ON CONFLICT (name, currency) DO UPDATE
SET name = EXCLUDED.name
RETURNING id, name, currency, normal_balance, created_at`

	var account model.Account
	if err := r.pool.QueryRow(ctx, query, uuid.NewString(), name, currency, normalBalance).Scan(
		&account.ID,
		&account.Name,
		&account.Currency,
		&account.NormalBalance,
		&account.CreatedAt,
	); err != nil {
		return model.Account{}, fmt.Errorf("ensure ledger account: %w", err)
	}

	return account, nil
}

func (r *PostgresLedgerRepository) CreateTransaction(ctx context.Context, entries []model.Entry) ([]model.Entry, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin ledger transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created := make([]model.Entry, 0, len(entries))
	for _, entry := range entries {
		createdEntry, err := insertEntry(ctx, tx, entry)
		if err != nil {
			return nil, err
		}
		created = append(created, createdEntry)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ledger transaction: %w", err)
	}

	return created, nil
}

func (r *PostgresLedgerRepository) CreateTransactionOnce(ctx context.Context, eventID string, eventType string, aggregateID string, entries []model.Entry) ([]model.Entry, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("begin idempotent ledger transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const consumedQuery = `
INSERT INTO consumed_ledger_events (event_id, event_type, aggregate_id)
VALUES ($1, $2, $3)
ON CONFLICT (event_id) DO NOTHING`

	tag, err := tx.Exec(ctx, consumedQuery, eventID, eventType, aggregateID)
	if err != nil {
		return nil, false, fmt.Errorf("record consumed ledger event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, fmt.Errorf("commit duplicate ledger event transaction: %w", err)
		}
		return nil, false, nil
	}

	created := make([]model.Entry, 0, len(entries))
	for _, entry := range entries {
		createdEntry, err := insertEntry(ctx, tx, entry)
		if err != nil {
			return nil, false, err
		}
		created = append(created, createdEntry)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit idempotent ledger transaction: %w", err)
	}

	return created, true, nil
}

func insertEntry(ctx context.Context, tx pgx.Tx, entry model.Entry) (model.Entry, error) {
	const query = `
INSERT INTO ledger_entries (
	id,
	transaction_id,
	account_id,
	direction,
	amount_cents,
	currency,
	reference_type,
	reference_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, transaction_id, account_id, direction, amount_cents, currency, reference_type, reference_id, created_at`

	var created model.Entry
	if err := tx.QueryRow(ctx, query,
		entry.ID,
		entry.TransactionID,
		entry.AccountID,
		entry.Direction,
		entry.AmountCents,
		entry.Currency,
		entry.ReferenceType,
		entry.ReferenceID,
	).Scan(
		&created.ID,
		&created.TransactionID,
		&created.AccountID,
		&created.Direction,
		&created.AmountCents,
		&created.Currency,
		&created.ReferenceType,
		&created.ReferenceID,
		&created.CreatedAt,
	); err != nil {
		return model.Entry{}, fmt.Errorf("insert ledger entry: %w", err)
	}

	return created, nil
}
