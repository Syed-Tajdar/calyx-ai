REVOKE EXECUTE ON FUNCTION insert_writeback_audit(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT) FROM callloom_sync;
DROP FUNCTION IF EXISTS insert_writeback_audit(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT);
DROP TRIGGER IF EXISTS writeback_audit_no_update ON writeback_audit;
DROP FUNCTION IF EXISTS writeback_audit_immutable();
DROP TABLE IF EXISTS writeback_audit;
