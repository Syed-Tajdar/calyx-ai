package transcribe

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Job is one claimed row of transcription_jobs, with what the worker needs from its integration.
type Job struct {
	ID                  string
	TenantID            string
	IntegrationID       string
	Platform            string
	ExternalCallID      string
	Reason              string
	OccurredAt          time.Time
	DurationSeconds     int
	CampaignName        string
	Attempts            int
	SyncMode            string
	APICredentialsBlob  []byte
	RecordingAccessBlob []byte
}

type RecordingRow struct {
	TenantID, Platform, ExternalCallID, JobID string
	SHA256                                    string
	Bytes                                     int64
	DurationSeconds                           float64
	ContentType                               string
	ObjectKey                                 string
	DownloadedAt                              time.Time
	ArchiveState                              string // archived | not_archived
	ArchiveError                              string
}

type TranscriptRow struct {
	TenantID, Platform, ExternalCallID string
	RecordingID, RecordingSHA256       string
	Language, Model, RequestID         string
	DurationSeconds                    float64
	Redaction                          string
	Scrubbed                           int
	Utterances                         []Utterance
}

type RecordingObject struct {
	ID              string
	SHA256          string
	Bytes           int64
	ContentType     string
	ObjectKey       string
	ArchiveState    string
	DownloadedAt    time.Time
	DurationSeconds float64
}

type TranscriptDoc struct {
	ID              string          `json:"transcript_id"`
	Version         int             `json:"transcript_version"`
	Language        string          `json:"language"`
	Model           string          `json:"model"`
	RequestID       string          `json:"deepgram_request_id"`
	DurationSeconds float64         `json:"duration_seconds"`
	Redaction       string          `json:"redaction"`
	RecordingSHA256 string          `json:"recording_sha256"`
	CreatedAt       time.Time       `json:"created_at"`
	Utterances      json.RawMessage `json:"utterances"`
}

type ExpireRow struct{ ID, ObjectKey string }

// Store is what the worker, the cleanup job and the internal routes need. PGStore implements it
// through SECURITY DEFINER functions (migration 000036); callloom_sync has no table access.
type Store interface {
	ClaimJob(ctx context.Context) (*Job, error)
	FinishJob(ctx context.Context, id, state, noRecReason, errText string, nextTry *time.Time) error
	RequeueStale(ctx context.Context, staleMinutes int) (int, error)
	MinutesToday(ctx context.Context, tenantID string) (float64, error)
	AddUsage(ctx context.Context, tenantID string, minutes float64) (float64, error)
	UpsertRecording(ctx context.Context, r RecordingRow) (string, error)
	InsertTranscript(ctx context.Context, t TranscriptRow) (string, error)
	GetRecordingObject(ctx context.Context, tenantID, platform, externalCallID string) (*RecordingObject, error)
	LatestTranscript(ctx context.Context, tenantID, platform, externalCallID string) (*TranscriptDoc, error)
	ListToExpire(ctx context.Context, before time.Time, limit int) ([]ExpireRow, error)
	MarkExpired(ctx context.Context, id string) error
}

type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) ClaimJob(ctx context.Context) (*Job, error) {
	rows, err := s.pool.Query(ctx, `SELECT * FROM claim_transcription_job()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var j Job
	var integrationID, campaign, syncMode *string
	var occurred *time.Time
	var duration *int
	if err := rows.Scan(&j.ID, &j.TenantID, &integrationID, &j.Platform, &j.ExternalCallID, &j.Reason, &occurred,
		&duration, &campaign, &j.Attempts, &syncMode, &j.APICredentialsBlob, &j.RecordingAccessBlob); err != nil {
		return nil, err
	}
	if integrationID != nil {
		j.IntegrationID = *integrationID
	}
	if occurred != nil {
		j.OccurredAt = *occurred
	}
	if duration != nil {
		j.DurationSeconds = *duration
	}
	if campaign != nil {
		j.CampaignName = *campaign
	}
	if syncMode != nil {
		j.SyncMode = *syncMode
	}
	return &j, rows.Err()
}

func (s *PGStore) FinishJob(ctx context.Context, id, state, noRecReason, errText string, nextTry *time.Time) error {
	_, err := s.pool.Exec(ctx, `SELECT finish_transcription_job($1::uuid, $2, $3, $4, $5)`,
		id, state, nullable(noRecReason), nullable(errText), nextTry)
	return err
}

func (s *PGStore) RequeueStale(ctx context.Context, staleMinutes int) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT requeue_stale_transcription_jobs($1)`, staleMinutes).Scan(&n)
	return n, err
}

