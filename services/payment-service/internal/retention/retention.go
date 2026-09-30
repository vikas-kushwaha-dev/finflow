package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Rule struct {
	Name      string
	Retention time.Duration
	CountSQL  string
	DeleteSQL string
}

type Result struct {
	Name     string
	Eligible int64
	Deleted  int64
	DryRun   bool
}

func Rules(outboxRetention time.Duration, consumedRetention time.Duration, replayRetention time.Duration) []Rule {
	return []Rule{
		{
			Name: "published_outbox_events", Retention: outboxRetention,
			CountSQL: `SELECT count(*) FROM outbox_events WHERE status = 'published' AND published_at < $1`,
			DeleteSQL: `
WITH candidates AS (
    SELECT ctid FROM outbox_events
    WHERE status = 'published' AND published_at < $1
    LIMIT $2 FOR UPDATE SKIP LOCKED
), deleted AS (
    DELETE FROM outbox_events WHERE ctid IN (SELECT ctid FROM candidates)
    RETURNING 1
)
SELECT count(*) FROM deleted`,
		},
		{
			Name: "consumed_ledger_events", Retention: consumedRetention,
			CountSQL: `SELECT count(*) FROM consumed_ledger_events WHERE consumed_at < $1`,
			DeleteSQL: `
WITH candidates AS (
    SELECT ctid FROM consumed_ledger_events
    WHERE consumed_at < $1
    LIMIT $2 FOR UPDATE SKIP LOCKED
), deleted AS (
    DELETE FROM consumed_ledger_events WHERE ctid IN (SELECT ctid FROM candidates)
    RETURNING 1
)
SELECT count(*) FROM deleted`,
		},
		{
			Name: "completed_dead_letter_replays", Retention: replayRetention,
			CountSQL: `SELECT count(*) FROM dead_letter_replays WHERE status IN ('succeeded', 'failed') AND completed_at < $1`,
			DeleteSQL: `
WITH candidates AS (
    SELECT ctid FROM dead_letter_replays
    WHERE status IN ('succeeded', 'failed') AND completed_at < $1
    LIMIT $2 FOR UPDATE SKIP LOCKED
), deleted AS (
    DELETE FROM dead_letter_replays WHERE ctid IN (SELECT ctid FROM candidates)
    RETURNING 1
)
SELECT count(*) FROM deleted`,
		},
	}
}

func RunRule(ctx context.Context, db Queryer, rule Rule, now time.Time, batchSize int, dryRun bool) (Result, error) {
	if rule.Retention <= 0 {
		return Result{}, fmt.Errorf("%s retention must be greater than zero", rule.Name)
	}
	if batchSize < 1 {
		return Result{}, fmt.Errorf("batch size must be at least 1")
	}
	cutoff := now.UTC().Add(-rule.Retention)
	result := Result{Name: rule.Name, DryRun: dryRun}
	if err := db.QueryRow(ctx, rule.CountSQL, cutoff).Scan(&result.Eligible); err != nil {
		return Result{}, fmt.Errorf("count %s: %w", rule.Name, err)
	}
	if dryRun {
		return result, nil
	}
	for {
		var deleted int64
		if err := db.QueryRow(ctx, rule.DeleteSQL, cutoff, batchSize).Scan(&deleted); err != nil {
			return Result{}, fmt.Errorf("delete %s batch: %w", rule.Name, err)
		}
		result.Deleted += deleted
		if deleted < int64(batchSize) {
			return result, nil
		}
	}
}
