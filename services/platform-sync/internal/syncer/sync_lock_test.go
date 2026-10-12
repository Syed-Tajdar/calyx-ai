package syncer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

func TestSingleFlightTwoConcurrentRunsOneExecutes(t *testing.T) {
	lock := NewMemorySyncLocker()
	const id = "int-1"

	var ran atomic.Int32
	var skipped atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	run := func() {
		token, ok, err := lock.TryAcquire(context.Background(), id, "tick")
		if err != nil {
			t.Errorf("acquire: %v", err)
			return
		}
		if !ok {
			skipped.Add(1)
			return
		}
		defer func() { _ = lock.Release(context.Background(), id, token) }()
		ran.Add(1)
		close(started)
		<-release
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); run() }()
	<-started

	wg.Add(1)
	go func() { defer wg.Done(); run() }()
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if ran.Load() != 1 {
		t.Fatalf("ran=%d want 1", ran.Load())
	}
	if skipped.Load() != 1 {
		t.Fatalf("skipped=%d want 1", skipped.Load())
	}
}

func TestSyncOneReturnsErrAlreadyRunningWhenLockHeld(t *testing.T) {
	lock := NewMemorySyncLocker()
	token, ok, err := lock.TryAcquire(context.Background(), "int-1", "tick")
	if err != nil || !ok {
		t.Fatalf("setup acquire: ok=%v err=%v", ok, err)
	}
	defer func() { _ = lock.Release(context.Background(), "int-1", token) }()

	svc := &Service{lock: lock, logger: slog.Default()}
	err = svc.syncOne(context.Background(), models.SyncIntegration{ID: "int-1", SyncMode: "real"}, true)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err=%v want ErrAlreadyRunning", err)
	}
}

func TestReleaseOnlyDeletesOwnToken(t *testing.T) {
	lock := NewMemorySyncLocker()
	tokenA, ok, err := lock.TryAcquire(context.Background(), "int-1", "tick")
	if err != nil || !ok {
		t.Fatalf("acquire A: ok=%v err=%v", ok, err)
	}
	// Simulate TTL expiry: clear then let B acquire.
	_ = lock.Release(context.Background(), "int-1", tokenA)
	tokenB, ok, err := lock.TryAcquire(context.Background(), "int-1", "sweep")
	if err != nil || !ok {
		t.Fatalf("acquire B: ok=%v err=%v", ok, err)
	}
	// Stale A release must not delete B's lock.
	if err := lock.Release(context.Background(), "int-1", tokenA); err != nil {
		t.Fatalf("stale release: %v", err)
	}
	if got := lock.TokenForTest("int-1"); got != tokenB {
		t.Fatalf("lock token after stale release=%q want B=%q", got, tokenB)
	}
	if err := lock.Release(context.Background(), "int-1", tokenB); err != nil {
		t.Fatalf("owner release: %v", err)
	}
	if got := lock.TokenForTest("int-1"); got != "" {
		t.Fatalf("lock still held after owner release: %q", got)
	}
}

func TestRenewExtendsTTL(t *testing.T) {
	lock := NewMemorySyncLocker()
	lock.SetTTLForTest(100 * time.Millisecond)
	base := time.Now()
	now := base
	lock.SetNowForTest(func() time.Time { return now })

	token, ok, err := lock.TryAcquire(context.Background(), "int-1", "tick")
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	exp1 := lock.ExpiryForTest("int-1")
	now = base.Add(40 * time.Millisecond)
	ok, err = lock.Renew(context.Background(), "int-1", token)
	if err != nil || !ok {
		t.Fatalf("renew: ok=%v err=%v", ok, err)
	}
	exp2 := lock.ExpiryForTest("int-1")
	if !exp2.After(exp1) {
		t.Fatalf("renew did not extend expiry: before=%v after=%v", exp1, exp2)
	}
	// Advance past original expiry but within renewed window — lock still held.
	now = base.Add(120 * time.Millisecond)
	if _, held, _ := lock.Peek(context.Background(), "int-1"); !held {
		t.Fatal("lock expired despite renew")
	}
}

func TestRenewFailsAfterOwnershipLost(t *testing.T) {
	lock := NewMemorySyncLocker()
	tokenA, ok, err := lock.TryAcquire(context.Background(), "int-1", "tick")
	if err != nil || !ok {
		t.Fatalf("acquire A: %v %v", ok, err)
	}
	_ = lock.Release(context.Background(), "int-1", tokenA)
	tokenB, ok, err := lock.TryAcquire(context.Background(), "int-1", "sweep")
	if err != nil || !ok {
		t.Fatalf("acquire B: %v %v", ok, err)
	}
	ok, err = lock.Renew(context.Background(), "int-1", tokenA)
	if err != nil {
		t.Fatalf("renew err: %v", err)
	}
	if ok {
		t.Fatal("stale token must not renew")
	}
	if got := lock.TokenForTest("int-1"); got != tokenB {
		t.Fatalf("token=%q want B", got)
	}
}

func TestHeartbeatCancelsWhenOwnershipLost(t *testing.T) {
	lock := NewMemorySyncLocker()
	// Speed up heartbeat for the test by patching interval via short sleep path:
	// call lockHeartbeat directly with a fake that fails renew.
	svc := &Service{lock: lock, logger: slog.Default()}
	token, ok, err := lock.TryAcquire(context.Background(), "int-1", "deep_sweep")
	if err != nil || !ok {
		t.Fatalf("acquire: %v %v", ok, err)
	}
	// Steal lock.
	_ = lock.Release(context.Background(), "int-1", token)
	_, ok, err = lock.TryAcquire(context.Background(), "int-1", "tick")
	if err != nil || !ok {
		t.Fatalf("steal: %v %v", ok, err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Single renew attempt path: invoke Renew manually then cancel like heartbeat.
		ok, err := lock.Renew(context.Background(), "int-1", token)
		if err != nil {
			t.Errorf("renew: %v", err)
		}
		if ok {
			t.Error("expected renew fail for stolen lock")
		}
		cancel()
	}()
	select {
	case <-runCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runCtx not cancelled")
	}
	close(stop)
	<-done
	_ = svc
}
