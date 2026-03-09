-- Settlement records: one per resolved market.
-- Settlement entries: one per position (payout or skip for losers).

CREATE TYPE settlement_status AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TYPE entry_status      AS ENUM ('pending', 'paid', 'failed', 'skipped');

CREATE TABLE settlements (
    id                   UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id            UUID              NOT NULL UNIQUE, -- one settlement per market
    resolution_id        VARCHAR(255)      NOT NULL,
    status               settlement_status NOT NULL DEFAULT 'pending',
    winning_outcome      INTEGER           NOT NULL CHECK (winning_outcome >= 0),
    total_pool_minor     BIGINT            NOT NULL DEFAULT 0 CHECK (total_pool_minor >= 0),
    insurance_fee_minor  BIGINT            NOT NULL DEFAULT 0 CHECK (insurance_fee_minor >= 0),
    net_pool_minor       BIGINT            NOT NULL DEFAULT 0 CHECK (net_pool_minor >= 0),
    winning_stake_minor  BIGINT            NOT NULL DEFAULT 0 CHECK (winning_stake_minor >= 0),
    winner_count         INTEGER           NOT NULL DEFAULT 0,
    loser_count          INTEGER           NOT NULL DEFAULT 0,
    currency             VARCHAR(10)       NOT NULL DEFAULT 'COINS',
    settled_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ       NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_settlements_market_id ON settlements (market_id);
CREATE INDEX idx_settlements_status    ON settlements (status);

CREATE TRIGGER settlements_updated_at
    BEFORE UPDATE ON settlements
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Settlement entries: one row per position (winner or loser).
CREATE TABLE settlement_entries (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_id   UUID         NOT NULL REFERENCES settlements (id) ON DELETE CASCADE,
    user_id         UUID         NOT NULL,
    position_id     UUID         NOT NULL REFERENCES positions (id),
    stake_minor     BIGINT       NOT NULL DEFAULT 0 CHECK (stake_minor >= 0),
    payout_minor    BIGINT       NOT NULL DEFAULT 0 CHECK (payout_minor >= 0),
    pnl_minor       BIGINT       NOT NULL DEFAULT 0,  -- can be negative for losers
    is_winner       BOOLEAN      NOT NULL DEFAULT FALSE,
    status          entry_status NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,  -- prevents duplicate wallet calls
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_settlement_entries_settlement_id ON settlement_entries (settlement_id);
CREATE INDEX idx_settlement_entries_user_id        ON settlement_entries (user_id);
CREATE INDEX idx_settlement_entries_pending        ON settlement_entries (settlement_id, status)
    WHERE status = 'pending';

CREATE TRIGGER settlement_entries_updated_at
    BEFORE UPDATE ON settlement_entries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
