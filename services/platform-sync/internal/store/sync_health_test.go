package store

import (
	"context"
	"testing"
	"time"
)

// v1 sync health (migration 000034) against the real SQL. Database-backed: skips unless
// DATABASE_URL points at a role that owns the integrations table (see scheduling_test.go).

type healthRow struct {
	status        string
	syncErr       *string
	failures      int
	lastSuccessAt *time.Time
	lastSyncAt    *time.Time
}

func readHealth(t *testing.T, st *IntegrationStore, id string) healthRow {
	t.Helper()
	var r healthRow
	if err := st.pool.QueryRow(context.Background(),
		`SELECT sync_status, last_sync_error, consecutive_failures, last_success_at, last_sync_at
		 FROM integrations WHERE id = $1::uuid`, id,
	).Scan(&r.status, &r.syncErr, &r.failures, &r.lastSuccessAt, &r.lastSyncAt); err != nil {
		t.Fatalf("read back: %v", err)
	}
	return r
}

// seedHealthIntegration is seedIntegration with a unique name: (tenant, platform, name) is
// unique, and a row left behind by another test must not skip this one.
func seedHealthIntegration(t *testing.T, st *IntegrationStore) string {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	var id string
	if err := st.pool.QueryRow(ctx, `
		INSERT INTO integrations
			(tenant_id, platform, name, webhook_token, connection_mode,
			 api_credentials_blob, sync_status, is_active)
		VALUES ($1::uuid, 'ringba', $2, $3, 'api_sync', '\x00'::bytea, 'ok', TRUE)
		RETURNING id`,
		testTenantID, "sync-health-test-"+suffix, "sync-health-test-"+suffix,
	).Scan(&id); err != nil {
		t.Skipf("cannot seed integration (insufficient grants?): %v", err)
	}
	t.Cleanup(func() {
		_, _ = st.pool.Exec(context.Background(), `DELETE FROM integrations WHERE id = $1::uuid`, id)
	})
	return id
}

func TestFailuresAreDegradedThenFailedAndSuccessClearsThem(t *testing.T) {
	pool := openTestPool(t)
	t.Cleanup(pool.Close) // registered first, so it runs after the row cleanup (Cleanup is LIFO; a defer would close the pool too early)
	st := NewIntegrationStore(pool)
	ctx := context.Background()
	id := seedHealthIntegration(t, st)

	want := []string{"degraded", "degraded", "failed", "failed"}
	for i, w := range want {
		got, err := st.RecordSyncFailure(ctx, id, "redis unavailable (sync lock): dial tcp: lookup redis: no such host", 3)
		if err != nil {
			t.Fatalf("RecordSyncFailure: %v", err)
		}
		if got != w {
			t.Fatalf("failure %d returned %q, want %q", i+1, got, w)
		}
	}
	r := readHealth(t, st, id)
	if r.status != "failed" || r.failures != 4 || r.syncErr == nil {
		t.Fatalf("after 4 failures: %+v", r)
	}
	if r.lastSuccessAt != nil || r.lastSyncAt != nil {
		t.Fatalf("failures must not move last_success_at / last_sync_at: %+v", r)
	}

	before := time.Now().Add(-time.Second)
	windowEnd := time.Now().UTC().Add(-time.Minute)
	if err := st.UpdateSyncState(ctx, id, "cursor-1", windowEnd, "ok", nil); err != nil {
		t.Fatalf("UpdateSyncState: %v", err)
	}
	r = readHealth(t, st, id)
	if r.status != "ok" || r.syncErr != nil || r.failures != 0 {
		t.Fatalf("success must write ok, clear the error and reset the count: %+v", r)
	}
	if r.lastSuccessAt == nil || r.lastSuccessAt.Before(before) {
		t.Fatalf("success must set last_success_at to now, got %v", r.lastSuccessAt)
	}
	if r.lastSyncAt == nil || r.lastSyncAt.Sub(windowEnd).Abs() > time.Second {
		t.Fatalf("last_sync_at must stay the window end, got %v want %v", r.lastSyncAt, windowEnd)
	}

	success := *r.lastSuccessAt
	if _, err := st.RecordSyncFailure(ctx, id, "network error", 3); err != nil {
		t.Fatalf("RecordSyncFailure: %v", err)
	}
	if r = readHealth(t, st, id); r.lastSuccessAt == nil || !r.lastSuccessAt.Equal(success) {
		t.Fatalf("a failure after a success moved last_success_at: %v -> %v", success, r.lastSuccessAt)
	}
}

func TestFailureOnMissingRowIsHarmless(t *testing.T) {
	pool := openTestPool(t)
	t.Cleanup(pool.Close) // registered first, so it runs after the row cleanup (Cleanup is LIFO; a defer would close the pool too early)
	st := NewIntegrationStore(pool)
	status, err := st.RecordSyncFailure(context.Background(), "00000000-0000-0000-0000-000000000000", "x", 3)
	if err != nil || status != "" {
		t.Fatalf("a deleted integration must be a no-op, got %q / %v", status, err)
	}
}
