-- R2 pre-flight P2: `degraded` must be scheduled exactly like Ringba's `error`.
--
-- The tick (RunAll), the hourly sweep and the deep sweep all select through
-- list_api_sync_integrations(), so this rule lives in SQL, not in Go. If any status were
-- filtered out, the first short window would stop that integration syncing forever.
--
-- Everything runs inside a transaction that is rolled back, so the database is unchanged.
--
--   docker exec -i deploy-postgres-1 psql -U callloom -d callloom -v ON_ERROR_STOP=1 \
--     -f - < scripts/verify-degraded-scheduling.sql

\set ON_ERROR_STOP on

BEGIN;

DO $$
DECLARE
    v_id      UUID;
    v_status  TEXT;
    v_found   BOOLEAN;
    v_err     TEXT;
BEGIN
    INSERT INTO integrations
        (tenant_id, platform, name, webhook_token, connection_mode,
         api_credentials_blob, sync_status, is_active)
    VALUES
        ('11111111-1111-1111-1111-111111111111'::uuid, 'retreaver',
         'p2-scheduling-test', 'p2-token-' || gen_random_uuid()::text, 'api_sync',
         '\x00'::bytea, 'degraded', TRUE)
    RETURNING id INTO v_id;

    -- 1. Every status the syncer can write must be scheduled identically.
    FOREACH v_status IN ARRAY ARRAY['idle', 'syncing', 'ok', 'error', 'degraded'] LOOP
        UPDATE integrations SET sync_status = v_status WHERE id = v_id;
        SELECT EXISTS (SELECT 1 FROM list_api_sync_integrations() WHERE id = v_id)
          INTO v_found;
        IF NOT v_found THEN
            RAISE EXCEPTION 'FAIL: sync_status=% excluded the integration from scheduling', v_status;
        END IF;
        RAISE NOTICE 'PASS sync_status=% is scheduled', v_status;
    END LOOP;

    -- 2. The single-integration fetch used by manual sync and RunWindow agrees.
    UPDATE integrations SET sync_status = 'degraded' WHERE id = v_id;
    SELECT EXISTS (
        SELECT 1 FROM get_api_sync_integration(
            '11111111-1111-1111-1111-111111111111'::uuid, v_id)
    ) INTO v_found;
    IF NOT v_found THEN
        RAISE EXCEPTION 'FAIL: get_api_sync_integration() excluded a degraded integration';
    END IF;
    RAISE NOTICE 'PASS get_api_sync_integration() returns a degraded integration';

    -- 3. Recovery: the status-only update a successful walk performs clears degraded.
    UPDATE integrations SET last_sync_error = 'window short' WHERE id = v_id;
    PERFORM update_integration_sync_status(v_id, 'ok', NULL);
    SELECT sync_status, last_sync_error INTO v_status, v_err
      FROM integrations WHERE id = v_id;
    IF v_status <> 'ok' THEN
        RAISE EXCEPTION 'FAIL: successful walk left sync_status=%', v_status;
    END IF;
    IF v_err IS NOT NULL THEN
        RAISE EXCEPTION 'FAIL: successful walk left last_sync_error=%', v_err;
    END IF;
    RAISE NOTICE 'PASS a successful walk clears degraded and its error, with no operator action';

    -- 4. Control. is_active is the one flag that does deschedule, so the checks above are
    --    not passing merely because the selector filters nothing.
    UPDATE integrations SET is_active = FALSE WHERE id = v_id;
    SELECT EXISTS (SELECT 1 FROM list_api_sync_integrations() WHERE id = v_id)
      INTO v_found;
    IF v_found THEN
        RAISE EXCEPTION 'FAIL: control check — an inactive integration was still selected';
    END IF;
    RAISE NOTICE 'PASS control — is_active=false does deschedule, so the selector is not inert';

    RAISE NOTICE 'ALL PASS: degraded is scheduled exactly like error';
END $$;

ROLLBACK;
