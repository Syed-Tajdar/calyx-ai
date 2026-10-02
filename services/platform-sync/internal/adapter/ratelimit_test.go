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

func rollingMax(times []time.Time) int {
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
	return maxWindow
}

func TestDualWindowLimiterHonoursPerMinuteOnSustainedWalk(t *testing.T) {
	// Prove sustained multi-page walk: never >80 hits in any rolling 60s of the
	// limiter's own admit times. 5/sec alone would allow ~300/min.
	//
	// An earlier run recorded time.Now() after Wait() returned and counted 81.
	// Wait() stamps the hit under the lock, then the test stamped again after the
	// goroutine was scheduled. Under load that second stamp sits far enough past
	// the limiter's stamp that one hit the limiter had already expired still fell
	// inside the test's inclusive [end-60s, end] window. The limiter itself never
	// appends when len(minHits) >= perMin. This test measures the admit times.
	lim := NewDualWindowLimiter(5, 80)
	const n = 160
	admitted := make([]time.Time, 0, n)
	observed := make([]time.Time, 0, n)
	start := time.Now()
	for i := 0; i < n; i++ {
		admitted = append(admitted, lim.Wait())
		observed = append(observed, time.Now())
	}
	elapsed := time.Since(start)
	maxAdmit := rollingMax(admitted)
	maxObserved := rollingMax(observed)
	t.Logf("n=%d elapsed=%s naive_avg_req_per_min=%.2f admit_max=%d observed_after_wait_max=%d first_admit=%s last_admit=%s",
		n, elapsed, float64(n)/elapsed.Minutes(), maxAdmit, maxObserved, admitted[0].Format(time.RFC3339Nano), admitted[len(admitted)-1].Format(time.RFC3339Nano))
	if maxAdmit > 80 {
		t.Fatalf("limiter admit window exceeded 80: got %d", maxAdmit)
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
