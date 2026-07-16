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
