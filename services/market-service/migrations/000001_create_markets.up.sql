-- Market Service: Initial schema

CREATE TYPE market_status AS ENUM (
    'draft',
    'active',
    'suspended',
    'pending_resolution',
    'resolved',
    'voided',
    'archived'
);

CREATE TYPE market_category AS ENUM (
    'sports',
    'entertainment',
    'politics',
    'weather',
    'finance',
    'trending',
    'local'
);

CREATE TYPE creator_type AS ENUM ('admin', 'user');

CREATE TABLE markets (
    id                  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    title               VARCHAR(500)    NOT NULL,
    question            TEXT            NOT NULL,
    resolution_criteria TEXT            NOT NULL,
    category            market_category NOT NULL,
    status              market_status   NOT NULL DEFAULT 'draft',
    creator_id          UUID            NOT NULL,
    creator_type        creator_type    NOT NULL DEFAULT 'admin',
    pool_amount_minor   BIGINT          NOT NULL DEFAULT 0,
    currency            VARCHAR(10)     NOT NULL DEFAULT 'COINS',
    closes_at           TIMESTAMPTZ     NOT NULL,
    resolves_at         TIMESTAMPTZ,
    metadata            JSONB           NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- Fast lookups for Resolution Service and listing APIs
CREATE INDEX idx_markets_status       ON markets (status);
CREATE INDEX idx_markets_category     ON markets (category);
CREATE INDEX idx_markets_closes_at    ON markets (closes_at) WHERE status = 'active';
CREATE INDEX idx_markets_creator      ON markets (creator_id);
CREATE INDEX idx_markets_status_cat   ON markets (status, category);

-- Auto-update updated_at on every write
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER markets_updated_at
    BEFORE UPDATE ON markets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
