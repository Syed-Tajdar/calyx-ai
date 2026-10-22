DROP FUNCTION IF EXISTS get_call_transcript_latest(UUID, TEXT, TEXT);
DROP FUNCTION IF EXISTS mark_recording_expired(UUID);
DROP FUNCTION IF EXISTS list_recordings_to_expire(TIMESTAMPTZ, INT);
DROP FUNCTION IF EXISTS get_call_recording_object(UUID, TEXT, TEXT);
DROP FUNCTION IF EXISTS insert_call_transcript(UUID, TEXT, TEXT, UUID, TEXT, TEXT, TEXT, TEXT, NUMERIC, TEXT, INT, JSONB);
DROP FUNCTION IF EXISTS upsert_call_recording(UUID, TEXT, TEXT, UUID, TEXT, BIGINT, NUMERIC, TEXT, TEXT, TIMESTAMPTZ, TEXT, TEXT);
DROP FUNCTION IF EXISTS get_transcription_archive_all(UUID);
DROP FUNCTION IF EXISTS add_transcription_usage(UUID, NUMERIC);
DROP FUNCTION IF EXISTS get_transcription_minutes_today(UUID);
DROP FUNCTION IF EXISTS requeue_stale_transcription_jobs(INT);
DROP FUNCTION IF EXISTS finish_transcription_job(UUID, TEXT, TEXT, TEXT, TIMESTAMPTZ);
DROP FUNCTION IF EXISTS claim_transcription_job();
DROP FUNCTION IF EXISTS list_transcription_tenants();

DROP TABLE IF EXISTS transcription_settings;
DROP TABLE IF EXISTS transcription_usage_daily;
DROP TABLE IF EXISTS recording_access_audit;
DROP FUNCTION IF EXISTS recording_access_audit_immutable();
DROP TABLE IF EXISTS call_transcript_utterances;
DROP TABLE IF EXISTS call_transcripts;
DROP TABLE IF EXISTS call_recordings;
DROP TABLE IF EXISTS transcription_jobs;

ALTER TABLE integrations
    DROP COLUMN IF EXISTS recording_access_set_at,
    DROP COLUMN IF EXISTS recording_access_blob;
