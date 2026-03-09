-- Rollback: Responsible Gambling Features

DROP TABLE IF EXISTS country_rg_policy CASCADE;
DROP TABLE IF EXISTS user_loss_tracking CASCADE;
DROP TABLE IF EXISTS daily_deposit_tracking CASCADE;
DROP TABLE IF EXISTS user_exclusion_settings CASCADE;
DROP TABLE IF EXISTS user_deposit_settings CASCADE;
