-- v1 sync health (Part 1, item 1). Every sync failure must be visible.
--
-- From 2026-10-18 to 2026-10-21 every sync failed at the redis sync lock, and the lock error
-- returned before anything was written, so Settings kept showing `ok`. From now on every
-- failure goes through record_integration_sync_failure(): one failure is `degraded`, N
-- consecutive failures are `failed`, and last_sync_error says why. last_success_at is the wall
-- clock of the last fully successful incremental sync. It moves only on success, separate from
-- last_sync_at (the end of the window that was fetched).

-- Additive only: two new columns (a constant default is metadata-only, no table rewrite), one
-- widened CHECK, one replaced and one new function. No existing row is updated. last_success_at
-- starts NULL and is set by the first successful sync after the deploy (one 120 s tick).
ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS last_success_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS consecutive_failures INT NOT NULL DEFAULT 0;

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_status_check;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_sync_status_check
    CHECK (sync_status IN ('idle', 'syncing', 'ok', 'error', 'degraded', 'failed'));

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
        last_success_at = CASE WHEN p_status = 'ok' THEN NOW() ELSE last_success_at END,
        consecutive_failures = CASE WHEN p_status = 'ok' THEN 0 ELSE consecutive_failures END,
        updated_at = NOW()
    WHERE id = p_id;
$$;

-- One more consecutive failure. Writes `degraded`, or `failed` once p_failed_after consecutive
-- failures are reached. Never touches last_sync_at, sync_cursor or last_success_at.
CREATE OR REPLACE FUNCTION record_integration_sync_failure(
    p_id UUID,
    p_error TEXT,
    p_failed_after INT
)
RETURNS TEXT
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    UPDATE integrations
    SET consecutive_failures = consecutive_failures + 1,
        sync_status = CASE
            WHEN consecutive_failures + 1 >= GREATEST(p_failed_after, 1) THEN 'failed'
            ELSE 'degraded'
        END,
        last_sync_error = left(p_error, 500),
        updated_at = NOW()
    WHERE id = p_id
    RETURNING sync_status;
$$;

GRANT EXECUTE ON FUNCTION update_integration_sync_state(UUID, TEXT, TIMESTAMPTZ, TEXT, TEXT) TO callloom_sync;
GRANT EXECUTE ON FUNCTION record_integration_sync_failure(UUID, TEXT, INT) TO callloom_sync;
