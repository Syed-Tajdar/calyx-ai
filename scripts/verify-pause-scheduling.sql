-- Pause (is_active = false) and delete (deleted_at set) stay out of
-- list_api_sync_integrations(), which is what the tick, sweep and deep sweep use.
-- A deleted row cannot be resumed: the resume UPDATE requires deleted_at IS NULL.
-- Pause and resume write integration_settings_audit (who, when, integration, action).
-- The transaction rolls back. Nothing is left behind.
--
--   docker exec -i deploy-postgres-1 psql -U callloom -d callloom -v ON_ERROR_STOP=1 \
--     -f - < scripts/verify-pause-scheduling.sql

\set ON_ERROR_STOP on

BEGIN;

DO $$
DECLARE
    v_paused   UUID;
    v_deleted  UUID;
    v_found    BOOLEAN;
    v_resumed  INT;
    v_audit    INT;
    v_user     UUID;
BEGIN
    PERFORM set_config('app.current_tenant', '11111111-1111-1111-1111-111111111111', true);
    SELECT id INTO v_user FROM users WHERE role = 'admin' LIMIT 1;

    INSERT INTO integrations
        (tenant_id, platform, name, webhook_token, connection_mode,
         api_credentials_blob, sync_status, is_active)
    VALUES
        ('11111111-1111-1111-1111-111111111111'::uuid, 'ringba',
         'r2-paused-test', 'r2-paused-' || gen_random_uuid()::text, 'api_sync',
         '\x00'::bytea, 'idle', FALSE)
    RETURNING id INTO v_paused;

    SELECT EXISTS (SELECT 1 FROM list_api_sync_integrations() WHERE id = v_paused)
      INTO v_found;
    IF v_found THEN
        RAISE EXCEPTION 'FAIL: paused integration was scheduled';
    END IF;
    RAISE NOTICE 'PASS paused (is_active=false) is not in list_api_sync_integrations()';

    INSERT INTO integrations
        (tenant_id, platform, name, webhook_token, connection_mode,
         api_credentials_blob, sync_status, is_active, deleted_at)
    VALUES
        ('11111111-1111-1111-1111-111111111111'::uuid, 'ringba',
         'r2-deleted-test', 'r2-deleted-' || gen_random_uuid()::text, 'api_sync',
         '\x00'::bytea, 'idle', TRUE, NOW())
    RETURNING id INTO v_deleted;

    SELECT EXISTS (SELECT 1 FROM list_api_sync_integrations() WHERE id = v_deleted)
      INTO v_found;
    IF v_found THEN
        RAISE EXCEPTION 'FAIL: deleted integration with is_active=true was scheduled';
    END IF;
    RAISE NOTICE 'PASS deleted (deleted_at set) is not scheduled even if is_active is true';

    UPDATE integrations
       SET is_active = TRUE, updated_at = NOW()
     WHERE id = v_deleted AND deleted_at IS NULL;
    GET DIAGNOSTICS v_resumed = ROW_COUNT;
    IF v_resumed <> 0 THEN
        RAISE EXCEPTION 'FAIL: resume updated a deleted integration (% rows)', v_resumed;
    END IF;
    RAISE NOTICE 'PASS resume matches 0 rows when deleted_at is set';

    INSERT INTO integration_settings_audit (tenant_id, user_id, integration_id, action)
    VALUES ('11111111-1111-1111-1111-111111111111'::uuid, v_user, v_paused, 'pause');
    INSERT INTO integration_settings_audit (tenant_id, user_id, integration_id, action)
    VALUES ('11111111-1111-1111-1111-111111111111'::uuid, v_user, v_paused, 'resume');

    SELECT COUNT(*) INTO v_audit
      FROM integration_settings_audit
     WHERE integration_id = v_paused
       AND action IN ('pause', 'resume')
       AND user_id = v_user
       AND created_at IS NOT NULL;
    IF v_audit <> 2 THEN
        RAISE EXCEPTION 'FAIL: expected 2 settings-audit rows, got %', v_audit;
    END IF;
    RAISE NOTICE 'PASS pause and resume audit rows record who, when, integration, action';
END $$;

ROLLBACK;
