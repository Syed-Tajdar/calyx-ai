package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

type fakeWriter struct {
	mu        sync.Mutex
	failUntil int
	calls     int
	pingOK    bool
	failMsg   error
	written   []models.PingMessage
}

func (f *fakeWriter) InsertBatch(_ context.Context, pings []models.PingMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failUntil {
		if f.failMsg != nil {
			return f.failMsg
		}
		return errors.New("transient clickhouse error")
	}
	f.written = append(f.written, pings...)
	return nil
}

func (f *fakeWriter) Ping(context.Context) error {
	if f.pingOK {
		return nil
	}
	return errors.New("ch down")
}

type fakeReader struct {
	mu      sync.Mutex
	queue   []kafkago.Message
	commits []kafkago.Message
}

func (r *fakeReader) Fetch(ctx context.Context) (kafkago.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queue) == 0 {
		select {
		case <-ctx.Done():
			return kafkago.Message{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return kafkago.Message{}, fmt.Errorf("empty")
		}
	}
	m := r.queue[0]
	r.queue = r.queue[1:]
	return m, nil
}

func (r *fakeReader) Commit(_ context.Context, msgs ...kafkago.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commits = append(r.commits, msgs...)
	return nil
}

type fakeDLQ struct {
	mu    sync.Mutex
	msgs  [][]byte
	fail  bool
	count int
}

func (d *fakeDLQ) Publish(_ context.Context, _, value []byte, _ []kafkago.Header) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.count++
	if d.fail {
		return errors.New("dlq unavailable")
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	d.msgs = append(d.msgs, cp)
	return nil
}

func testPing(id string) models.PingMessage {
	return models.PingMessage{
		TenantID:       "11111111-1111-1111-1111-111111111111",
		Platform:       "ringba",
		ExternalCallID: id,
		ANIHash:        "abc",
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSizeTriggeredFlushKeepsRunAlive(t *testing.T) {
	w := &fakeWriter{failUntil: 2, pingOK: false} // fail first batch path (batch + one-by-one each call)
	// failUntil on every InsertBatch — size flush calls InsertBatch once for batch then N for one-by-one
	w.failUntil = 100
	w.pingOK = false // outage → retry forever, never poison
	r := &fakeReader{}
	d := &fakeDLQ{}
	cfg := config.Config{BatchSize: 3, BatchFlushMillis: 60_000}
	svc := NewService(cfg, r, w, slog.New(slog.NewTextHandler(io.Discard, nil)), d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	for i := 0; i < 3; i++ {
		p := testPing(fmt.Sprintf("smoke-size-%d", i))
		r.mu.Lock()
		r.queue = append(r.queue, kafkago.Message{
			Topic: "t", Partition: 0, Offset: int64(i), Value: mustJSON(t, p),
		})
		r.mu.Unlock()
	}
	time.Sleep(500 * time.Millisecond)
	if !svc.paused.Load() {
		t.Fatal("expected fetch paused while flush failing and batch full")
	}
	select {
	case err := <-done:
		t.Fatalf("Run exited early: %v", err)
	default:
	}
	// Recover CH
	w.mu.Lock()
	w.failUntil = 0
	w.calls = 0
	w.pingOK = true
	w.mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		n := len(w.written)
		w.mu.Unlock()
		if n >= 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	w.mu.Lock()
	n := len(w.written)
	w.mu.Unlock()
	if n < 3 {
		t.Fatalf("written=%d want >=3 after recovery", n)
	}
	r.mu.Lock()
	commits := len(r.commits)
	r.mu.Unlock()
	if commits < 3 {
		t.Fatalf("commits=%d want >=3", commits)
	}
	cancel()
	<-done
}

func TestUnknownErrorPoisonsAfterNWhenCHHealthy(t *testing.T) {
	w := &fakeWriter{failUntil: 1000, pingOK: true, failMsg: errors.New("weird unknown row error")}
	r := &fakeReader{}
	d := &fakeDLQ{}
	cfg := config.Config{BatchSize: 1, BatchFlushMillis: 50}
	svc := NewService(cfg, r, w, slog.New(slog.NewTextHandler(io.Discard, nil)), d)
	base := time.Now()
	clock := base
	svc.now = func() time.Time { return clock }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = svc.Run(ctx) }()

	p := testPing("smoke-poison-unknown")
	r.mu.Lock()
	r.queue = append(r.queue, kafkago.Message{
		Topic: "t", Partition: 0, Offset: 42, Value: mustJSON(t, p), Key: []byte("k"),
	})
	r.mu.Unlock()

	// Advance fake clock past min interval between each counted attempt (5 total → span ≥ 3m).
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		n := d.count
		d.mu.Unlock()
		if n >= 1 {
			break
		}
		clock = clock.Add(UnknownAttemptMinInterval)
		time.Sleep(80 * time.Millisecond)
	}
	d.mu.Lock()
	if d.count < 1 {
		d.mu.Unlock()
		cancel()
		t.Fatal("expected DLQ publish after N spaced attempts")
	}
	d.mu.Unlock()
	span := clock.Sub(base)
	if span < UnknownAttemptMinSpan() {
		t.Fatalf("poison span=%v want >= %v (few minutes)", span, UnknownAttemptMinSpan())
	}
	r.mu.Lock()
	if len(r.commits) < 1 {
		r.mu.Unlock()
		cancel()
		t.Fatal("expected commit after DLQ")
	}
	r.mu.Unlock()
	cancel()
}

func TestTransientCHErrorDoesNotCountTowardPoison(t *testing.T) {
	ex := &ch.Exception{Code: chCodeTooManyParts, Name: "TOO_MANY_PARTS", Message: "parts"}
	w := &fakeWriter{failUntil: 1000, pingOK: true, failMsg: ex}
	r := &fakeReader{}
	d := &fakeDLQ{}
	cfg := config.Config{BatchSize: 1, BatchFlushMillis: 50}
	svc := NewService(cfg, r, w, slog.New(slog.NewTextHandler(io.Discard, nil)), d)
	base := time.Now()
	clock := base
	svc.now = func() time.Time { return clock }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = svc.Run(ctx) }()

	p := testPing("smoke-too-many-parts")
	r.mu.Lock()
	r.queue = append(r.queue, kafkago.Message{
		Topic: "t", Partition: 0, Offset: 99, Value: mustJSON(t, p),
	})
	r.mu.Unlock()

	for i := 0; i < 20; i++ {
		clock = clock.Add(UnknownAttemptMinInterval)
		time.Sleep(60 * time.Millisecond)
	}
	d.mu.Lock()
	if d.count != 0 {
		d.mu.Unlock()
		cancel()
		t.Fatal("TOO_MANY_PARTS must not DLQ even with healthy ping")
	}
	d.mu.Unlock()
	cancel()
}

