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

func (r *PostgresLedgerRepository) ListBalances(ctx context.Context) ([]model.Balance, error) {
	const query = `
SELECT
	a.id,
	a.name,
	a.currency,
	COALESCE(SUM(CASE
		WHEN e.direction = a.normal_balance THEN e.amount_cents
		ELSE -e.amount_cents
	END), 0) AS amount_cents,
	now() AS as_of
FROM ledger_accounts a
LEFT JOIN ledger_entries e ON e.account_id = a.id
GROUP BY a.id, a.name, a.currency
ORDER BY a.name, a.currency`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list ledger balances: %w", err)
	}
	defer rows.Close()

	balances := []model.Balance{}
	for rows.Next() {
		var balance model.Balance
		if err := rows.Scan(
			&balance.AccountID,
			&balance.AccountName,
			&balance.Currency,
			&balance.AmountCents,
			&balance.AsOf,
		); err != nil {
			return nil, fmt.Errorf("scan ledger balance: %w", err)
		}
		balances = append(balances, balance)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ledger balances: %w", err)
	}

	return balances, nil
}

func (r *PostgresLedgerRepository) ListEntriesByReference(ctx context.Context, referenceType string, referenceID string) ([]model.Entry, error) {
	const query = `
SELECT id, transaction_id, account_id, direction, amount_cents, currency, reference_type, reference_id, created_at
FROM ledger_entries
WHERE reference_type = $1 AND reference_id = $2
ORDER BY created_at, id`

	rows, err := r.pool.Query(ctx, query, referenceType, referenceID)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries by reference: %w", err)
	}
	defer rows.Close()

	entries := []model.Entry{}
	for rows.Next() {
		var entry model.Entry
		if err := rows.Scan(
			&entry.ID,
			&entry.TransactionID,
			&entry.AccountID,
			&entry.Direction,
			&entry.AmountCents,
			&entry.Currency,
			&entry.ReferenceType,
			&entry.ReferenceID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ledger entries: %w", err)
	}

	return entries, nil
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
