-- Settlement Service: positions and fraud_alerts
-- Tracks each user's aggregated stake per outcome per market.

CREATE TYPE position_status AS ENUM ('open', 'settling', 'settled', 'refunded');

CREATE TABLE positions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID         NOT NULL,
    market_id     UUID         NOT NULL,
    outcome_index INTEGER      NOT NULL CHECK (outcome_index >= 0),
    stake_minor   BIGINT       NOT NULL DEFAULT 0 CHECK (stake_minor >= 0),
    currency      VARCHAR(10)  NOT NULL DEFAULT 'COINS',
    status        position_status NOT NULL DEFAULT 'open',
    order_count   INTEGER      NOT NULL DEFAULT 1 CHECK (order_count >= 0),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- Natural key: one aggregated position per user/market/outcome
    CONSTRAINT positions_user_market_outcome_uk UNIQUE (user_id, market_id, outcome_index)
);

-- Indexes for common query patterns
CREATE INDEX idx_positions_market_id   ON positions (market_id);
CREATE INDEX idx_positions_user_id     ON positions (user_id);
CREATE INDEX idx_positions_market_open ON positions (market_id) WHERE status = 'open';

-- Auto-update updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER positions_updated_at
    BEFORE UPDATE ON positions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Fraud alerts table
CREATE TABLE fraud_alerts (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    market_id     UUID         NOT NULL,
    outcome_index INTEGER      NOT NULL,
    reason        TEXT         NOT NULL,
    severity      VARCHAR(20)  NOT NULL CHECK (severity IN ('low', 'medium', 'high')),
    detected_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_alerts_market_id ON fraud_alerts (market_id);
CREATE INDEX idx_fraud_alerts_detected  ON fraud_alerts (detected_at DESC);
