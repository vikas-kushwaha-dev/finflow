CREATE TABLE IF NOT EXISTS ledger_accounts (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    currency CHAR(3) NOT NULL,
    normal_balance TEXT NOT NULL CHECK (normal_balance IN ('debit', 'credit')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, currency)
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY,
    transaction_id UUID NOT NULL,
    account_id UUID NOT NULL REFERENCES ledger_accounts(id),
    direction TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency CHAR(3) NOT NULL,
    reference_type TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_transaction_id ON ledger_entries (transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_reference ON ledger_entries (reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_account_id ON ledger_entries (account_id);
