-- Drop tables in reverse order of creation
DROP TRIGGER IF EXISTS rg_limits_updated_at ON rg_limits;
DROP TABLE IF EXISTS rg_limits;

DROP TRIGGER IF EXISTS orders_updated_at ON orders;
DROP TABLE IF EXISTS orders;

-- Drop function used by triggers
DROP FUNCTION IF EXISTS update_updated_at();

-- Drop ENUM types
DROP TYPE IF EXISTS time_in_force;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_type;
