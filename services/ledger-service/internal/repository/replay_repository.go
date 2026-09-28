package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrReplayAlreadyClaimed = errors.New("dead-letter event was already claimed for replay")

type ReplaySource struct {
	EventID   string
	Operator  string
	Reason    string
	Topic     string
	Partition int
	Offset    int64
}

type ReplayAuditRepository struct {
	pool *pgxpool.Pool
}

func NewReplayAuditRepository(pool *pgxpool.Pool) *ReplayAuditRepository {
	return &ReplayAuditRepository{pool: pool}
}

func (r *ReplayAuditRepository) StartReplay(ctx context.Context, source ReplaySource) (string, error) {
	const query = `
INSERT INTO dead_letter_replays (
    id, event_id, operator_name, reason, source_topic, source_partition, source_offset, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'started')
ON CONFLICT (event_id) DO UPDATE
SET operator_name = EXCLUDED.operator_name,
    reason = EXCLUDED.reason,
    source_topic = EXCLUDED.source_topic,
    source_partition = EXCLUDED.source_partition,
    source_offset = EXCLUDED.source_offset,
    status = 'started',
    last_error = NULL,
    started_at = now(),
    completed_at = NULL
WHERE dead_letter_replays.status = 'failed'
RETURNING id`

	id := uuid.NewString()
	if err := r.pool.QueryRow(ctx, query, id, source.EventID, source.Operator, source.Reason, source.Topic, source.Partition, source.Offset).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrReplayAlreadyClaimed
		}
		return "", fmt.Errorf("start dead-letter replay audit: %w", err)
	}
	return id, nil
}

func (r *ReplayAuditRepository) FinishReplay(ctx context.Context, id string, succeeded bool, failure string) error {
	status := "failed"
	if succeeded {
		status = "succeeded"
	}
	const query = `
UPDATE dead_letter_replays
SET status = $2, last_error = NULLIF($3, ''), completed_at = now()
WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id, status, failure)
	if err != nil {
		return fmt.Errorf("finish dead-letter replay audit: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("finish dead-letter replay audit: audit %s not found", id)
	}
	return nil
}
