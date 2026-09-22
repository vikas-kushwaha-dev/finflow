ALTER TABLE outbox_events
DROP COLUMN IF EXISTS trace_state,
DROP COLUMN IF EXISTS trace_parent;
