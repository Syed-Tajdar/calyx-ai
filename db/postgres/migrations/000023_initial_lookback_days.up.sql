-- Per-integration initial lookback window for empty-cursor / backfill syncs.

ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS initial_lookback_days INT NOT NULL DEFAULT 30;

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_initial_lookback_days_check;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_initial_lookback_days_check
    CHECK (initial_lookback_days BETWEEN 1 AND 365);

DROP FUNCTION IF EXISTS list_api_sync_integrations();

CREATE OR REPLACE FUNCTION list_api_sync_integrations()
RETURNS TABLE (
    id                     UUID,
    tenant_id              UUID,
    platform               TEXT,
    name                   TEXT,
    sync_mode              TEXT,
    api_credentials_blob   BYTEA,
    sync_cursor            TEXT,
    last_sync_at           TIMESTAMPTZ,
    sync_status            TEXT,
    last_sync_error        TEXT,
    initial_lookback_days  INT
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT id, tenant_id, platform, name, sync_mode, api_credentials_blob, sync_cursor,
           last_sync_at, sync_status, last_sync_error, initial_lookback_days
    FROM integrations
    WHERE connection_mode = 'api_sync'
      AND is_active = TRUE
      AND api_credentials_blob IS NOT NULL;
$$;

GRANT EXECUTE ON FUNCTION list_api_sync_integrations() TO callloom_sync;

DROP FUNCTION IF EXISTS get_api_sync_integration(UUID, UUID);

CREATE OR REPLACE FUNCTION get_api_sync_integration(p_tenant_id UUID, p_id UUID)
RETURNS TABLE (
    id                     UUID,
    tenant_id              UUID,
    platform               TEXT,
    name                   TEXT,
    sync_mode              TEXT,
    api_credentials_blob   BYTEA,
    sync_cursor            TEXT,
    last_sync_at           TIMESTAMPTZ,
    sync_status            TEXT,
    last_sync_error        TEXT,
    initial_lookback_days  INT
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT id, tenant_id, platform, name, sync_mode, api_credentials_blob, sync_cursor,
           last_sync_at, sync_status, last_sync_error, initial_lookback_days
    FROM integrations
    WHERE id = p_id
      AND tenant_id = p_tenant_id
      AND connection_mode = 'api_sync'
      AND is_active = TRUE
      AND api_credentials_blob IS NOT NULL;
$$;

GRANT EXECUTE ON FUNCTION get_api_sync_integration(UUID, UUID) TO callloom_sync;
