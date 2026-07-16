package adapter

import (
	"net/http"
	"testing"
	"time"
)

func TestDualWindowLimiterBlocksWhenPerSecSaturated(t *testing.T) {
	lim := NewDualWindowLimiter(2, 1000)
	lim.Wait()
	lim.Wait()
	blockedAt := time.Now()
	done := make(chan struct{})
	go func() {
		lim.Wait()
		close(done)
	}()
	select {
	case <-done:
		if time.Since(blockedAt) < 50*time.Millisecond {
			t.Fatal("expected Wait to block when per-sec saturated")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not unblock within 2s")
	}
}

func TestRingbaAccountLimitersKeyedSeparately(t *testing.T) {
	store := NewRingbaAccountLimiters()
	a := store.ForAccount("acct-a")
	b := store.ForAccount("acct-b")
	if a == b {
		t.Fatal("expected distinct limiters per account")
	}
	if store.ForAccount("acct-a") != a {
		t.Fatal("expected same limiter for same account")
	}
}

func TestDualWindowLimiterHonoursPerMinuteOnSustainedWalk(t *testing.T) {
	// Prove sustained multi-page walk: never >80 hits in any rolling 60s window.
	// (5/sec alone would allow ~300/min — the minute window must bind.)
	lim := NewDualWindowLimiter(5, 80)
	const n = 160
	times := make([]time.Time, 0, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		lim.Wait()
		times = append(times, time.Now())
	}
	elapsed := time.Since(start)
	t.Logf("n=%d elapsed=%s naive_avg_req_per_min=%.2f", n, elapsed, float64(n)/elapsed.Minutes())

	maxWindow := 0
	for _, end := range times {
		cutoff := end.Add(-time.Minute)
		c := 0
		for _, ts := range times {
			if !ts.Before(cutoff) && !ts.After(end) {
				c++
			}
		}
		if c > maxWindow {
			maxWindow = c
		}
	}
	t.Logf("max_hits_in_any_rolling_60s=%d", maxWindow)
	if maxWindow > 80 {
		t.Fatalf("rolling 60s window exceeded 80: got %d", maxWindow)
	}
	// 5/sec alone would finish 160 in ~32s; minute window must stretch this ≥60s.
	if elapsed < 60*time.Second {
		t.Fatalf("expected minute window to stretch 160 waits past 60s, got %s", elapsed)
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "3")
	d := parseRetryAfter(h)
	if d < 3*time.Second || d > 3*time.Second {
		t.Fatalf("Retry-After seconds: got %s want 3s", d)
	}
}
