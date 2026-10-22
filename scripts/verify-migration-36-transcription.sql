\set ON_ERROR_STOP on
-- Migration 000036 checks on a THROWAWAY database (fake ids, no real data).
INSERT INTO tenants (id, name) VALUES
  ('aaaaaaaa-0000-0000-0000-000000000001', 'Tenant A'),
  ('bbbbbbbb-0000-0000-0000-000000000002', 'Tenant B');
INSERT INTO users (id, tenant_id, email, password_hash, role) VALUES
  ('aaaaaaaa-0000-0000-0000-0000000000a1', 'aaaaaaaa-0000-0000-0000-000000000001', 'a@example.test', 'x', 'admin');
INSERT INTO integrations (id, tenant_id, platform, name, webhook_token, connection_mode, sync_mode)
VALUES ('aaaaaaaa-0000-0000-0000-0000000000c1', 'aaaaaaaa-0000-0000-0000-000000000001', 'ringba', 'Ringba A', 'tok-a', 'api_sync', 'real'),
       ('bbbbbbbb-0000-0000-0000-0000000000c2', 'bbbbbbbb-0000-0000-0000-000000000002', 'ringba', 'Ringba B', 'tok-b', 'api_sync', 'mock');
INSERT INTO transcription_jobs (tenant_id, integration_id, platform, external_call_id, reason, duration_seconds)
VALUES ('bbbbbbbb-0000-0000-0000-000000000002', 'bbbbbbbb-0000-0000-0000-0000000000c2', 'ringba', 'B-CALL', 'converted', 60);

-- ---------------- API role, under RLS ----------------
SET ROLE callloom_api;
SELECT set_config('app.current_tenant', 'aaaaaaaa-0000-0000-0000-000000000001', false);
DO $$
DECLARE n INT;
BEGIN
  SELECT count(*) INTO n FROM list_transcription_tenants();
  ASSERT n = 1, 'list_transcription_tenants sees only the real api_sync tenant (got ' || n || ')';
  INSERT INTO transcription_jobs (tenant_id, integration_id, platform, external_call_id, reason, duration_seconds, campaign_name)
  VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000c1', 'ringba', 'A-CALL-1', 'converted', 180, 'Final Expense');
  INSERT INTO transcription_jobs (tenant_id, platform, external_call_id, reason)
  VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'ringba', 'A-CALL-1', 'on_demand') ON CONFLICT DO NOTHING;
  SELECT count(*) INTO n FROM transcription_jobs;
  ASSERT n = 1, 'one job per call, and tenant B''s job is invisible (got ' || n || ')';
  BEGIN
    INSERT INTO transcription_jobs (tenant_id, platform, external_call_id, reason)
    VALUES ('bbbbbbbb-0000-0000-0000-000000000002', 'ringba', 'X', 'on_demand');
    RAISE EXCEPTION 'RLS let tenant A insert a job for tenant B';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  BEGIN
    INSERT INTO call_recordings (tenant_id, platform, external_call_id, sha256, bytes, content_type, downloaded_at, archive_state)
    VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'ringba', 'A', repeat('a', 64), 1, 'audio/mpeg', NOW(), 'archived');
    RAISE EXCEPTION 'API may not write call_recordings';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  UPDATE integrations SET recording_access_blob = '\x01'::bytea, recording_access_set_at = NOW()
   WHERE id = 'aaaaaaaa-0000-0000-0000-0000000000c1';
  INSERT INTO transcription_settings (tenant_id, archive_all) VALUES ('aaaaaaaa-0000-0000-0000-000000000001', true)
  ON CONFLICT (tenant_id) DO UPDATE SET archive_all = EXCLUDED.archive_all;
  INSERT INTO recording_access_audit (tenant_id, user_id, platform, external_call_id, action, outcome)
  VALUES ('aaaaaaaa-0000-0000-0000-000000000001', 'aaaaaaaa-0000-0000-0000-0000000000a1', 'ringba', 'A-CALL-1', 'play', 'refused');
  BEGIN
    UPDATE recording_access_audit SET outcome = 'served';
    RAISE EXCEPTION 'audit row was updated';
  EXCEPTION WHEN insufficient_privilege OR raise_exception THEN
    IF SQLERRM = 'audit row was updated' THEN RAISE; END IF;
  END;
  BEGIN
    PERFORM claim_transcription_job();
    RAISE EXCEPTION 'API may not claim jobs';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;
  RAISE NOTICE 'PASS api role: RLS, unique job per call, grants, audit append-only';
