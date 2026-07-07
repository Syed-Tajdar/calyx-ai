DROP FUNCTION IF EXISTS update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ, TEXT, TEXT);
DROP FUNCTION IF EXISTS list_api_sync_integrations();

CREATE OR REPLACE FUNCTION update_integration_sync_state(
    p_id UUID,
    p_cursor TEXT,
    p_sync_at TIMESTAMPTZ
)
RETURNS VOID
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    UPDATE integrations
    SET sync_cursor = p_cursor,
        last_sync_at = p_sync_at,
        updated_at = NOW()
    WHERE id = p_id;
$$;

CREATE OR REPLACE FUNCTION list_api_sync_integrations()
RETURNS TABLE (
    id                 UUID,
    tenant_id          UUID,
    platform           TEXT,
    name               TEXT,
    api_credentials_blob BYTEA,
    sync_cursor        TEXT,
    last_sync_at       TIMESTAMPTZ
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT id, tenant_id, platform, name, api_credentials_blob, sync_cursor, last_sync_at
    FROM integrations
    WHERE connection_mode = 'api_sync'
      AND is_active = TRUE
      AND api_credentials_blob IS NOT NULL;
$$;

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_status_check;
ALTER TABLE integrations
    DROP COLUMN IF EXISTS last_sync_error,
    DROP COLUMN IF EXISTS sync_status;
