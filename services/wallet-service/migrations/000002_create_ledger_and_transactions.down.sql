DROP FUNCTION IF EXISTS apply_double_entry(UUID, UUID, UUID, entry_type, BIGINT, TEXT);
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS transactions;
DROP TYPE IF EXISTS entry_type;
DROP TYPE IF EXISTS txn_status;
DROP TYPE IF EXISTS txn_type;
