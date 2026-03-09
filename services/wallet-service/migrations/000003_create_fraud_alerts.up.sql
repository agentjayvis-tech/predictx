CREATE TYPE fraud_alert_type AS ENUM (
    'rapid_changes', -- >10 balance changes in 60 seconds
    'large_credit',  -- single credit > FRAUD_LARGE_CREDIT_THRESHOLD COINS
    'rapid_drain'    -- balance drops >80% in 5 minutes
);

CREATE TABLE IF NOT EXISTS fraud_alerts (
    id         UUID             NOT NULL DEFAULT gen_random_uuid(),
    user_id    UUID             NOT NULL,
    wallet_id  UUID             NOT NULL,
    alert_type fraud_alert_type NOT NULL,
    details    JSONB            NOT NULL DEFAULT '{}',
    resolved   BOOLEAN          NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, id)
);

CREATE INDEX IF NOT EXISTS idx_fraud_alerts_unresolved
    ON fraud_alerts (user_id, created_at)
    WHERE resolved = FALSE;
