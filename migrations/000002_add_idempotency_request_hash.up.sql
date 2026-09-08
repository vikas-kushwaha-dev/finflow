ALTER TABLE payments
ADD COLUMN IF NOT EXISTS idempotency_request_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_payments_idempotency_request_hash
ON payments (idempotency_request_hash)
WHERE idempotency_request_hash IS NOT NULL;

