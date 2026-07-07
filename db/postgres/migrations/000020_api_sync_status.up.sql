-- Sync status and error message for API-key integrations.

DROP FUNCTION IF EXISTS update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS list_api_sync_integrations();

ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS sync_status TEXT NOT NULL DEFAULT 'idle',
    ADD COLUMN IF NOT EXISTS last_sync_error TEXT;

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_status_check;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_sync_status_check
    CHECK (sync_status IN ('idle', 'syncing', 'ok', 'error'));

CREATE OR REPLACE FUNCTION list_api_sync_integrations()
RETURNS TABLE (
    id                   UUID,
    tenant_id            UUID,
    platform             TEXT,
    name                 TEXT,
    api_credentials_blob BYTEA,
    sync_cursor          TEXT,
    last_sync_at         TIMESTAMPTZ,
    sync_status          TEXT,
    last_sync_error      TEXT
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT id, tenant_id, platform, name, api_credentials_blob, sync_cursor,
           last_sync_at, sync_status, last_sync_error
    FROM integrations
    WHERE connection_mode = 'api_sync'
      AND is_active = TRUE
      AND api_credentials_blob IS NOT NULL;
$$;

CREATE OR REPLACE FUNCTION update_integration_sync_state(
    p_id UUID,
    p_cursor TEXT,
    p_sync_at TIMESTAMPTZ,
    p_status TEXT,
    p_error TEXT
)
RETURNS VOID
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    UPDATE integrations
    SET sync_cursor = p_cursor,
        last_sync_at = p_sync_at,
        sync_status = p_status,
        last_sync_error = p_error,
        updated_at = NOW()
    WHERE id = p_id;
$$;

GRANT EXECUTE ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ, TEXT, TEXT) TO callloom_sync;
GRANT SELECT ON integrations TO callloom_sync;
