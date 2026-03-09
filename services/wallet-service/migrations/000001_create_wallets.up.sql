-- Wallets: one row per (user_id, currency).
-- balance_minor is the authoritative cached balance in smallest currency unit.
-- Sharded by user_id in Citus; composite PK includes user_id for co-location.
-- balance_minor is maintained atomically via apply_double_entry() stored proc.

CREATE TABLE IF NOT EXISTS wallets (
    id            UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL,
    currency      VARCHAR(10) NOT NULL,  -- 'COINS', 'NGN', 'KES', 'PHP', 'USDC'
    balance_minor BIGINT      NOT NULL DEFAULT 0 CHECK (balance_minor >= 0),
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_wallets_currency ON wallets (currency);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
