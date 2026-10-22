\set ON_ERROR_STOP on
-- Migration 000037 checks on a THROWAWAY database (run by verify-migration-37-function-grants.sh).
-- 1. No SECURITY DEFINER function in public is executable by PUBLIC.
-- 2. The exact role matrix: each function is executable by its one caller role and by no other app role.
-- 3. Real calls: the caller role succeeds, the other roles get "permission denied".
DO $$
DECLARE r RECORD; n INT;
BEGIN
  SELECT count(*) INTO n FROM pg_proc p
   WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace
     AND EXISTS (SELECT 1 FROM aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) a
                  WHERE a.grantee = 0 AND a.privilege_type = 'EXECUTE');
  ASSERT n = 0, n || ' SECURITY DEFINER function(s) still executable by PUBLIC';

  FOR r IN
    SELECT * FROM (VALUES
      ('authenticate_user(text,text)',                         'callloom_api'),
      ('list_workflow_tenants()',                              'callloom_api'),
      ('list_transcription_tenants()',                         'callloom_api'),
      ('lookup_integration_by_token(text)',                    'callloom_ingestion'),
      ('list_api_sync_integrations()',                         'callloom_sync'),
      ('get_api_sync_integration(uuid,uuid)',                  'callloom_sync'),
      ('get_api_sync_integration_any(uuid,uuid)',              'callloom_sync'),
      ('insert_writeback_audit(uuid,uuid,uuid,text,text,numeric,text,text,text,text,text)', 'callloom_sync'),
      ('insert_writeback_audit_v2(uuid,uuid,uuid,text,text,numeric,text,text,text,text,text,text,jsonb)', 'callloom_sync'),
      ('update_integration_sync_state(uuid,text,timestamptz,text,text)', 'callloom_sync'),
      ('update_integration_sync_status(uuid,text,text)',       'callloom_sync'),
      ('record_integration_sync_failure(uuid,text,int)',       'callloom_sync'),
      ('get_publisher_pause_integration(uuid,uuid)',           'callloom_sync'),
      ('insert_publisher_action_audit(uuid,uuid,uuid,text,text,text,text,uuid,uuid,text,text,text,text,jsonb)', 'callloom_sync'),
      ('claim_transcription_job()',                            'callloom_sync'),
      ('finish_transcription_job(uuid,text,text,text,timestamptz)', 'callloom_sync'),
      ('requeue_stale_transcription_jobs(int)',                'callloom_sync'),
      ('get_transcription_minutes_today(uuid)',                'callloom_sync'),
      ('add_transcription_usage(uuid,numeric)',                'callloom_sync'),
      ('get_transcription_archive_all(uuid)',                  'callloom_sync'),
      ('upsert_call_recording(uuid,text,text,uuid,text,bigint,numeric,text,text,timestamptz,text,text)', 'callloom_sync'),
      ('insert_call_transcript(uuid,text,text,uuid,text,text,text,text,numeric,text,int,jsonb)', 'callloom_sync'),
      ('get_call_recording_object(uuid,text,text)',            'callloom_sync'),
      ('list_recordings_to_expire(timestamptz,int)',           'callloom_sync'),
      ('mark_recording_expired(uuid)',                         'callloom_sync'),
      ('get_call_transcript_latest(uuid,text,text)',           'callloom_sync')
    ) AS t(fn, caller)
  LOOP
    FOR n IN 1..3 LOOP
      DECLARE role TEXT := (ARRAY['callloom_api','callloom_ingestion','callloom_sync'])[n];
              allowed BOOLEAN := has_function_privilege(role, r.fn, 'EXECUTE');
      BEGIN
        ASSERT allowed = (role = r.caller), format('%s: %s EXECUTE=%s, expected %s', r.fn, role, allowed, role = r.caller);
      END;
    END LOOP;
  END LOOP;
  -- Every SECURITY DEFINER function in public is in the matrix above (a new one must be added here).
  SELECT count(*) INTO n FROM pg_proc p WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace;
  ASSERT n = 26, n || ' SECURITY DEFINER functions in public, the matrix lists 26 — add the new one(s)';
  RAISE NOTICE 'PASS no SECURITY DEFINER function executable by PUBLIC; role matrix exact (all 26 functions x 3 roles)';
END $$;

-- Real calls. Seed as the owner.
INSERT INTO tenants (id, name) VALUES ('37373737-0000-4000-8000-000000000001', 'grants test');
INSERT INTO users (tenant_id, email, password_hash, role)
VALUES ('37373737-0000-4000-8000-000000000001', 'grants@example.test', crypt('pw-test', gen_salt('bf')), 'admin');
INSERT INTO integrations (id, tenant_id, platform, name, webhook_token, connection_mode, sync_mode, api_credentials_blob)
VALUES ('37373737-0000-4000-8000-0000000000c1', '37373737-0000-4000-8000-000000000001', 'retreaver', 'grants test',
        'grants-tok-1', 'api_sync', 'mock', '\x00'::bytea);

-- The credential-blob function: callloom_sync only.
SET ROLE callloom_sync;
DO $$ BEGIN
  ASSERT (SELECT count(*) FROM get_api_sync_integration('37373737-0000-4000-8000-000000000001', '37373737-0000-4000-8000-0000000000c1')) = 1,
         'sync reads its integration';
  ASSERT (SELECT count(*) FROM get_api_sync_integration_any('37373737-0000-4000-8000-000000000001', '37373737-0000-4000-8000-0000000000c1')) = 1,
         'sync reads _any';
  PERFORM update_integration_sync_status('37373737-0000-4000-8000-0000000000c1', 'ok', NULL);
  ASSERT (SELECT count(*) FROM list_api_sync_integrations() WHERE id = '37373737-0000-4000-8000-0000000000c1') = 1, 'sync lists it';
  RAISE NOTICE 'PASS callloom_sync calls its functions';
END $$;
RESET ROLE;

SET ROLE callloom_api;
DO $$ DECLARE n INT; BEGIN
  SELECT count(*) INTO n FROM authenticate_user('grants@example.test', 'pw-test');
  ASSERT n = 1, 'api authenticates (got ' || n || ')';
  PERFORM list_workflow_tenants();
  BEGIN
    PERFORM get_api_sync_integration('37373737-0000-4000-8000-000000000001', '37373737-0000-4000-8000-0000000000c1');
    RAISE EXCEPTION 'api read the credential blob';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  BEGIN
    PERFORM lookup_integration_by_token('grants-tok-1');
    RAISE EXCEPTION 'api called the ingestion lookup';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  RAISE NOTICE 'PASS callloom_api: login + workflow tenants ok; credential blob and token lookup refused';
END $$;
RESET ROLE;

SET ROLE callloom_ingestion;
DO $$ BEGIN
  PERFORM lookup_integration_by_token('grants-tok-1');
  BEGIN
    PERFORM get_api_sync_integration_any('37373737-0000-4000-8000-000000000001', '37373737-0000-4000-8000-0000000000c1');
    RAISE EXCEPTION 'ingestion read the credential blob';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  BEGIN
    PERFORM authenticate_user('grants@example.test', 'pw-test');
    RAISE EXCEPTION 'ingestion called authenticate_user';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  RAISE NOTICE 'PASS callloom_ingestion: token lookup ok; credential blob and login refused';
END $$;
RESET ROLE;
SELECT 'ALL MIGRATION 37 CHECKS PASSED' AS result;
