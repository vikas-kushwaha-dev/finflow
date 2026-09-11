package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/event"
)

type OutboxRepository interface {
	ClaimPending(ctx context.Context, limit int) ([]event.OutboxEvent, error)
	MarkPublished(ctx context.Context, id string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id string, reason string) error
}

func (r *PostgresPaymentRepository) ClaimPending(ctx context.Context, limit int) ([]event.OutboxEvent, error) {
	if limit <= 0 {
		limit = 25
	}

	const query = `
WITH claimed AS (
	SELECT id
	FROM outbox_events
	WHERE status IN ('pending', 'failed')
	ORDER BY created_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events
SET status = 'publishing',
	attempts = attempts + 1,
	last_error = NULL
WHERE id IN (SELECT id FROM claimed)
RETURNING id, aggregate_type, aggregate_id, event_type, payload, status, attempts, COALESCE(last_error, ''), created_at, published_at`

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim outbox events transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()

	events := []event.OutboxEvent{}
	for rows.Next() {
		var outboxEvent event.OutboxEvent
		if err := rows.Scan(
			&outboxEvent.ID,
			&outboxEvent.AggregateType,
			&outboxEvent.AggregateID,
			&outboxEvent.EventType,
			&outboxEvent.Payload,
			&outboxEvent.Status,
			&outboxEvent.Attempts,
			&outboxEvent.LastError,
			&outboxEvent.CreatedAt,
			&outboxEvent.PublishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, outboxEvent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim outbox events transaction: %w", err)
	}

	return events, nil
}

func (r *PostgresPaymentRepository) MarkPublished(ctx context.Context, id string, publishedAt time.Time) error {
	const query = `
UPDATE outbox_events
SET status = 'published',
	published_at = $2,
	last_error = NULL
WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, id, publishedAt.UTC()); err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}

	return nil
}

func (r *PostgresPaymentRepository) MarkFailed(ctx context.Context, id string, reason string) error {
	const query = `
UPDATE outbox_events
SET status = 'failed',
	last_error = $2
WHERE id = $1`

	if _, err := r.pool.Exec(ctx, query, id, reason); err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}

	return nil
}
