-- Stage 4 R1-fix F1: add the 'degraded' sync status.
--
-- A windowed sync that keeps returning fewer rows than the platform's own `Total` must not
-- advance its cursor, and must not look healthy while it is stuck. 'error' is the wrong word for
-- it — the request succeeded, the data did not add up — and a stuck sync that reads 'ok' is the
-- silent gap this stage exists to prevent. 'degraded' means: the cursor is held, the last window
-- is incomplete, and the next successful walk clears it automatically.

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_status_check;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_sync_status_check
    CHECK (sync_status IN ('idle', 'syncing', 'ok', 'error', 'degraded'));
