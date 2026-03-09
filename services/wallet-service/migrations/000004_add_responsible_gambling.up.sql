-- Responsible Gambling Features
-- Deposit limits, cool-off periods, self-exclusion, loss tracking

-- User deposit settings: daily/monthly deposit limits
CREATE TABLE IF NOT EXISTS user_deposit_settings (
    user_id               UUID        PRIMARY KEY,
    daily_deposit_limit_minor   BIGINT      NOT NULL DEFAULT 5000000,  -- $50 USD equivalent
    monthly_deposit_limit_minor BIGINT,
    enabled               BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_deposit_settings_enabled
    ON user_deposit_settings (enabled);

CREATE TRIGGER user_deposit_settings_updated_at
    BEFORE UPDATE ON user_deposit_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- User exclusion settings: cool-off periods and self-exclusion
CREATE TABLE IF NOT EXISTS user_exclusion_settings (
    user_id               UUID        PRIMARY KEY,
    country_code          VARCHAR(2),  -- 'IN', 'NG', 'KE', 'PH', etc.
    cool_off_until        TIMESTAMPTZ,  -- NULL = not in cool-off
    cool_off_duration_hours INT,  -- 24, 168 (7d), 720 (30d)
    cool_off_cancellable  BOOLEAN     DEFAULT FALSE,  -- Whether user can cancel in their region
    is_self_excluded      BOOLEAN     NOT NULL DEFAULT FALSE,
    self_excluded_at      TIMESTAMPTZ,
    self_exclusion_duration_days INT,  -- NULL = permanent
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_exclusion_cool_off_until
    ON user_exclusion_settings (cool_off_until);

CREATE INDEX IF NOT EXISTS idx_user_exclusion_self_excluded
    ON user_exclusion_settings (is_self_excluded);

CREATE TRIGGER user_exclusion_settings_updated_at
    BEFORE UPDATE ON user_exclusion_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Daily deposit tracking: running total for each day
CREATE TABLE IF NOT EXISTS daily_deposit_tracking (
    user_id               UUID        NOT NULL,
    tracked_date          DATE        NOT NULL,
    total_deposited_minor BIGINT      NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, tracked_date),
    FOREIGN KEY (user_id) REFERENCES user_deposit_settings(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_daily_deposit_tracking_user_date
    ON daily_deposit_tracking (user_id, tracked_date DESC);

CREATE TRIGGER daily_deposit_tracking_updated_at
    BEFORE UPDATE ON daily_deposit_tracking
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- User loss tracking: consecutive loss counter and notification threshold
CREATE TABLE IF NOT EXISTS user_loss_tracking (
    user_id                          UUID        PRIMARY KEY,
    loss_streak_notification_threshold INT        NOT NULL DEFAULT 3,  -- 1-10, user-configurable
    consecutive_losses               INT         NOT NULL DEFAULT 0,
    last_position_timestamp          TIMESTAMPTZ,
    alert_sent_at                    TIMESTAMPTZ,  -- NULL until first alert
    created_at                       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_loss_tracking_consecutive_losses
    ON user_loss_tracking (consecutive_losses);

CREATE TRIGGER user_loss_tracking_updated_at
    BEFORE UPDATE ON user_loss_tracking
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Country RG policies: define cancellation rules and limits per region
CREATE TABLE IF NOT EXISTS country_rg_policy (
    country_code VARCHAR(2)  PRIMARY KEY,
    cool_off_cancellable BOOLEAN      NOT NULL DEFAULT FALSE,  -- Can users cancel cool-offs?
    max_daily_deposit_limit_minor BIGINT,  -- Maximum user can set
    max_cool_off_duration_hours INT    DEFAULT 2592000,  -- Max: 30 days
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER country_rg_policy_updated_at
    BEFORE UPDATE ON country_rg_policy
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed initial country policies (India allows cancellation, others don't)
INSERT INTO country_rg_policy (country_code, cool_off_cancellable, max_daily_deposit_limit_minor, max_cool_off_duration_hours)
VALUES
  ('IN', true, 500000000, 2592000),    -- India: can cancel, max ~$5000
  ('NG', false, 300000000, 2592000),   -- Nigeria: cannot cancel, max ~$3000
  ('KE', false, 300000000, 2592000),   -- Kenya: cannot cancel, max ~$3000
  ('PH', false, 400000000, 2592000),   -- Philippines: cannot cancel, max ~$4000
  ('US', true, 1000000000, 2592000),   -- US: can cancel, max ~$10000
  ('GB', true, 800000000, 2592000)     -- UK: can cancel, max ~$8000
ON CONFLICT (country_code) DO NOTHING;
