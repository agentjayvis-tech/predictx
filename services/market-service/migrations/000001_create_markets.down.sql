DROP TRIGGER IF EXISTS markets_updated_at ON markets;
DROP FUNCTION IF EXISTS update_updated_at();
DROP TABLE IF EXISTS markets;
DROP TYPE IF EXISTS creator_type;
DROP TYPE IF EXISTS market_category;
DROP TYPE IF EXISTS market_status;
