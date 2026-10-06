-- Revert F1's 'degraded' status. Existing degraded rows become 'error' so the narrowed CHECK
-- can be applied without failing on live data.

UPDATE integrations SET sync_status = 'error' WHERE sync_status = 'degraded';

ALTER TABLE integrations DROP CONSTRAINT IF EXISTS integrations_sync_status_check;
ALTER TABLE integrations
    ADD CONSTRAINT integrations_sync_status_check
    CHECK (sync_status IN ('idle', 'syncing', 'ok', 'error'));
