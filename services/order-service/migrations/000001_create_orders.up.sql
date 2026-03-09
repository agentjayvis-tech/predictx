-- Create ENUM types for orders
CREATE TYPE order_type AS ENUM ('buy', 'sell');
CREATE TYPE order_status AS ENUM ('pending', 'matched', 'settled', 'cancelled', 'failed');
CREATE TYPE time_in_force AS ENUM ('ioc', 'gtc');

-- Orders table: tracks all user orders across markets
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    market_id UUID NOT NULL,
    order_type order_type NOT NULL,
    status order_status NOT NULL DEFAULT 'pending',
    time_in_force time_in_force NOT NULL DEFAULT 'gtc',
    price_minor BIGINT NOT NULL,          -- Bet amount in minor units (e.g., 100 coins)
    quantity_shares BIGINT NOT NULL DEFAULT 1,  -- For scalar markets
    currency VARCHAR(10) NOT NULL DEFAULT 'COINS',
    outcome_index INT NOT NULL,            -- 0/1 for binary, index for scalar
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT price_positive CHECK (price_minor > 0),
    CONSTRAINT quantity_positive CHECK (quantity_shares > 0)
);

-- Indexes for efficient querying
CREATE INDEX idx_orders_user_id ON orders (user_id);
CREATE INDEX idx_orders_market_id ON orders (market_id);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_user_status ON orders (user_id, status);
CREATE INDEX idx_orders_market_status ON orders (market_id, status);
CREATE INDEX idx_orders_created_at ON orders (created_at DESC);
CREATE INDEX idx_orders_idempotency_key ON orders (idempotency_key);

-- Auto-update trigger for updated_at column
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Responsible Gambling (RG) limits per user
-- Tracks daily and weekly spending to enforce responsible gambling limits
CREATE TABLE rg_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE NOT NULL,
    daily_spent_minor BIGINT NOT NULL DEFAULT 0,   -- Sum of BUY orders today
    weekly_spent_minor BIGINT NOT NULL DEFAULT 0,  -- Sum of BUY orders this week
    daily_reset_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    weekly_reset_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for efficient lookups
CREATE INDEX idx_rg_limits_user_id ON rg_limits (user_id);

-- Auto-update trigger for rg_limits
CREATE TRIGGER rg_limits_updated_at
    BEFORE UPDATE ON rg_limits
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
