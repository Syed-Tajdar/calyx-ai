-- v2 Phase 2B — Transcription (Deepgram) and the recording archive.
-- Design: docs/v2-intelligence-report.md §4.1-4.8 and §2B. Every table is tenant-scoped with RLS on
-- app.current_tenant. No recording URL is stored anywhere (they are short-lived or public links); the
-- audio lives only in the archive bucket, addressed by its SHA-256.
--
-- Additive only: new tables and functions, two new nullable columns on integrations. Nothing existing
-- is changed or rewritten.

-- Ringba Recording Access credentials per integration: encrypted JSON {"username","password"} with the
-- same CREDENTIAL_ENCRYPTION_KEY as api_credentials_blob. Write-only from the UI (never shown again).
ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS recording_access_blob   BYTEA,
    ADD COLUMN IF NOT EXISTS recording_access_set_at TIMESTAMPTZ;

-- The queue. One row per call: (tenant, platform, external_call_id) is unique, so a call is never
-- transcribed twice whatever enqueues it (cron, a later conversion, the button, a backfill).
CREATE TABLE IF NOT EXISTS transcription_jobs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    integration_id      UUID REFERENCES integrations(id),
    platform            TEXT NOT NULL CHECK (platform IN ('ringba', 'retreaver')),
    external_call_id    TEXT NOT NULL,
    reason              TEXT NOT NULL CHECK (reason IN ('converted', 'should_convert', 'on_demand', 'backfill', 'archive_only')),
    state               TEXT NOT NULL DEFAULT 'queued'
                        CHECK (state IN ('queued', 'running', 'done', 'no_recording', 'failed', 'capped')),
    no_recording_reason TEXT CHECK (no_recording_reason IN ('not_answered', 'recording_disabled', 'blocked_or_cut', 'fetch_failed', 'no_access')),
    occurred_at         TIMESTAMPTZ,
    duration_seconds    INT,                 -- from the call data; the cap estimate before the audio is known
    campaign_name       TEXT,                -- picks the language rule (Spanish campaigns)
    attempts            INT NOT NULL DEFAULT 0,
    next_try_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error          TEXT,                -- short, redacted; never a URL or a key
    requested_by        UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, platform, external_call_id)
);
CREATE INDEX IF NOT EXISTS idx_transcription_jobs_due ON transcription_jobs (state, next_try_at);

-- One archived recording per call. Proof of integrity: sha256, bytes, duration, platform, call id,
-- download time; the transcript links to the hash.
CREATE TABLE IF NOT EXISTS call_recordings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    platform          TEXT NOT NULL,
    external_call_id  TEXT NOT NULL,
    job_id            UUID REFERENCES transcription_jobs(id),
    sha256            TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    bytes             BIGINT NOT NULL CHECK (bytes > 0),
    duration_seconds  NUMERIC(10, 2),
    content_type      TEXT NOT NULL,
    object_key        TEXT,                  -- recordings/{tenant}/{platform}/{yyyy}/{mm}/{sha256}.mp3
    downloaded_at     TIMESTAMPTZ NOT NULL,  -- UTC
    archive_state     TEXT NOT NULL CHECK (archive_state IN ('archived', 'not_archived', 'expired')),
    archive_error     TEXT,
    archived_at       TIMESTAMPTZ,
    expired_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, platform, external_call_id)
);

CREATE TABLE IF NOT EXISTS call_transcripts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    platform            TEXT NOT NULL,
    external_call_id    TEXT NOT NULL,
    recording_id        UUID REFERENCES call_recordings(id),
    recording_sha256    TEXT NOT NULL,
    transcript_version  INT NOT NULL DEFAULT 1,
    language            TEXT,
    model               TEXT NOT NULL,
    deepgram_request_id TEXT,
    duration_seconds    NUMERIC(10, 2),
    redaction           TEXT NOT NULL,       -- e.g. 'deepgram:pci,ssn+calyx' or 'deepgram:numbers+calyx'
    scrubbed_count      INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, platform, external_call_id, transcript_version)
);