END $$;
RESET ROLE;

-- immutability also holds for the owner
DO $$ BEGIN
  DELETE FROM recording_access_audit;
  RAISE EXCEPTION 'owner deleted an audit row';
EXCEPTION WHEN raise_exception THEN
  IF SQLERRM <> 'recording_access_audit is immutable' THEN RAISE; END IF;
  RAISE NOTICE 'PASS audit immutable for the owner too';
END $$;

-- ---------------- platform-sync role, functions only ----------------
SET ROLE callloom_sync;
DO $$
DECLARE j RECORD; j2 RECORD; tmp RECORD; rid UUID; tid UUID; m NUMERIC; doc RECORD; n INT; att INT;
BEGIN
  BEGIN
    PERFORM 1 FROM transcription_jobs;
    RAISE EXCEPTION 'sync role read the table directly';
  EXCEPTION WHEN insufficient_privilege THEN NULL; END;

  SELECT * INTO j FROM claim_transcription_job();
  ASSERT j.id IS NOT NULL, 'claimed a job';
  ASSERT j.attempts = 1, 'attempts incremented';
  -- second claim gets the other tenant's job, never the running one
  SELECT * INTO j2 FROM claim_transcription_job();
  ASSERT j2.id IS NOT NULL AND j2.id <> j.id, 'second claim is a different job';
  ASSERT (SELECT count(*) FROM claim_transcription_job()) = 0, 'nothing left to claim';

  -- the tenant A job: check the join data
  -- oldest first: tenant B's job was queued first
  ASSERT j.external_call_id = 'B-CALL' AND j2.external_call_id = 'A-CALL-1', 'claims oldest first';
  tmp := j; j := j2; j2 := tmp;
  ASSERT j.sync_mode = 'real' AND j.recording_access_blob = '\x01'::bytea AND j.campaign_name = 'Final Expense',
         'claim returns sync_mode, both blobs, campaign';

  -- capped: back in the queue, the attempt is not counted
  PERFORM finish_transcription_job(j.id, 'capped', NULL, NULL, NOW() - interval '1 second');
  SELECT * INTO j FROM claim_transcription_job();
  ASSERT j.external_call_id = 'A-CALL-1' AND j.attempts = 1, 'capped job reclaimed with attempts still 1 (got ' || j.attempts || ')';

  ASSERT get_transcription_archive_all(j.tenant_id) = true, 'archive_all read';
  ASSERT get_transcription_archive_all('bbbbbbbb-0000-0000-0000-000000000002') = false, 'archive_all default false';

  rid := upsert_call_recording(j.tenant_id, 'ringba', 'A-CALL-1', j.id, repeat('ab', 32), 12345, 179.5, 'audio/mpeg',
                               'recordings/a/ringba/2026/10/' || repeat('ab', 32) || '.mp3', NOW() - interval '25 months', 'archived', NULL);
  ASSERT upsert_call_recording(j.tenant_id, 'ringba', 'A-CALL-1', j.id, repeat('ab', 32), 12345, 179.5, 'audio/mpeg',
                               'recordings/a/ringba/2026/10/' || repeat('ab', 32) || '.mp3', NOW() - interval '25 months', 'archived', NULL) = rid,
         'upsert is idempotent per call';

  tid := insert_call_transcript(j.tenant_id, 'ringba', 'A-CALL-1', rid, repeat('ab', 32), 'en', 'nova-3', 'req-1', 179.5,
           'deepgram:pci,ssn+calyx', 2,
           '[{"idx":0,"speaker":0,"start_ms":400,"end_ms":3100,"text":"Hello"},{"idx":1,"speaker":1,"start_ms":3600,"end_ms":8200,"text":"card [REDACTED]"}]'::jsonb);
  PERFORM insert_call_transcript(j.tenant_id, 'ringba', 'A-CALL-1', rid, repeat('ab', 32), 'en', 'nova-3', 'req-2', 179.5,
           'deepgram:pci,ssn+calyx', 0, '[{"idx":0,"speaker":0,"start_ms":0,"end_ms":10,"text":"v2"}]'::jsonb);
  SELECT * INTO doc FROM get_call_transcript_latest(j.tenant_id, 'ringba', 'A-CALL-1');
  ASSERT doc.transcript_version = 2, 'latest is version 2';
  ASSERT jsonb_array_length(doc.utterances) = 1 AND doc.utterances->0->>'text' = 'v2' AND doc.utterances->0->>'role' = 'unknown',
         'utterances json has keys (got ' || doc.utterances::text || ')';
  ASSERT (SELECT sha256 FROM get_call_recording_object(j.tenant_id, 'ringba', 'A-CALL-1')) = repeat('ab', 32), 'recording object';
  ASSERT (SELECT count(*) FROM get_call_recording_object('bbbbbbbb-0000-0000-0000-000000000002', 'ringba', 'A-CALL-1')) = 0,
         'recording object is per tenant';

  m := add_transcription_usage(j.tenant_id, 3.0);
  m := add_transcription_usage(j.tenant_id, 2.5);
  ASSERT m = 5.5 AND get_transcription_minutes_today(j.tenant_id) = 5.5, 'usage adds up (got ' || m || ')';
  ASSERT get_transcription_minutes_today('bbbbbbbb-0000-0000-0000-000000000002') = 0, 'other tenant 0';

  PERFORM finish_transcription_job(j.id, 'done', NULL, repeat('e', 400), NULL);

  -- the other job: no recording
  PERFORM finish_transcription_job(j2.id, 'no_recording', 'not_answered', NULL, NULL);

  -- retention: 24 months
  SELECT count(*) INTO n FROM list_recordings_to_expire(NOW() - interval '24 months', 100);
  ASSERT n = 1, 'one recording older than 24 months (got ' || n || ')';
  PERFORM mark_recording_expired(rid);
  SELECT count(*) INTO n FROM list_recordings_to_expire(NOW() - interval '24 months', 100);
  ASSERT n = 0, 'expired recordings are not listed again';

  ASSERT requeue_stale_transcription_jobs(30) = 0, 'no stale running jobs';
  RAISE NOTICE 'PASS sync role: claim/skip-locked order, capped, archive_all, upsert, transcript versions + utterances json, usage, finish, expiry, requeue';
END $$;
RESET ROLE;

DO $$
DECLARE r RECORD;
BEGIN
  SELECT state, attempts, length(last_error) AS el INTO r FROM transcription_jobs WHERE external_call_id = 'A-CALL-1';
  ASSERT r.state = 'done' AND r.el = 300, 'job done, error truncated to 300 (' || r.state || ',' || r.el || ')';
  SELECT state, no_recording_reason AS nr INTO r FROM transcription_jobs WHERE external_call_id = 'B-CALL';
  ASSERT r.state = 'no_recording' AND r.nr = 'not_answered', 'no_recording reason stored';
  ASSERT (SELECT archive_state FROM call_recordings WHERE external_call_id = 'A-CALL-1') = 'expired', 'row expired';
  ASSERT (SELECT count(*) FROM call_transcript_utterances) = 3, 'utterances stored for both versions';
  -- stale requeue
  UPDATE transcription_jobs SET state = 'running', updated_at = NOW() - interval '1 hour' WHERE external_call_id = 'B-CALL';
  ASSERT requeue_stale_transcription_jobs(30) = 1, 'stale running job requeued';
  RAISE NOTICE 'PASS final state checks';
END $$;
SELECT 'ALL MIGRATION 36 CHECKS PASSED' AS result;
