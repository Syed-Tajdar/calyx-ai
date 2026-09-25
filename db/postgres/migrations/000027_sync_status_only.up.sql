-- Status-only update: does not advance last_sync_at or sync_cursor (S1A F003b).

CREATE OR REPLACE FUNCTION update_integration_sync_status(
    p_id UUID,
    p_status TEXT,
    p_error TEXT
)
RETURNS VOID
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    UPDATE integrations
    SET sync_status = p_status,
        last_sync_error = p_error,
        updated_at = NOW()
    WHERE id = p_id;
$$;

GRANT EXECUTE ON FUNCTION update_integration_sync_status(UUID, TEXT, TEXT) TO callloom_sync;