func (s *PGStore) MinutesToday(ctx context.Context, tenantID string) (float64, error) {
	var m float64
	err := s.pool.QueryRow(ctx, `SELECT get_transcription_minutes_today($1::uuid)::float8`, tenantID).Scan(&m)
	return m, err
}

func (s *PGStore) AddUsage(ctx context.Context, tenantID string, minutes float64) (float64, error) {
	var m float64
	err := s.pool.QueryRow(ctx, `SELECT add_transcription_usage($1::uuid, $2::numeric)::float8`, tenantID, minutes).Scan(&m)
	return m, err
}

func (s *PGStore) UpsertRecording(ctx context.Context, r RecordingRow) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT upsert_call_recording($1::uuid, $2, $3, $4::uuid, $5, $6, $7::numeric, $8, $9, $10, $11, $12)::text`,
		r.TenantID, r.Platform, r.ExternalCallID, r.JobID, r.SHA256, r.Bytes, r.DurationSeconds, r.ContentType,
		nullable(r.ObjectKey), r.DownloadedAt, r.ArchiveState, nullable(r.ArchiveError)).Scan(&id)
	return id, err
}

func (s *PGStore) InsertTranscript(ctx context.Context, t TranscriptRow) (string, error) {
	us := t.Utterances
	if us == nil {
		us = []Utterance{}
	}
	body, err := json.Marshal(us)
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx, `SELECT insert_call_transcript($1::uuid, $2, $3, $4::uuid, $5, $6, $7, $8, $9::numeric, $10, $11, $12::jsonb)::text`,
		t.TenantID, t.Platform, t.ExternalCallID, t.RecordingID, t.RecordingSHA256, nullable(t.Language), t.Model,
		nullable(t.RequestID), t.DurationSeconds, t.Redaction, t.Scrubbed, string(body)).Scan(&id)
	return id, err
}

func (s *PGStore) GetRecordingObject(ctx context.Context, tenantID, platform, externalCallID string) (*RecordingObject, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, sha256, bytes, content_type, COALESCE(object_key, ''), archive_state,
		downloaded_at, COALESCE(duration_seconds, 0)::float8 FROM get_call_recording_object($1::uuid, $2, $3)`,
		tenantID, platform, externalCallID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var o RecordingObject
	if err := rows.Scan(&o.ID, &o.SHA256, &o.Bytes, &o.ContentType, &o.ObjectKey, &o.ArchiveState, &o.DownloadedAt, &o.DurationSeconds); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *PGStore) LatestTranscript(ctx context.Context, tenantID, platform, externalCallID string) (*TranscriptDoc, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, transcript_version, COALESCE(language, ''), model,
		COALESCE(deepgram_request_id, ''), COALESCE(duration_seconds, 0)::float8, redaction, recording_sha256, created_at,
		utterances FROM get_call_transcript_latest($1::uuid, $2, $3)`, tenantID, platform, externalCallID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var d TranscriptDoc
	var us []byte
	if err := rows.Scan(&d.ID, &d.Version, &d.Language, &d.Model, &d.RequestID, &d.DurationSeconds, &d.Redaction,
		&d.RecordingSHA256, &d.CreatedAt, &us); err != nil {
		return nil, err
	}
	d.Utterances = us
	return &d, nil
}

func (s *PGStore) ListToExpire(ctx context.Context, before time.Time, limit int) ([]ExpireRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, COALESCE(object_key, '') FROM list_recordings_to_expire($1, $2)`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpireRow
	for rows.Next() {
		var r ExpireRow
		if err := rows.Scan(&r.ID, &r.ObjectKey); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGStore) MarkExpired(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `SELECT mark_recording_expired($1::uuid)`, id)
	return err
}

func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
