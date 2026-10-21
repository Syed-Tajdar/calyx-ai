// Package redishealth watches redis reachability for ingestion (v1 sync health, item 5; same as platform-sync's).
//
// On 2026-10-18 redis came up without a network after a reboot. The service must not crash or
// flood the log when that happens: it logs one clear error, keeps retrying with backoff, logs a
// reminder every few minutes while it lasts, and recovers by itself when redis returns.
package redishealth

import (
	"context"
	"log/slog"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

var userinfoRE = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)[^\s/@"']+@`)

// Watch pings redis in the background and reports whether it is reachable.
type Watch struct {
	ping    func(context.Context) error
	logger  *slog.Logger
	service string

	up        atomic.Bool
	mu        sync.Mutex
	lastErr   string
	recovered chan struct{}

	// Tunables (tests shorten them).
	MinBackoff   time.Duration // first retry after a failure
	MaxBackoff   time.Duration // retry interval cap while down
	HealthyEvery time.Duration // check interval while up
	RemindEvery  time.Duration // "still unavailable" reminder while down
	PingTimeout  time.Duration
}

// New returns a watch for the given ping function (e.g. client.Ping(ctx).Err()).
func New(ping func(context.Context) error, logger *slog.Logger, service string) *Watch {
	return &Watch{
		ping:         ping,
		logger:       logger,
		service:      service,
		recovered:    make(chan struct{}, 1),
		MinBackoff:   time.Second,
		MaxBackoff:   30 * time.Second,
		HealthyEvery: 5 * time.Second,
		RemindEvery:  5 * time.Minute,
		PingTimeout:  2 * time.Second,
	}
}

// Up reports whether the last check reached redis.
func (w *Watch) Up() bool { return w.up.Load() }

// LastError is the last ping error (redacted), empty while up.
func (w *Watch) LastError() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastErr
}

// Recovered receives once each time redis comes back after being down.
func (w *Watch) Recovered() <-chan struct{} { return w.recovered }

// Start runs one check now (so callers know the state before serving), then keeps watching
// until ctx is cancelled.
func (w *Watch) Start(ctx context.Context) {
	st := &loopState{backoff: w.MinBackoff}
	wait := w.check(ctx, st)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = w.check(ctx, st)
		}
	}()
}

type loopState struct {
	backoff    time.Duration
	downSince  time.Time
	lastRemind time.Time
	checked    bool
}

// check pings once, logs on transitions only, and returns how long to wait before the next one.
func (w *Watch) check(ctx context.Context, st *loopState) time.Duration {
	pctx, cancel := context.WithTimeout(ctx, w.PingTimeout)
	err := w.ping(pctx)
	cancel()
	now := time.Now()
	wasUp := w.up.Load()
	first := !st.checked
	st.checked = true

	if err == nil {
		if !wasUp && !st.downSince.IsZero() {
			w.logger.Info("redis reachable again — resuming",
				"service", w.service, "down_for", now.Sub(st.downSince).Round(time.Second).String())
			select {
			case w.recovered <- struct{}{}:
			default:
			}
		}
		w.up.Store(true)
		w.mu.Lock()
		w.lastErr = ""
		w.mu.Unlock()
		st.backoff = w.MinBackoff
		st.downSince = time.Time{}
		return w.HealthyEvery
	}

	text := userinfoRE.ReplaceAllString(err.Error(), `${1}[REDACTED]@`)
	w.mu.Lock()
	w.lastErr = text
	w.mu.Unlock()
	w.up.Store(false)
	if wasUp || first {
		st.downSince = now
		st.lastRemind = now
		w.logger.Error("redis unavailable — retrying with backoff",
			"service", w.service, "error", text, "max_backoff", w.MaxBackoff.String())
	} else if now.Sub(st.lastRemind) >= w.RemindEvery {
		st.lastRemind = now
		w.logger.Warn("redis still unavailable",
			"service", w.service, "error", text,
			"down_for", now.Sub(st.downSince).Round(time.Second).String())
	}
	wait := st.backoff
	st.backoff *= 2
	if st.backoff > w.MaxBackoff {
		st.backoff = w.MaxBackoff
	}
	return wait
}
