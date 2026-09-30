CREATE INDEX IF NOT EXISTS idx_outbox_events_published_retention
ON outbox_events (published_at)
WHERE status = 'published';

CREATE INDEX IF NOT EXISTS idx_consumed_ledger_events_retention
ON consumed_ledger_events (consumed_at);

CREATE INDEX IF NOT EXISTS idx_dead_letter_replays_completed_retention
ON dead_letter_replays (completed_at)
WHERE status IN ('succeeded', 'failed');
