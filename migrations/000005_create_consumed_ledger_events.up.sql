CREATE TABLE IF NOT EXISTS consumed_ledger_events (
    event_id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    consumed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_consumed_ledger_events_aggregate
ON consumed_ledger_events (aggregate_id);

