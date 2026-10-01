\set ON_ERROR_STOP on
BEGIN TRANSACTION READ ONLY;

\echo 'payment lookup by primary key'
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, amount_cents, currency, status, created_at
FROM payments
WHERE id = (SELECT id FROM payments ORDER BY created_at DESC LIMIT 1);

\echo 'payment idempotency lookup'
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, idempotency_request_hash
FROM payments
WHERE idempotency_key = (
    SELECT idempotency_key FROM payments
    WHERE idempotency_key IS NOT NULL
    ORDER BY created_at DESC LIMIT 1
);

\echo 'ledger entries by payment reference'
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, transaction_id, account_id, direction, amount_cents, currency, created_at
FROM ledger_entries
WHERE reference_type = 'payment'
  AND reference_id = (
      SELECT reference_id FROM ledger_entries
      WHERE reference_type = 'payment'
      ORDER BY created_at DESC LIMIT 1
  )
ORDER BY created_at, id;

\echo 'ledger balances'
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT a.id, a.name, a.currency,
       COALESCE(SUM(CASE WHEN e.direction = a.normal_balance THEN e.amount_cents ELSE -e.amount_cents END), 0)
FROM ledger_accounts a
LEFT JOIN ledger_entries e ON e.account_id = a.id
GROUP BY a.id, a.name, a.currency
ORDER BY a.name, a.currency;

\echo 'published outbox retention scan'
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT count(*) FROM outbox_events
WHERE status = 'published' AND published_at < now() - interval '30 days';

\echo 'consumed event retention scan'
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT count(*) FROM consumed_ledger_events
WHERE consumed_at < now() - interval '90 days';

ROLLBACK;
