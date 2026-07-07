REVOKE EXECUTE ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ) FROM callloom_sync;
REVOKE EXECUTE ON FUNCTION list_api_sync_integrations() FROM callloom_sync;
DROP FUNCTION IF EXISTS update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS list_api_sync_integrations();
DROP ROLE IF EXISTS callloom_sync;

REVOKE INSERT, UPDATE ON integrations FROM callloom_api;

DROP INDEX IF EXISTS idx_integrations_api_sync;
ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_connection_mode_check;
ALTER TABLE integrations
    DROP COLUMN IF EXISTS sync_cursor,
    DROP COLUMN IF EXISTS last_sync_at,
    DROP COLUMN IF EXISTS api_credentials_blob,
    DROP COLUMN IF EXISTS connection_mode;
