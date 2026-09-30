package adapter

import (
	"sync"
	"time"
)

// DualWindowLimiter enforces BOTH a per-second and per-minute rate limit.
// Scope: one limiter instance per external account key (e.g. Ringba account_id).
type DualWindowLimiter struct {
	mu sync.Mutex

	perSec  int
	perMin  int
	secHits []time.Time
	minHits []time.Time
}

func NewDualWindowLimiter(perSec, perMin int) *DualWindowLimiter {
	if perSec < 1 {
		perSec = 5
	}
	if perMin < 1 {
		perMin = 80
	}
	return &DualWindowLimiter{perSec: perSec, perMin: perMin}
}

// Wait blocks until a request is allowed under both windows.
func (l *DualWindowLimiter) Wait() {
	for {
		l.mu.Lock()
		now := time.Now()
		l.secHits = prune(l.secHits, now.Add(-time.Second))
		l.minHits = prune(l.minHits, now.Add(-time.Minute))
		if len(l.secHits) < l.perSec && len(l.minHits) < l.perMin {
			l.secHits = append(l.secHits, now)
			l.minHits = append(l.minHits, now)
			l.mu.Unlock()
			return
		}
		var sleep time.Duration
		if len(l.secHits) >= l.perSec {
			sleep = time.Until(l.secHits[0].Add(time.Second + time.Millisecond))
		}
		if len(l.minHits) >= l.perMin {
			untilMin := time.Until(l.minHits[0].Add(time.Minute + time.Millisecond))
			if untilMin > sleep {
				sleep = untilMin
			}
		}
		l.mu.Unlock()
		if sleep < time.Millisecond {
			sleep = time.Millisecond
		}
		time.Sleep(sleep)
	}
}

func prune(hits []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(hits) && hits[i].Before(cutoff) {
		i++
	}
	if i == 0 {
		return hits
	}
	return append([]time.Time(nil), hits[i:]...)
}

// RingbaAccountLimiters holds per-account_id dual-window limiters (5/sec AND 80/min).
type RingbaAccountLimiters struct {
	mu   sync.Mutex
	byID map[string]*DualWindowLimiter
}

func NewRingbaAccountLimiters() *RingbaAccountLimiters {
	return &RingbaAccountLimiters{byID: map[string]*DualWindowLimiter{}}
}

func (r *RingbaAccountLimiters) ForAccount(accountID string) *DualWindowLimiter {
	if accountID == "" {
		accountID = "_unknown"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if lim, ok := r.byID[accountID]; ok {
		return lim
	}
	lim := NewDualWindowLimiter(5, 80)
	r.byID[accountID] = lim
	return lim
}

// Shared across RingbaAdapter instances in-process.
var ringbaLimiters = NewRingbaAccountLimiters()

// RetreaverAccountLimiters holds per-company_id dual-window limiters.
//
// Stage 4 R1 / C9. Retreaver publishes **no numeric rate limit** — only a documented 429 and
// "Highly concurrent requests are frowned upon"
// (https://retreaver.github.io/core-api-docs/#introduction). R0.3 observed 20 sequential GETs at
// ~3.0 req/s with zero 429s.
//
// The defaults below are therefore a deliberate conservative choice, not a measured ceiling:
// 2/sec and 60/min, keyed on company_id so two integrations on the same Retreaver account share
// one limiter and cannot together exceed it. Because the published guidance is about
// concurrency rather than rate, requests are also serialised per account by the limiter's lock.
type RetreaverAccountLimiters struct {
	mu   sync.Mutex
	byID map[string]*DualWindowLimiter
}

func NewRetreaverAccountLimiters() *RetreaverAccountLimiters {
	return &RetreaverAccountLimiters{byID: map[string]*DualWindowLimiter{}}
}

// RetreaverRatePerSec — `RETREAVER_RATE_PER_SEC` (default 2).
func RetreaverRatePerSec() int { return envInt("RETREAVER_RATE_PER_SEC", 2) }

// RetreaverRatePerMin — `RETREAVER_RATE_PER_MIN` (default 60).
func RetreaverRatePerMin() int { return envInt("RETREAVER_RATE_PER_MIN", 60) }

func (r *RetreaverAccountLimiters) ForAccount(companyID string) *DualWindowLimiter {
	if companyID == "" {
		companyID = "_unknown"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if lim, ok := r.byID[companyID]; ok {
		return lim
	}
	lim := NewDualWindowLimiter(RetreaverRatePerSec(), RetreaverRatePerMin())
	r.byID[companyID] = lim
	return lim
}

// Shared across RetreaverAdapter instances in-process.
var retreaverLimiters = NewRetreaverAccountLimiters()
