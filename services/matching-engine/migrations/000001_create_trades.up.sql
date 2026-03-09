-- Match type enum
CREATE TYPE match_type AS ENUM ('clob', 'amm');

-- Trades: immutable record of every matched trade
CREATE TABLE IF NOT EXISTS trades (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id       UUID        NOT NULL,
    buyer_id        UUID        NOT NULL,
    -- seller_id is NULL for AMM fills (engine is the counterparty)
    seller_id       UUID,
    price_minor     BIGINT      NOT NULL CHECK (price_minor > 0),
    quantity        BIGINT      NOT NULL CHECK (quantity > 0),
    outcome_index   INT         NOT NULL CHECK (outcome_index >= 0),
    match_type      match_type  NOT NULL DEFAULT 'clob',
    -- Per-market monotonically increasing sequence number for deterministic ordering
    seq_no          BIGINT      NOT NULL,
    matched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup by market (most common query pattern)
CREATE INDEX idx_trades_market_id ON trades (market_id);

-- Fast lookup by buyer (user portfolio)
CREATE INDEX idx_trades_buyer_id ON trades (buyer_id);

-- Ordered replay: market_id + seq_no (WAL consistency check)
CREATE UNIQUE INDEX idx_trades_market_seq ON trades (market_id, seq_no);

-- Time-series analytics
CREATE INDEX idx_trades_matched_at ON trades (matched_at DESC);
