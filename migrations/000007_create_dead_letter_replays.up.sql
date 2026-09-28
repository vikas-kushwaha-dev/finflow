CREATE TABLE IF NOT EXISTS dead_letter_replays (
    id UUID PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    operator_name TEXT NOT NULL,
    reason TEXT NOT NULL,
    source_topic TEXT NOT NULL,
    source_partition INTEGER NOT NULL,
    source_offset BIGINT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('started', 'succeeded', 'failed')),
    last_error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dead_letter_replays_status_started_at
ON dead_letter_replays (status, started_at);
