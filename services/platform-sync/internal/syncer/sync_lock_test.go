package syncer

import (
	"context"
	"errors"
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
		token, ok, err := lock.TryAcquire(context.Background(), id)
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
	token, ok, err := lock.TryAcquire(context.Background(), "int-1")
	if err != nil || !ok {
		t.Fatalf("setup acquire: ok=%v err=%v", ok, err)
	}
	defer func() { _ = lock.Release(context.Background(), "int-1", token) }()

	svc := &Service{lock: lock}
	err = svc.syncOne(context.Background(), models.SyncIntegration{ID: "int-1"}, true)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err=%v want ErrAlreadyRunning", err)
	}
}

func TestReleaseOnlyDeletesOwnToken(t *testing.T) {
	lock := NewMemorySyncLocker()
	tokenA, ok, err := lock.TryAcquire(context.Background(), "int-1")
	if err != nil || !ok {
		t.Fatalf("acquire A: ok=%v err=%v", ok, err)
	}
	// Simulate TTL expiry: clear then let B acquire.
	_ = lock.Release(context.Background(), "int-1", tokenA)
	tokenB, ok, err := lock.TryAcquire(context.Background(), "int-1")
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
