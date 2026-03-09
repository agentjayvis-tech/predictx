DROP TABLE IF EXISTS fraud_alerts;
DROP TRIGGER IF EXISTS positions_updated_at ON positions;
DROP TABLE IF EXISTS positions;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TYPE IF EXISTS position_status;
