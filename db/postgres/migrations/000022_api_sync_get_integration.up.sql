-- SECURITY DEFINER lookup for platform-sync manual sync/test (bypasses RLS).

CREATE OR REPLACE FUNCTION get_api_sync_integration(p_tenant_id UUID, p_id UUID)
RETURNS TABLE (
    id                   UUID,
    tenant_id            UUID,
    platform             TEXT,
    name                 TEXT,
    sync_mode            TEXT,
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
    SELECT id, tenant_id, platform, name, sync_mode, api_credentials_blob, sync_cursor,
           last_sync_at, sync_status, last_sync_error
    FROM integrations
    WHERE id = p_id
      AND tenant_id = p_tenant_id
      AND connection_mode = 'api_sync'
      AND is_active = TRUE
      AND api_credentials_blob IS NOT NULL;
$$;

GRANT EXECUTE ON FUNCTION get_api_sync_integration(UUID, UUID) TO callloom_sync;
