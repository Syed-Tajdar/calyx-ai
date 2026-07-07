-- API Sync connection mode (alongside existing webhook mode).

ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS connection_mode TEXT NOT NULL DEFAULT 'webhook',
    ADD COLUMN IF NOT EXISTS api_credentials_blob BYTEA,
    ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sync_cursor TEXT;

ALTER TABLE integrations
    ADD CONSTRAINT integrations_connection_mode_check
    CHECK (connection_mode IN ('webhook', 'api_sync'));

CREATE INDEX IF NOT EXISTS idx_integrations_api_sync
    ON integrations (connection_mode)
    WHERE connection_mode = 'api_sync' AND is_active = TRUE;

GRANT INSERT, UPDATE ON integrations TO callloom_api;

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'callloom_sync') THEN
        CREATE ROLE callloom_sync LOGIN PASSWORD 'sync_dev';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE callloom TO callloom_sync;
GRANT USAGE ON SCHEMA public TO callloom_sync;

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

GRANT EXECUTE ON FUNCTION list_api_sync_integrations() TO callloom_sync;
GRANT EXECUTE ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ) TO callloom_sync;
