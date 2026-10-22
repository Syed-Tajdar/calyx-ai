-- Restores the state before 000037: PUBLIC may execute these functions again. The named role grants
-- stay (they existed before 000037 too).
GRANT EXECUTE ON FUNCTION authenticate_user(TEXT, TEXT) TO PUBLIC;
GRANT EXECUTE ON FUNCTION list_workflow_tenants() TO PUBLIC;
GRANT EXECUTE ON FUNCTION lookup_integration_by_token(TEXT) TO PUBLIC;
GRANT EXECUTE ON FUNCTION list_api_sync_integrations() TO PUBLIC;
GRANT EXECUTE ON FUNCTION get_api_sync_integration(UUID, UUID) TO PUBLIC;
GRANT EXECUTE ON FUNCTION get_api_sync_integration_any(UUID, UUID) TO PUBLIC;
GRANT EXECUTE ON FUNCTION insert_writeback_audit(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT) TO PUBLIC;
GRANT EXECUTE ON FUNCTION insert_writeback_audit_v2(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT, TEXT, JSONB) TO PUBLIC;
GRANT EXECUTE ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ, TEXT, TEXT) TO PUBLIC;
GRANT EXECUTE ON FUNCTION update_integration_sync_status(UUID, TEXT, TEXT) TO PUBLIC;
GRANT EXECUTE ON FUNCTION record_integration_sync_failure(UUID, TEXT, INT) TO PUBLIC;
GRANT EXECUTE ON FUNCTION get_publisher_pause_integration(UUID, UUID) TO PUBLIC;
GRANT EXECUTE ON FUNCTION insert_publisher_action_audit(UUID, UUID, UUID, TEXT, TEXT, TEXT, TEXT, UUID, UUID, TEXT, TEXT, TEXT, TEXT, JSONB) TO PUBLIC;