func TestUnknownAttemptBackoffRequiresMinInterval(t *testing.T) {
	svc := NewService(config.Config{BatchSize: 1}, &fakeReader{}, &fakeWriter{pingOK: true},
		slog.New(slog.NewTextHandler(io.Discard, nil)), &fakeDLQ{})
	base := time.Now()
	clock := base
	svc.now = func() time.Time { return clock }
	msg := kafkago.Message{Topic: "t", Partition: 0, Offset: 1}

	n, _, counted := svc.bumpAttempts(msg)
	if n != 1 || !counted {
		t.Fatalf("first: n=%d counted=%v", n, counted)
	}
	n, _, counted = svc.bumpAttempts(msg)
	if counted || n != 1 {
		t.Fatalf("immediate second must not count: n=%d counted=%v", n, counted)
	}
	clock = base.Add(UnknownAttemptMinInterval)
	n, span, counted := svc.bumpAttempts(msg)
	if !counted || n != 2 {
		t.Fatalf("after interval: n=%d counted=%v", n, counted)
	}
	if span < UnknownAttemptMinInterval {
		t.Fatalf("span=%v", span)
	}
	// Advance to attempt 5
	for i := 0; i < 3; i++ {
		clock = clock.Add(UnknownAttemptMinInterval)
		n, span, counted = svc.bumpAttempts(msg)
	}
	if n != 5 || !counted {
		t.Fatalf("fifth: n=%d counted=%v", n, counted)
	}
	if span < UnknownAttemptMinSpan() {
		t.Fatalf("span to poison=%v want >= %v", span, UnknownAttemptMinSpan())
	}
}

func TestIsTransientClickHouseErrorByCode(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&ch.Exception{Code: chCodeTooManyParts, Name: "TOO_MANY_PARTS"}, true},
		{&ch.Exception{Code: chCodeMemoryLimitExceeded, Name: "MEMORY_LIMIT_EXCEEDED"}, true},
		{&ch.Exception{Code: chCodeTimeoutExceeded, Name: "TIMEOUT_EXCEEDED"}, true},
		{&ch.Exception{Code: chCodeNetworkError, Name: "NETWORK_ERROR"}, true},
		{&ch.Exception{Code: 60, Name: "UNKNOWN_TABLE"}, false},
		{errors.New("connection refused"), true},
		{errors.New("i/o timeout"), true},
		{errors.New("weird unknown row error"), false},
	}
	for _, tc := range cases {
		if got := isTransientClickHouseError(tc.err); got != tc.want {
			t.Fatalf("err=%v got=%v want=%v", tc.err, got, tc.want)
		}
	}
}

func TestUnknownErrorRetriesForeverWhenCHUnhealthy(t *testing.T) {
	w := &fakeWriter{failUntil: 1000, pingOK: false, failMsg: errors.New("weird unknown")}
	r := &fakeReader{}
	d := &fakeDLQ{}
	cfg := config.Config{BatchSize: 1, BatchFlushMillis: 60_000}
	svc := NewService(cfg, r, w, slog.New(slog.NewTextHandler(io.Discard, nil)), d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = svc.Run(ctx) }()

	p := testPing("smoke-outage")
	r.mu.Lock()
	r.queue = append(r.queue, kafkago.Message{
		Topic: "t", Partition: 0, Offset: 7, Value: mustJSON(t, p),
	})
	r.mu.Unlock()
	time.Sleep(800 * time.Millisecond)
	d.mu.Lock()
	if d.count != 0 {
		d.mu.Unlock()
		cancel()
		t.Fatal("must not DLQ during CH outage")
	}
	d.mu.Unlock()
	cancel()
}

func TestDLQFailureDoesNotCommit(t *testing.T) {
	w := &fakeWriter{pingOK: true}
	r := &fakeReader{}
	d := &fakeDLQ{fail: true}
	cfg := config.Config{BatchSize: 10, BatchFlushMillis: 60_000}
	svc := NewService(cfg, r, w, slog.New(slog.NewTextHandler(io.Discard, nil)), d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = svc.Run(ctx) }()

	bad := testPing("x")
	bad.TenantID = "not-a-uuid"
	r.mu.Lock()
	r.queue = append(r.queue, kafkago.Message{
		Topic: "t", Partition: 0, Offset: 9, Value: mustJSON(t, bad),
	})
	r.mu.Unlock()
	time.Sleep(500 * time.Millisecond)
	r.mu.Lock()
	n := len(r.commits)
	r.mu.Unlock()
	if n != 0 {
		cancel()
		t.Fatalf("commits=%d want 0 when DLQ fails", n)
	}
	cancel()
}
