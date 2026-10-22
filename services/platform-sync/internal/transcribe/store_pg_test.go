package transcribe

// PGStore against a real database with migration 000036 applied. Skips unless DATABASE_URL is set
// (a role that owns the tables, used only to seed and clean up). With SYNC_DATABASE_URL set the
// store itself runs as callloom_sync, which has EXECUTE on the functions and no table access — the
// production setup.
//
//	go test ./internal/transcribe/                                   # skips
//	DATABASE_URL=postgres://owner@.../callloom SYNC_DATABASE_URL=postgres://callloom_sync@... go test ./internal/transcribe/ -run PG

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const pgTestTenant = "2b2b2b2b-0000-4000-8000-000000000001"

func openPG(t *testing.T, env string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Skipf("%s not set — skipping database-backed transcription store test", env)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("no database: %v", err)
	}
	return pool
}

func TestPGStoreRoundTrip(t *testing.T) {
	owner := openPG(t, "DATABASE_URL")
	defer owner.Close()
	storePool := owner
	if os.Getenv("SYNC_DATABASE_URL") != "" {
		storePool = openPG(t, "SYNC_DATABASE_URL")
		defer storePool.Close()
	}
	ctx := context.Background()
	cleanup := func() {
		for _, q := range []string{
			`DELETE FROM call_transcripts WHERE tenant_id = $1::uuid`,
			`DELETE FROM call_recordings WHERE tenant_id = $1::uuid`,
			`DELETE FROM transcription_usage_daily WHERE tenant_id = $1::uuid`,
			`DELETE FROM transcription_jobs WHERE tenant_id = $1::uuid`,
			`DELETE FROM integrations WHERE tenant_id = $1::uuid`,
			`DELETE FROM tenants WHERE id = $1::uuid`,
		} {
			if _, err := owner.Exec(ctx, q, pgTestTenant); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	defer cleanup()

	if _, err := owner.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1::uuid, 'pg store test')`, pgTestTenant); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	var integrationID string
	if err := owner.QueryRow(ctx, `INSERT INTO integrations (tenant_id, platform, name, webhook_token, connection_mode,
			sync_mode, api_credentials_blob, recording_access_blob, recording_access_set_at)
		VALUES ($1::uuid, 'ringba', 'pg store test', $2, 'api_sync', 'real', '\x0a'::bytea, '\x0b'::bytea, NOW())
		RETURNING id::text`, pgTestTenant, "pg-store-"+time.Now().UTC().Format("150405.000")).Scan(&integrationID); err != nil {
		t.Fatalf("seed integration: %v", err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO transcription_jobs (tenant_id, integration_id, platform, external_call_id, reason,
			occurred_at, duration_seconds, campaign_name, next_try_at)
		VALUES ($1::uuid, $2::uuid, 'ringba', 'PG-CALL-1', 'converted', NOW() - interval '1 hour', 125, 'Medicare Español',
			NOW() - interval '10 years')`, pgTestTenant, integrationID); err != nil {
		t.Fatalf("seed job: %v", err)
	}

	s := NewPGStore(storePool)
	job, err := s.ClaimJob(ctx)
	if err != nil || job == nil {
		t.Fatalf("claim: %v %v", job, err)
	}
	if job.TenantID != pgTestTenant || job.ExternalCallID != "PG-CALL-1" || job.IntegrationID != integrationID ||
		job.SyncMode != "real" || job.DurationSeconds != 125 || job.CampaignName != "Medicare Español" || job.Attempts != 1 ||
		string(job.APICredentialsBlob) != "\x0a" || string(job.RecordingAccessBlob) != "\x0b" || job.OccurredAt.IsZero() {
		t.Fatalf("claimed job fields: %+v", job)
	}

	sha := strings.Repeat("c3", 32)
	recID, err := s.UpsertRecording(ctx, RecordingRow{TenantID: job.TenantID, Platform: "ringba", ExternalCallID: "PG-CALL-1",
		JobID: job.ID, SHA256: sha, Bytes: 4321, DurationSeconds: 124.5, ContentType: "audio/mpeg",
		ObjectKey: ObjectKey(job.TenantID, "ringba", time.Now(), sha, "audio/mpeg"), DownloadedAt: time.Now().Add(-25 * 30 * 24 * time.Hour),
		ArchiveState: "archived"})
	if err != nil || recID == "" {
		t.Fatalf("upsert recording: %q %v", recID, err)
	}
	if _, err := s.InsertTranscript(ctx, TranscriptRow{TenantID: job.TenantID, Platform: "ringba", ExternalCallID: "PG-CALL-1",
		RecordingID: recID, RecordingSHA256: sha, Language: "es", Model: "nova-3", RequestID: "req-pg", DurationSeconds: 124.5,
		Redaction: "deepgram:numbers+calyx", Scrubbed: 1,
		Utterances: []Utterance{{Index: 0, Speaker: 0, StartMs: 0, EndMs: 900, Text: "Hola"}, {Index: 1, Speaker: 1, StartMs: 1000, EndMs: 2000, Text: "[REDACTED]"}}}); err != nil {
		t.Fatalf("insert transcript: %v", err)
	}
	doc, err := s.LatestTranscript(ctx, job.TenantID, "ringba", "PG-CALL-1")
	if err != nil || doc == nil || doc.Version != 1 || doc.Language != "es" || doc.RecordingSHA256 != sha {
		t.Fatalf("latest transcript: %+v %v", doc, err)
	}
	var us []map[string]interface{}
	if err := json.Unmarshal(doc.Utterances, &us); err != nil || len(us) != 2 || us[1]["text"] != "[REDACTED]" {
		t.Fatalf("utterances: %s %v", doc.Utterances, err)
	}
	obj, err := s.GetRecordingObject(ctx, job.TenantID, "ringba", "PG-CALL-1")
	if err != nil || obj == nil || obj.SHA256 != sha || obj.Bytes != 4321 || obj.ArchiveState != "archived" {
		t.Fatalf("recording object: %+v %v", obj, err)
	}
	if none, err := s.GetRecordingObject(ctx, job.TenantID, "ringba", "NOPE"); err != nil || none != nil {
		t.Fatalf("missing recording should be nil: %+v %v", none, err)
	}

	before, err := s.MinutesToday(ctx, job.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.AddUsage(ctx, job.TenantID, 2.08)
	if err != nil || after-before < 2.07 || after-before > 2.09 {
		t.Fatalf("usage: before %v after %v err %v", before, after, err)
	}

	next := time.Now().Add(time.Hour)
	if err := s.FinishJob(ctx, job.ID, "queued", "", "transient: deepgram 503", &next); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if again, err := s.ClaimJob(ctx); err != nil || (again != nil && again.ID == job.ID) {
		t.Fatalf("job retried before next_try_at: %+v %v", again, err)
	}
	if err := s.FinishJob(ctx, job.ID, "done", "", "", nil); err != nil {
		t.Fatalf("finish done: %v", err)
	}

	exp, err := s.ListToExpire(ctx, time.Now().Add(-24*30*24*time.Hour), 1000)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range exp {
		if r.ID == recID {
			found = true
		}
	}
	if !found {
		t.Fatalf("old recording not listed for expiry: %+v", exp)
	}
	if err := s.MarkExpired(ctx, recID); err != nil {
		t.Fatal(err)
	}
	if obj, _ := s.GetRecordingObject(ctx, job.TenantID, "ringba", "PG-CALL-1"); obj == nil || obj.ArchiveState != "expired" {
		t.Fatalf("not expired: %+v", obj)
	}
	if _, err := s.RequeueStale(ctx, 30); err != nil {
		t.Fatal(err)
	}
}
