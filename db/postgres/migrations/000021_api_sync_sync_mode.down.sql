-- Down migration: remove per-integration sync mode column.

DROP FUNCTION IF EXISTS list_api_sync_integrations();

CREATE OR REPLACE FUNCTION list_api_sync_integrations()
RETURNS TABLE (
    id                 UUID,
    tenant_id          UUID,
    platform           TEXT,
    name               TEXT,
    api_credentials_blob BYTEA,
    sync_cursor        TEXT,
    last_sync_at       TIMESTAMPTZ,
    sync_status        TEXT,
    last_sync_error    TEXT
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

GRANT EXECUTE ON FUNCTION list_api_sync_integrations() TO callloom_sync;

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_mode_check;
ALTER TABLE integrations
    DROP COLUMN IF EXISTS sync_mode;

