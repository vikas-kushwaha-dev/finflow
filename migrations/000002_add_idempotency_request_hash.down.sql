DROP INDEX IF EXISTS idx_payments_idempotency_request_hash;

ALTER TABLE payments
DROP COLUMN IF EXISTS idempotency_request_hash;