CREATE TABLE IF NOT EXISTS call_transcript_utterances (
    transcript_id UUID NOT NULL REFERENCES call_transcripts(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    idx           INT NOT NULL,
    speaker       INT,
    role          TEXT NOT NULL DEFAULT 'unknown' CHECK (role IN ('caller', 'agent', 'unknown')),
    start_ms      INT NOT NULL,
    end_ms        INT NOT NULL,
    text          TEXT NOT NULL,             -- already redacted (Deepgram + Calyx scrub)
    PRIMARY KEY (transcript_id, idx)
);

-- Who played or downloaded which recording. Append-only.
CREATE TABLE IF NOT EXISTS recording_access_audit (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id),
    user_id          UUID REFERENCES users(id),
    recording_id     UUID REFERENCES call_recordings(id),
    platform         TEXT NOT NULL,
    external_call_id TEXT NOT NULL,
    action           TEXT NOT NULL CHECK (action IN ('play', 'download', 'evidence_pack')),
    outcome          TEXT NOT NULL CHECK (outcome IN ('served', 'refused', 'failed')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE OR REPLACE FUNCTION recording_access_audit_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'recording_access_audit is immutable';
END;
$$;
DROP TRIGGER IF EXISTS recording_access_audit_no_update ON recording_access_audit;
CREATE TRIGGER recording_access_audit_no_update
    BEFORE UPDATE OR DELETE ON recording_access_audit
    FOR EACH ROW EXECUTE FUNCTION recording_access_audit_immutable();

-- Minutes per Eastern day, for the hard cap (TRANSCRIBE_DAILY_MINUTES_CAP) and the Providers tab.
CREATE TABLE IF NOT EXISTS transcription_usage_daily (
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    usage_date  DATE NOT NULL,
    minutes     NUMERIC(12, 2) NOT NULL DEFAULT 0,
    calls       INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, usage_date)
);

-- Admin switches per tenant. archive_all: archive every recorded call, not only transcribed ones.
CREATE TABLE IF NOT EXISTS transcription_settings (
    tenant_id   UUID PRIMARY KEY REFERENCES tenants(id),
    archive_all BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by  UUID REFERENCES users(id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
DECLARE t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['transcription_jobs','call_recordings','call_transcripts','call_transcript_utterances',
                             'recording_access_audit','transcription_usage_daily','transcription_settings'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', t || '_isolation', t);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (tenant_id = current_setting(''app.current_tenant'', true)::uuid) '
            'WITH CHECK (tenant_id = current_setting(''app.current_tenant'', true)::uuid)',
            t || '_isolation', t);
    END LOOP;
END $$;

-- API (callloom_api, under RLS): enqueue, read, audit, settings. It never writes transcripts or recordings.
GRANT SELECT, INSERT, UPDATE ON transcription_jobs TO callloom_api;
GRANT SELECT ON call_recordings, call_transcripts, call_transcript_utterances, transcription_usage_daily TO callloom_api;
GRANT SELECT, INSERT ON recording_access_audit TO callloom_api;
GRANT SELECT, INSERT, UPDATE ON transcription_settings TO callloom_api;

-- Tenants the API's enqueue job visits: those with a Live (sync_mode real) Ringba or Retreaver API integration.
-- Test-mode (mock) integrations are never enqueued automatically; the button still works for them on a laptop.
CREATE OR REPLACE FUNCTION list_transcription_tenants()
RETURNS TABLE (tenant_id UUID)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT DISTINCT i.tenant_id FROM integrations i
     WHERE i.deleted_at IS NULL AND i.is_active AND i.connection_mode = 'api_sync' AND i.sync_mode = 'real'
       AND i.platform IN ('ringba', 'retreaver');
$$;

-- ---- platform-sync worker (callloom_sync): SECURITY DEFINER functions only, like the sync ----------

-- Claim the next due job (oldest first), mark it running, return it with what the worker needs.
CREATE OR REPLACE FUNCTION claim_transcription_job()
RETURNS TABLE (
    id                    UUID,
    tenant_id             UUID,
    integration_id        UUID,
    platform              TEXT,
    external_call_id      TEXT,
    reason                TEXT,
    occurred_at           TIMESTAMPTZ,
    duration_seconds      INT,
    campaign_name         TEXT,
    attempts              INT,
    sync_mode             TEXT,
    api_credentials_blob  BYTEA,
    recording_access_blob BYTEA
)
SECURITY DEFINER
SET search_path = public
LANGUAGE plpgsql
AS $$
DECLARE j_id UUID;
BEGIN
    SELECT j.id INTO j_id
      FROM transcription_jobs j
     WHERE j.state IN ('queued', 'capped') AND j.next_try_at <= NOW()
     ORDER BY j.next_try_at, j.created_at
     LIMIT 1
     FOR UPDATE SKIP LOCKED;
    IF j_id IS NULL THEN
        RETURN;
    END IF;
    UPDATE transcription_jobs j SET state = 'running', attempts = j.attempts + 1, updated_at = NOW() WHERE j.id = j_id;
    RETURN QUERY
        SELECT j.id, j.tenant_id, j.integration_id, j.platform, j.external_call_id, j.reason, j.occurred_at,
               j.duration_seconds, j.campaign_name, j.attempts, i.sync_mode, i.api_credentials_blob, i.recording_access_blob
          FROM transcription_jobs j
          LEFT JOIN integrations i ON i.id = j.integration_id AND i.deleted_at IS NULL
         WHERE j.id = j_id;
END;
$$;

-- Finish a job: done | no_recording | failed (final) or queued | capped (retry at p_next_try_at).
CREATE OR REPLACE FUNCTION finish_transcription_job(
    p_id UUID, p_state TEXT, p_no_recording_reason TEXT, p_error TEXT, p_next_try_at TIMESTAMPTZ
)
RETURNS VOID
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    UPDATE transcription_jobs
       SET state = p_state,
           no_recording_reason = p_no_recording_reason,
           last_error = left(p_error, 300),
           next_try_at = COALESCE(p_next_try_at, next_try_at),
           attempts = CASE WHEN p_state = 'capped' THEN GREATEST(attempts - 1, 0) ELSE attempts END,
           updated_at = NOW()
     WHERE id = p_id;
$$;

-- Jobs left 'running' by a crash or a restart go back to the queue after p_stale_minutes.
CREATE OR REPLACE FUNCTION requeue_stale_transcription_jobs(p_stale_minutes INT)
RETURNS INT
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    WITH r AS (
        UPDATE transcription_jobs SET state = 'queued', updated_at = NOW()
         WHERE state = 'running' AND updated_at < NOW() - make_interval(mins => p_stale_minutes)
        RETURNING 1)
    SELECT count(*)::int FROM r;
$$;

CREATE OR REPLACE FUNCTION get_transcription_minutes_today(p_tenant_id UUID)
RETURNS NUMERIC
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT COALESCE((SELECT minutes FROM transcription_usage_daily
                      WHERE tenant_id = p_tenant_id
                        AND usage_date = (NOW() AT TIME ZONE 'America/New_York')::date), 0);
$$;

CREATE OR REPLACE FUNCTION add_transcription_usage(p_tenant_id UUID, p_minutes NUMERIC)
RETURNS NUMERIC
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    INSERT INTO transcription_usage_daily (tenant_id, usage_date, minutes, calls)
    VALUES (p_tenant_id, (NOW() AT TIME ZONE 'America/New_York')::date, p_minutes, 1)
    ON CONFLICT (tenant_id, usage_date)
    DO UPDATE SET minutes = transcription_usage_daily.minutes + EXCLUDED.minutes,
                  calls = transcription_usage_daily.calls + 1, updated_at = NOW()
    RETURNING minutes;
$$;

CREATE OR REPLACE FUNCTION get_transcription_archive_all(p_tenant_id UUID)
RETURNS BOOLEAN
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT COALESCE((SELECT archive_all FROM transcription_settings WHERE tenant_id = p_tenant_id), FALSE);
$$;

-- One recording row per call; a second fetch of the same call (a retry) updates it in place.
CREATE OR REPLACE FUNCTION upsert_call_recording(
    p_tenant_id UUID, p_platform TEXT, p_external_call_id TEXT, p_job_id UUID, p_sha256 TEXT, p_bytes BIGINT,
    p_duration NUMERIC, p_content_type TEXT, p_object_key TEXT, p_downloaded_at TIMESTAMPTZ,
    p_archive_state TEXT, p_archive_error TEXT
)
RETURNS UUID
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    INSERT INTO call_recordings (tenant_id, platform, external_call_id, job_id, sha256, bytes, duration_seconds,
                                 content_type, object_key, downloaded_at, archive_state, archive_error, archived_at)
    VALUES (p_tenant_id, p_platform, p_external_call_id, p_job_id, p_sha256, p_bytes, p_duration, p_content_type,
            p_object_key, p_downloaded_at, p_archive_state, left(p_archive_error, 300),
            CASE WHEN p_archive_state = 'archived' THEN NOW() END)
    ON CONFLICT (tenant_id, platform, external_call_id) DO UPDATE
       SET job_id = EXCLUDED.job_id, sha256 = EXCLUDED.sha256, bytes = EXCLUDED.bytes,
           duration_seconds = EXCLUDED.duration_seconds, content_type = EXCLUDED.content_type,
           object_key = EXCLUDED.object_key, downloaded_at = EXCLUDED.downloaded_at,
           archive_state = EXCLUDED.archive_state, archive_error = EXCLUDED.archive_error,
           archived_at = EXCLUDED.archived_at
    RETURNING id;
$$;

-- A transcript and its utterances in one call. p_utterances: [{"idx","speaker","start_ms","end_ms","text"}].
CREATE OR REPLACE FUNCTION insert_call_transcript(
    p_tenant_id UUID, p_platform TEXT, p_external_call_id TEXT, p_recording_id UUID, p_recording_sha256 TEXT,
    p_language TEXT, p_model TEXT, p_request_id TEXT, p_duration NUMERIC, p_redaction TEXT, p_scrubbed INT,
    p_utterances JSONB
)
RETURNS UUID
SECURITY DEFINER
SET search_path = public
LANGUAGE plpgsql
AS $$
DECLARE t_id UUID; v INT;
BEGIN
    SELECT COALESCE(MAX(transcript_version), 0) + 1 INTO v FROM call_transcripts
     WHERE tenant_id = p_tenant_id AND platform = p_platform AND external_call_id = p_external_call_id;
    INSERT INTO call_transcripts (tenant_id, platform, external_call_id, recording_id, recording_sha256,
                                  transcript_version, language, model, deepgram_request_id, duration_seconds,
                                  redaction, scrubbed_count)
    VALUES (p_tenant_id, p_platform, p_external_call_id, p_recording_id, p_recording_sha256, v, p_language, p_model,
            p_request_id, p_duration, p_redaction, p_scrubbed)
    RETURNING id INTO t_id;
    INSERT INTO call_transcript_utterances (transcript_id, tenant_id, idx, speaker, start_ms, end_ms, text)
    SELECT t_id, p_tenant_id, (u->>'idx')::int, (u->>'speaker')::int, (u->>'start_ms')::int, (u->>'end_ms')::int,
           u->>'text'
      FROM jsonb_array_elements(p_utterances) u;
    RETURN t_id;
END;
$$;

-- Playback / evidence (internal route; the API has already checked admin and written the audit row).
CREATE OR REPLACE FUNCTION get_call_recording_object(p_tenant_id UUID, p_platform TEXT, p_external_call_id TEXT)
RETURNS TABLE (id UUID, sha256 TEXT, bytes BIGINT, content_type TEXT, object_key TEXT, archive_state TEXT,
               downloaded_at TIMESTAMPTZ, duration_seconds NUMERIC)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT r.id, r.sha256, r.bytes, r.content_type, r.object_key, r.archive_state, r.downloaded_at, r.duration_seconds
      FROM call_recordings r
     WHERE r.tenant_id = p_tenant_id AND r.platform = p_platform AND r.external_call_id = p_external_call_id;
$$;

-- Retention: archived recordings downloaded before p_before; the cleanup job deletes the object, then
-- marks the row expired (the transcript and later QA stay).
CREATE OR REPLACE FUNCTION list_recordings_to_expire(p_before TIMESTAMPTZ, p_limit INT)
RETURNS TABLE (id UUID, object_key TEXT)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT r.id, r.object_key FROM call_recordings r
     WHERE r.archive_state = 'archived' AND r.downloaded_at < p_before
     ORDER BY r.downloaded_at LIMIT p_limit;
$$;

CREATE OR REPLACE FUNCTION mark_recording_expired(p_id UUID)
RETURNS VOID
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    UPDATE call_recordings SET archive_state = 'expired', expired_at = NOW() WHERE id = p_id;
$$;

-- Latest transcript with its utterances, for the evidence pack (internal route).
CREATE OR REPLACE FUNCTION get_call_transcript_latest(p_tenant_id UUID, p_platform TEXT, p_external_call_id TEXT)
RETURNS TABLE (id UUID, transcript_version INT, language TEXT, model TEXT, deepgram_request_id TEXT,
               duration_seconds NUMERIC, redaction TEXT, recording_sha256 TEXT, created_at TIMESTAMPTZ, utterances JSONB)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT t.id, t.transcript_version, t.language, t.model, t.deepgram_request_id, t.duration_seconds, t.redaction,
           t.recording_sha256, t.created_at,
           COALESCE((SELECT jsonb_agg(jsonb_build_object('idx', u.idx, 'speaker', u.speaker, 'role', u.role,
                                                         'start_ms', u.start_ms, 'end_ms', u.end_ms, 'text', u.text)
                                      ORDER BY u.idx)
                       FROM call_transcript_utterances u WHERE u.transcript_id = t.id), '[]'::jsonb)
      FROM call_transcripts t
     WHERE t.tenant_id = p_tenant_id AND t.platform = p_platform AND t.external_call_id = p_external_call_id
     ORDER BY t.transcript_version DESC
     LIMIT 1;
$$;

-- Functions are EXECUTE-able by PUBLIC by default; these SECURITY DEFINER ones are for one role each
-- (the worker: callloom_sync; the enqueue job: callloom_api), so revoke PUBLIC first.
REVOKE ALL ON FUNCTION list_transcription_tenants() FROM PUBLIC;
REVOKE ALL ON FUNCTION claim_transcription_job() FROM PUBLIC;
REVOKE ALL ON FUNCTION finish_transcription_job(UUID, TEXT, TEXT, TEXT, TIMESTAMPTZ) FROM PUBLIC;
REVOKE ALL ON FUNCTION requeue_stale_transcription_jobs(INT) FROM PUBLIC;
REVOKE ALL ON FUNCTION get_transcription_minutes_today(UUID) FROM PUBLIC;
REVOKE ALL ON FUNCTION add_transcription_usage(UUID, NUMERIC) FROM PUBLIC;
REVOKE ALL ON FUNCTION get_transcription_archive_all(UUID) FROM PUBLIC;
REVOKE ALL ON FUNCTION upsert_call_recording(UUID, TEXT, TEXT, UUID, TEXT, BIGINT, NUMERIC, TEXT, TEXT, TIMESTAMPTZ, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION insert_call_transcript(UUID, TEXT, TEXT, UUID, TEXT, TEXT, TEXT, TEXT, NUMERIC, TEXT, INT, JSONB) FROM PUBLIC;
REVOKE ALL ON FUNCTION get_call_recording_object(UUID, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION list_recordings_to_expire(TIMESTAMPTZ, INT) FROM PUBLIC;
REVOKE ALL ON FUNCTION mark_recording_expired(UUID) FROM PUBLIC;
REVOKE ALL ON FUNCTION get_call_transcript_latest(UUID, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION list_transcription_tenants() TO callloom_api;
GRANT EXECUTE ON FUNCTION claim_transcription_job() TO callloom_sync;
GRANT EXECUTE ON FUNCTION finish_transcription_job(UUID, TEXT, TEXT, TEXT, TIMESTAMPTZ) TO callloom_sync;
GRANT EXECUTE ON FUNCTION requeue_stale_transcription_jobs(INT) TO callloom_sync;
GRANT EXECUTE ON FUNCTION get_transcription_minutes_today(UUID) TO callloom_sync;
GRANT EXECUTE ON FUNCTION add_transcription_usage(UUID, NUMERIC) TO callloom_sync;
GRANT EXECUTE ON FUNCTION get_transcription_archive_all(UUID) TO callloom_sync;
GRANT EXECUTE ON FUNCTION upsert_call_recording(UUID, TEXT, TEXT, UUID, TEXT, BIGINT, NUMERIC, TEXT, TEXT, TIMESTAMPTZ, TEXT, TEXT) TO callloom_sync;
GRANT EXECUTE ON FUNCTION insert_call_transcript(UUID, TEXT, TEXT, UUID, TEXT, TEXT, TEXT, TEXT, NUMERIC, TEXT, INT, JSONB) TO callloom_sync;
GRANT EXECUTE ON FUNCTION get_call_recording_object(UUID, TEXT, TEXT) TO callloom_sync;
GRANT EXECUTE ON FUNCTION list_recordings_to_expire(TIMESTAMPTZ, INT) TO callloom_sync;
GRANT EXECUTE ON FUNCTION mark_recording_expired(UUID) TO callloom_sync;
GRANT EXECUTE ON FUNCTION get_call_transcript_latest(UUID, TEXT, TEXT) TO callloom_sync;
