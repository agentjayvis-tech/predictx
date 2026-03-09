-- Transaction types
CREATE TYPE txn_type AS ENUM (
    'deposit',      -- COINS granted (admin gift, daily reward, purchase)
    'spend',        -- COINS used to place a prediction
    'refund',       -- COINS returned when a market is voided/disputed
    'payout',       -- COINS awarded on winning prediction
    'daily_reward', -- daily login bonus
    'adjustment'    -- admin correction
);

CREATE TYPE txn_status AS ENUM (
    'pending',
    'completed',
    'failed',
    'reversed'
);

CREATE TYPE entry_type AS ENUM ('credit', 'debit');

-- transactions: one row per economic event, with idempotency key for dedup.
CREATE TABLE IF NOT EXISTS transactions (
    id              UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    txn_type        txn_type     NOT NULL,
    status          txn_status   NOT NULL DEFAULT 'completed',
    currency        VARCHAR(10)  NOT NULL,
    amount_minor    BIGINT       NOT NULL CHECK (amount_minor > 0),
    description     TEXT,
    reference_id    UUID,
    reference_type  VARCHAR(50),
    metadata        JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    PRIMARY KEY (user_id, id),
    UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_created
    ON transactions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_transactions_reference
    ON transactions (reference_type, reference_id)
    WHERE reference_id IS NOT NULL;

-- ledger_entries: immutable double-entry audit log.
-- Every balance change produces one row (single-sided, user-perspective).
CREATE TABLE IF NOT EXISTS ledger_entries (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL,
    wallet_id           UUID        NOT NULL,
    transaction_id      UUID        NOT NULL,
    entry_type          entry_type  NOT NULL,
    amount_minor        BIGINT      NOT NULL CHECK (amount_minor > 0),
    balance_after_minor BIGINT      NOT NULL CHECK (balance_after_minor >= 0),
    currency            VARCHAR(10) NOT NULL,
    description         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id)
);

CREATE INDEX IF NOT EXISTS idx_ledger_wallet_created
    ON ledger_entries (user_id, wallet_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ledger_transaction
    ON ledger_entries (user_id, transaction_id);

-- apply_double_entry: atomic stored procedure.
--   1. Locks wallet row (FOR UPDATE)
--   2. Checks sufficient balance for debits
--   3. Updates wallet.balance_minor
--   4. Inserts a ledger_entry
-- Returns new balance. Raises 'insufficient_funds' exception on underfunded debit.
CREATE OR REPLACE FUNCTION apply_double_entry(
    p_user_id        UUID,
    p_wallet_id      UUID,
    p_transaction_id UUID,
    p_entry_type     entry_type,
    p_amount_minor   BIGINT,
    p_description    TEXT
) RETURNS BIGINT LANGUAGE plpgsql AS $$
DECLARE
    v_balance  BIGINT;
    v_currency VARCHAR(10);
BEGIN
    SELECT balance_minor, currency
    INTO v_balance, v_currency
    FROM wallets
    WHERE user_id = p_user_id AND id = p_wallet_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'wallet_not_found';
    END IF;

    IF p_entry_type = 'debit' AND v_balance < p_amount_minor THEN
        RAISE EXCEPTION 'insufficient_funds';
    END IF;

    IF p_entry_type = 'credit' THEN
        v_balance := v_balance + p_amount_minor;
    ELSE
        v_balance := v_balance - p_amount_minor;
    END IF;

    UPDATE wallets
    SET balance_minor = v_balance
    WHERE user_id = p_user_id AND id = p_wallet_id;

    INSERT INTO ledger_entries (
        user_id, wallet_id, transaction_id, entry_type,
        amount_minor, balance_after_minor, currency, description
    ) VALUES (
        p_user_id, p_wallet_id, p_transaction_id, p_entry_type,
        p_amount_minor, v_balance, v_currency, p_description
    );

    RETURN v_balance;
END;
$$;
