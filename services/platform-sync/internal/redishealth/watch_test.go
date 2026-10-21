package redishealth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingHandler counts log records by level and keeps their messages.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) count(level slog.Level, contains string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.records {
		if r.Level == level && strings.Contains(r.Message, contains) {
			n++
		}
	}
	return n
}

func fastWatch(ping func(context.Context) error, h *recordingHandler) *Watch {
	w := New(ping, slog.New(h), "test")
	w.MinBackoff = 5 * time.Millisecond
	w.MaxBackoff = 20 * time.Millisecond
	w.HealthyEvery = 5 * time.Millisecond
	w.RemindEvery = time.Hour
	w.PingTimeout = 50 * time.Millisecond
	return w
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

// Redis down at startup (2026-10-18): one clear error, retries with backoff, no crash, and
// recovery by itself when redis returns.
func TestDownAtStartLogsOnceRetriesAndRecovers(t *testing.T) {
	var down atomic.Bool
	down.Store(true)
	var pings atomic.Int32
	ping := func(context.Context) error {
		pings.Add(1)
		if down.Load() {
			return errors.New("dial tcp: lookup redis on 127.0.0.11:53: no such host")
		}
		return nil
	}
	h := &recordingHandler{}
	w := fastWatch(ping, h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)
	if w.Up() {
		t.Fatal("Start must report redis down after the first check")
	}
	if !strings.Contains(w.LastError(), "no such host") {
		t.Fatalf("LastError must carry the cause, got %q", w.LastError())
	}
	waitFor(t, func() bool { return pings.Load() >= 5 }, "several retries while down")
	if n := h.count(slog.LevelError, "redis unavailable"); n != 1 {
		t.Fatalf("an outage must be logged once, got %d error lines", n)
	}

	down.Store(false)
	select {
	case <-w.Recovered():
	case <-time.After(2 * time.Second):
		t.Fatal("recovery was not signalled")
	}
	if !w.Up() || w.LastError() != "" {
		t.Fatalf("after recovery: up=%v lastErr=%q", w.Up(), w.LastError())
	}
	if n := h.count(slog.LevelInfo, "redis reachable again"); n != 1 {
		t.Fatalf("recovery must be logged once, got %d", n)
	}
}

func TestOutageWhileRunningIsDetectedAndLoggedOnce(t *testing.T) {
	var down atomic.Bool
	ping := func(context.Context) error {
		if down.Load() {
			return errors.New("dial tcp 172.18.0.3:6379: connect: connection refused")
		}
		return nil
	}
	h := &recordingHandler{}
	w := fastWatch(ping, h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)
	if !w.Up() {
		t.Fatal("redis is up at start")
	}

	down.Store(true)
	waitFor(t, func() bool { return !w.Up() }, "outage detected")
	time.Sleep(60 * time.Millisecond) // several backoff rounds
	if n := h.count(slog.LevelError, "redis unavailable"); n != 1 {
		t.Fatalf("want exactly one error line for the outage, got %d", n)
	}
	down.Store(false)
	waitFor(t, w.Up, "recovered")
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	w := fastWatch(func(context.Context) error { return errors.New("down") }, &recordingHandler{})
	st := &loopState{backoff: w.MinBackoff}
	var waits []time.Duration
	for i := 0; i < 5; i++ {
		waits = append(waits, w.check(context.Background(), st))
	}
	want := []time.Duration{5, 10, 20, 20, 20}
	for i, d := range want {
		if waits[i] != d*time.Millisecond {
			t.Fatalf("wait %d = %s, want %s (all: %v)", i, waits[i], d*time.Millisecond, waits)
		}
	}
}

func TestPasswordInErrorIsRedacted(t *testing.T) {
	w := fastWatch(func(context.Context) error {
		return errors.New("dial redis://:Sup3rSecretPw@redis:6379: no such host")
	}, &recordingHandler{})
	w.check(context.Background(), &loopState{backoff: w.MinBackoff})
	if strings.Contains(w.LastError(), "Sup3rSecretPw") {
		t.Fatalf("password leaked: %q", w.LastError())
	}
}
