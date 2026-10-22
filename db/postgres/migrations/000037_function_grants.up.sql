-- v1 hardening: least privilege on the older SECURITY DEFINER functions.
--
-- Postgres grants EXECUTE on every new function to PUBLIC. No migration before 000036 revoked it, so
-- callloom_api and callloom_ingestion could call all 13 of these, including the three that return the
-- encrypted api_credentials_blob. Each function already has an explicit GRANT to the one role that calls
-- it; this revokes PUBLIC and re-asserts that grant. The owner (callloom, which runs `migrate` and the
-- psql verify scripts) keeps EXECUTE as owner. No data change, no function body change.
--
-- Callers, proven by code read (2026-10-22, branch v2/hardening-grants):
--
--   function                          role                 caller
--   authenticate_user                 callloom_api         services/api/src/auth/auth.service.ts:26
--   list_workflow_tenants             callloom_api         services/api/src/workflows/workflow-evaluator.service.ts:118
--   lookup_integration_by_token       callloom_ingestion   services/ingestion/internal/store/integrations.go:45
--   list_api_sync_integrations        callloom_sync        services/platform-sync/internal/store/integrations.go:22
--   get_api_sync_integration    (*)   callloom_sync        services/platform-sync/internal/store/integrations.go:39
--   get_api_sync_integration_any (*)  callloom_sync        services/platform-sync/internal/store/integrations.go:44
--   insert_writeback_audit            callloom_sync        services/platform-sync/internal/store/integrations.go:84
--   insert_writeback_audit_v2         callloom_sync        services/platform-sync/internal/store/integrations.go:95
--   update_integration_sync_state     callloom_sync        services/platform-sync/internal/store/integrations.go:205
--   update_integration_sync_status    callloom_sync        services/platform-sync/internal/store/integrations.go:214
--   record_integration_sync_failure   callloom_sync        services/platform-sync/internal/store/integrations.go:226
--   get_publisher_pause_integration (*) callloom_sync      services/platform-sync/internal/store/publisher_pause.go:26
--   insert_publisher_action_audit     callloom_sync        services/platform-sync/internal/store/publisher_pause.go:74
--
--   (*) returns the encrypted api_credentials_blob: callloom_sync only. The platform-sync probes
--   (cmd/probe-*, cmd/verify-retreaver-call) read platform-sync's own DATABASE_URL, so they are
--   callloom_sync too. The API and ingestion never call these three.

REVOKE ALL ON FUNCTION authenticate_user(TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION authenticate_user(TEXT, TEXT) TO callloom_api;

REVOKE ALL ON FUNCTION list_workflow_tenants() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION list_workflow_tenants() TO callloom_api;

REVOKE ALL ON FUNCTION lookup_integration_by_token(TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION lookup_integration_by_token(TEXT) TO callloom_ingestion;

REVOKE ALL ON FUNCTION list_api_sync_integrations() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION list_api_sync_integrations() TO callloom_sync;

REVOKE ALL ON FUNCTION get_api_sync_integration(UUID, UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION get_api_sync_integration(UUID, UUID) TO callloom_sync;

REVOKE ALL ON FUNCTION get_api_sync_integration_any(UUID, UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION get_api_sync_integration_any(UUID, UUID) TO callloom_sync;

REVOKE ALL ON FUNCTION insert_writeback_audit(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION insert_writeback_audit(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT) TO callloom_sync;

REVOKE ALL ON FUNCTION insert_writeback_audit_v2(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT, TEXT, JSONB) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION insert_writeback_audit_v2(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT, TEXT, JSONB) TO callloom_sync;

REVOKE ALL ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ, TEXT, TEXT) TO callloom_sync;

REVOKE ALL ON FUNCTION update_integration_sync_status(UUID, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION update_integration_sync_status(UUID, TEXT, TEXT) TO callloom_sync;

REVOKE ALL ON FUNCTION record_integration_sync_failure(UUID, TEXT, INT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION record_integration_sync_failure(UUID, TEXT, INT) TO callloom_sync;

REVOKE ALL ON FUNCTION get_publisher_pause_integration(UUID, UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION get_publisher_pause_integration(UUID, UUID) TO callloom_sync;

REVOKE ALL ON FUNCTION insert_publisher_action_audit(UUID, UUID, UUID, TEXT, TEXT, TEXT, TEXT, UUID, UUID, TEXT, TEXT, TEXT, TEXT, JSONB) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION insert_publisher_action_audit(UUID, UUID, UUID, TEXT, TEXT, TEXT, TEXT, UUID, UUID, TEXT, TEXT, TEXT, TEXT, JSONB) TO callloom_sync;
