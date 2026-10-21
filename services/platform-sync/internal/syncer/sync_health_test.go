package syncer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// v1 sync health (Part 1, item 6). These are the 2026-10-18 outage as tests: redis gone, every
// sync failing at the lock, and Settings still saying `ok`.

// The exact error platform-sync logged from 2026-10-18 15:05 UTC until 2026-10-21.
var redisDNSFailure = errors.New("dial tcp: lookup redis on 127.0.0.11:53: no such host")

const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// encryptCreds builds the blob format crypto.CredentialDecryptor reads: nonce | ciphertext | tag.
func encryptCreds(t *testing.T, creds map[string]string) []byte {
	t.Helper()
	key, _ := hex.DecodeString(testKeyHex)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal(creds)
	return append(nonce, gcm.Seal(nil, nonce, plain, nil)...)
}

// healthySvc runs the real success path with the mock adapter (sync_mode "mock": no network),
// an in-memory dedup and a recording publisher.
func healthySvc(t *testing.T, st *fakeStore) *Service {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEMO_MODE_ENABLED", "true")
	dec, err := crypto.NewCredentialDecryptor(testKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{
		store:     st,
		adapter:   adapter.NewRegistry(),
		producer:  &recordingPublisher{},
		dedup:     &stubContentDedup{hashes: map[string]string{}},
		decryptor: dec,
		logger:    slog.Default(),
		lock:      NewMemorySyncLocker(),
	}
}

func mockItem(t *testing.T) models.SyncIntegration {
	return models.SyncIntegration{
		ID: "int-1", TenantID: "11111111-1111-1111-1111-111111111111",
		Platform: "ringba", SyncMode: "mock",
		APICredentialsBlob: encryptCreds(t, map[string]string{"api_token": "test-only"}),
	}
}

func TestRedisDownIsDegradedWithAnError(t *testing.T) {
	item := models.SyncIntegration{ID: "int-1", Platform: "ringba", SyncMode: "real"}
	st := newFakeStore(item)
	svc := &Service{store: st, logger: slog.Default(), lock: failingLocker{err: redisDNSFailure}}

	if err := svc.syncOne(context.Background(), item, false); err == nil {
		t.Fatal("a sync whose lock cannot be taken must fail")
	}
	r := st.row("int-1")
	if r.status != syncStatusDegraded {
		t.Fatalf("redis down must write %q, got %q", syncStatusDegraded, r.status)
	}
	if r.lastError == nil || !strings.Contains(*r.lastError, "redis unavailable (sync lock)") ||
		!strings.Contains(*r.lastError, "no such host") {
		t.Fatalf("last_sync_error must say redis is unavailable and why, got %v", r.lastError)
	}
	if r.lastSuccessAt != nil || r.lastSyncAt != nil {
		t.Fatal("a failure must not move last_success_at or last_sync_at")
	}
}

func TestRepeatedFailuresBecomeFailed(t *testing.T) {
	t.Setenv("SYNC_FAILED_AFTER", "3")
	item := models.SyncIntegration{ID: "int-1", Platform: "ringba", SyncMode: "real"}
	st := newFakeStore(item)
	svc := &Service{store: st, logger: slog.Default(), lock: failingLocker{err: redisDNSFailure}}

	want := []string{syncStatusDegraded, syncStatusDegraded, syncStatusFailed, syncStatusFailed}
	for i, w := range want {
		_ = svc.syncOne(context.Background(), item, false)
		if got := st.row("int-1").status; got != w {
			t.Fatalf("after failure %d: status %q, want %q", i+1, got, w)
		}
	}
}

func TestRecoveryWritesOkAndClearsTheError(t *testing.T) {
	item := mockItem(t)
	st := newFakeStore(item)

	down := &Service{store: st, logger: slog.Default(), lock: failingLocker{err: redisDNSFailure}}
	for i := 0; i < 3; i++ {
		_ = down.syncOne(context.Background(), item, false)
	}
	if st.row("int-1").status != syncStatusFailed {
		t.Fatalf("setup: want failed, got %q", st.row("int-1").status)
	}

	if err := healthySvc(t, st).syncOne(context.Background(), item, false); err != nil {
		t.Fatalf("recovered sync: %v", err)
	}
	r := st.row("int-1")
	if r.status != "ok" {
		t.Fatalf("a successful sync must write ok, got %q", r.status)
	}
	if r.lastError != nil {
		t.Fatalf("a successful sync must clear last_sync_error, got %q", *r.lastError)
	}
	if r.consecutiveFailures != 0 {
		t.Fatalf("a successful sync must reset the failure count, got %d", r.consecutiveFailures)
	}
	if r.lastSuccessAt == nil {
		t.Fatal("a successful sync must set last_success_at")
	}
}

func TestLastSuccessMovesOnlyOnSuccess(t *testing.T) {
	item := mockItem(t)
	st := newFakeStore(item)
	clock := time.Date(2026, 10, 21, 14, 22, 0, 0, time.UTC)
	st.now = func() time.Time { return clock }

	if err := healthySvc(t, st).syncOne(context.Background(), item, false); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	first := *st.row("int-1").lastSuccessAt

	clock = clock.Add(10 * time.Minute)
	down := &Service{store: st, logger: slog.Default(), lock: failingLocker{err: redisDNSFailure}}
	_ = down.syncOne(context.Background(), item, false)
	down.MarkAllUnavailable(context.Background(), "redis unavailable: "+redisDNSFailure.Error())
	down.failRun(context.Background(), context.Background(), item, errors.New("HTTP 500"), "sweep")
	if got := *st.row("int-1").lastSuccessAt; !got.Equal(first) {
		t.Fatalf("failures moved last_success_at from %s to %s", first, got)
	}

	clock = clock.Add(10 * time.Minute)
	if err := healthySvc(t, st).syncOne(context.Background(), item, false); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := *st.row("int-1").lastSuccessAt; !got.Equal(clock) {
		t.Fatalf("a success must move last_success_at to now (%s), got %s", clock, got)
	}
}

func TestNoSecretReachesLastSyncError(t *testing.T) {
	item := models.SyncIntegration{ID: "int-1", Platform: "ringba", SyncMode: "real"}
	st := newFakeStore(item)
	leaky := errors.New("dial redis://:Sup3rSecretPw@redis:6379 failed; GET https://api.example.com/x?api_key=AKIA123SECRET " +
		"Authorization: Bearer eyJhbGciOi.secret.token postgres://callloom_sync:dbpass123@postgres:5432/callloom " +
		strings.Repeat("x", 600))
	svc := &Service{store: st, logger: slog.Default(), lock: failingLocker{err: leaky}}
	_ = svc.syncOne(context.Background(), item, false)

	msg := *st.row("int-1").lastError
	for _, secret := range []string{"Sup3rSecretPw", "AKIA123SECRET", "eyJhbGciOi.secret.token", "dbpass123"} {
		if strings.Contains(msg, secret) {
			t.Fatalf("last_sync_error leaks %q: %s", secret, msg)
		}
	}
	if n := len([]rune(msg)); n > 300 {
		t.Fatalf("last_sync_error must be short, got %d runes", n)
	}
}

func TestShutdownIsNotAFailure(t *testing.T) {
	item := models.SyncIntegration{ID: "int-1", Platform: "ringba", SyncMode: "real"}
	st := newFakeStore(item)
	svc := &Service{store: st, logger: slog.Default()}
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	svc.failRun(parent, parent, item, context.Canceled, "incremental")
	if r := st.row("int-1"); r.status != "ok" || r.lastError != nil {
		t.Fatalf("a shutdown must not be recorded, got %q / %v", r.status, r.lastError)
	}
}

func TestLockLostMidRunIsARedisFailure(t *testing.T) {
	item := models.SyncIntegration{ID: "int-1", Platform: "ringba", SyncMode: "real"}
	st := newFakeStore(item)
	svc := &Service{store: st, logger: slog.Default()}
	runCtx, cancel := context.WithCancelCause(context.Background())
	cancel(&lockLostError{msg: "sync lock renew failed: " + redisDNSFailure.Error()})
	svc.failRun(context.Background(), runCtx, item, runCtx.Err(), "incremental")

	r := st.row("int-1")
	if r.status != syncStatusDegraded || r.lastError == nil ||
		!strings.Contains(*r.lastError, "redis unavailable (sync lock)") {
		t.Fatalf("a lost lock must be recorded as a redis failure, got %q / %v", r.status, r.lastError)
	}
	if strings.Contains(*r.lastError, "context canceled") {
		t.Fatalf("the cause must replace 'context canceled', got %q", *r.lastError)
	}
}

func TestMarkAllUnavailableRecordsEveryIntegration(t *testing.T) {
	t.Setenv("APP_ENV", "production") // demo rows are skipped in production, real ones are not
	a := models.SyncIntegration{ID: "a", Platform: "ringba", SyncMode: "real"}
	b := models.SyncIntegration{ID: "b", Platform: "retreaver", SyncMode: "real"}
	demo := models.SyncIntegration{ID: "demo", Platform: "ringba", SyncMode: "mock"}
	st := newFakeStore(a, b, demo)
	svc := &Service{store: st, logger: slog.Default()}

	svc.MarkAllUnavailable(context.Background(), "redis unavailable: "+redisDNSFailure.Error())
	for _, id := range []string{"a", "b"} {
		if r := st.row(id); r.status != syncStatusDegraded || r.lastError == nil {
			t.Fatalf("%s: want degraded with an error, got %q / %v", id, r.status, r.lastError)
		}
	}
	if r := st.row("demo"); r.status != "ok" {
		t.Fatalf("a demo integration that does not sync in production must be left alone, got %q", r.status)
	}
}
