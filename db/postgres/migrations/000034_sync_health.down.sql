DROP FUNCTION IF EXISTS record_integration_sync_failure(UUID, TEXT, INT);

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

UPDATE integrations SET sync_status = 'error' WHERE sync_status = 'failed';

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_status_check;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_sync_status_check
    CHECK (sync_status IN ('idle', 'syncing', 'ok', 'error', 'degraded'));

ALTER TABLE integrations
    DROP COLUMN IF EXISTS consecutive_failures,
    DROP COLUMN IF EXISTS last_success_at;
