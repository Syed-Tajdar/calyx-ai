package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// R2 pre-flight P2. `degraded` must be scheduled exactly like Ringba's `error`: picked up on
// every tick, sweep and deep sweep, and cleared by the next successful walk. All three paths
// select through list_api_sync_integrations(), so the rule lives in SQL rather than in Go and
// can only be tested against a real database. The test skips when one is not reachable.
//
// Seeding needs table privileges, which the runtime `callloom_sync` role deliberately does not
// have — it holds EXECUTE on the SECURITY DEFINER functions only. Point DATABASE_URL at a role
// that owns the table to run these; otherwise they skip.
//
//	go test ./internal/store/...                       # skips
//	DATABASE_URL=postgres://... go test ./internal/store/...

const testTenantID = "11111111-1111-1111-1111-111111111111"

func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping database-backed scheduling test")
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

// seedIntegration inserts a synthetic api_sync row and removes it when the test ends. The
// credentials blob is never decrypted by the selector, so a dummy value is enough.
func seedIntegration(t *testing.T, pool *pgxpool.Pool, status string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO integrations
			(tenant_id, platform, name, webhook_token, connection_mode,
			 api_credentials_blob, sync_status, is_active)
		VALUES ($1::uuid, 'retreaver', 'p2-scheduling-test', $2, 'api_sync',
			 '\x00'::bytea, $3, TRUE)
		RETURNING id`,
		testTenantID, "p2-test-"+time.Now().UTC().Format("20060102150405"), status,
	).Scan(&id)
	if err != nil {
		t.Skipf("cannot seed integration (insufficient grants?): %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM integrations WHERE id = $1::uuid`, id)
	})
	return id
}

func selected(t *testing.T, st *IntegrationStore, id string) bool {
	t.Helper()
	items, err := st.ListAPISync(context.Background())
	if err != nil {
		t.Fatalf("ListAPISync: %v", err)
	}
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func setStatus(t *testing.T, pool *pgxpool.Pool, id, status string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE integrations SET sync_status = $2 WHERE id = $1::uuid`, id, status); err != nil {
		t.Fatalf("set status %s: %v", status, err)
	}
}

// A degraded integration is still handed to the next tick, sweep and deep sweep. If this ever
// fails, the first short window would stop that integration syncing forever.
func TestDegradedIntegrationIsStillScheduled(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	st := NewIntegrationStore(pool)

	id := seedIntegration(t, pool, "degraded")
	if !selected(t, st, id) {
		t.Fatal("degraded integration was not selected — the sync would stall forever")
	}
}

// Every status the syncer can write must be scheduled identically. `degraded` is not special.
func TestSchedulingIgnoresSyncStatus(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	st := NewIntegrationStore(pool)

	id := seedIntegration(t, pool, "idle")
	for _, status := range []string{"idle", "syncing", "ok", "error", "degraded"} {
		setStatus(t, pool, id, status)
		if !selected(t, st, id) {
			t.Errorf("sync_status=%q excluded the integration from scheduling", status)
		}
	}
}

// The recovery half of the rule: a successful walk writes `ok` through the same status-only
// update the syncer uses, which clears `degraded` with no operator action.
func TestSuccessfulWalkClearsDegraded(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	st := NewIntegrationStore(pool)

	id := seedIntegration(t, pool, "degraded")
	if err := st.UpdateSyncStatus(context.Background(), id, "ok", nil); err != nil {
		t.Fatalf("UpdateSyncStatus: %v", err)
	}

	var status string
	var syncErr *string
	if err := pool.QueryRow(context.Background(),
		`SELECT sync_status, last_sync_error FROM integrations WHERE id = $1::uuid`, id,
	).Scan(&status, &syncErr); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "ok" {
		t.Errorf("status = %q, want ok", status)
	}
	if syncErr != nil {
		t.Errorf("last_sync_error = %q, want cleared", *syncErr)
	}
	if !selected(t, st, id) {
		t.Error("recovered integration is no longer scheduled")
	}
}

// Control: is_active is the one flag that does remove an integration from scheduling. Without
// this, the tests above could pass simply because the selector filters nothing at all.
func TestInactiveIntegrationIsNotScheduled(t *testing.T) {
	pool := openTestPool(t)
	defer pool.Close()
	st := NewIntegrationStore(pool)

	id := seedIntegration(t, pool, "degraded")
	if !selected(t, st, id) {
		t.Fatal("precondition: degraded integration should be scheduled")
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE integrations SET is_active = FALSE WHERE id = $1::uuid`, id); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if selected(t, st, id) {
		t.Error("inactive integration was still selected")
	}
}
