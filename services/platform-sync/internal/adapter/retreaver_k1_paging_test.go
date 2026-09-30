package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"testing"
	"time"
)

// Stage 4 R0.4 — deterministic reproduction of K1 / audit F012.
//
// The Retreaver adapter pages with `page` + `per_page` over a window that has a start
// (`updated_at_start`) but NO end, sorted `updated_at desc`. The result set is therefore live:
// when a call's `updated_at` changes during the page walk it jumps to position 1 and pushes every
// row after it down by one. A row that was going to be the first row of page 2 slides to the last
// row of page 1 — which has already been fetched — and is never returned.
//
// This is the same failure that silently skipped 431 Ringba calls before S1A.
//
// The test does NOT fix anything. It asserts the bug is present so R1 has a red test to turn green.

type k1Call struct {
	uuid      string
	updatedAt time.Time
}

// k1Server serves Retreaver-shaped `calls.json` honouring page/per_page over a desc-sorted,
// mutable dataset. `mutate` runs after each response so the next page sees a changed ordering.
func k1Server(t *testing.T, calls []k1Call, mutate func(served int, calls []k1Call)) *httptest.Server {
	t.Helper()
	served := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage < 1 {
			perPage = 100
		}

		// Match the real API (R0.3): honour updated_at_start / updated_at_end, sort desc,
		// and expose the exact row count for the window in the `Total` header.
		var wStart, wEnd time.Time
		if v := r.URL.Query().Get("updated_at_start"); v != "" {
			wStart, _ = time.Parse(time.RFC3339, v)
		}
		if v := r.URL.Query().Get("updated_at_end"); v != "" {
			wEnd, _ = time.Parse(time.RFC3339, v)
		}

		ordered := make([]k1Call, 0, len(calls))
		for _, c := range calls {
			if !wStart.IsZero() && c.updatedAt.Before(wStart) {
				continue
			}
			// Unbounded end (the pre-R1 behaviour) keeps every newer row in the set —
			// which is precisely how a mid-walk update shifts the pages.
			if !wEnd.IsZero() && !c.updatedAt.Before(wEnd) {
				continue
			}
			ordered = append(ordered, c)
		}
		sort.SliceStable(ordered, func(i, j int) bool {
			return ordered[i].updatedAt.After(ordered[j].updatedAt)
		})
		w.Header().Set("Total", strconv.Itoa(len(ordered)))

		start := (page - 1) * perPage
		end := start + perPage
		if start > len(ordered) {
			start = len(ordered)
		}
		if end > len(ordered) {
			end = len(ordered)
		}
		window := ordered[start:end]

		body := map[string]any{"calls": []any{}}
		items := make([]any, 0, len(window))
		for _, c := range window {
			items = append(items, map[string]any{
				"uuid":         c.uuid,
				"caller":       "+15555550100",
				"caller_state": "TX",
				"revenue":      "1.00",
				"payout":       "0.50",
				"created_at":   c.updatedAt.Format(time.RFC3339),
				"updated_at":   c.updatedAt.Format(time.RFC3339),
			})
		}
		body["calls"] = items

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)

		served++
		if mutate != nil {
			mutate(served, calls)
		}
	}))
}

// walkAllPages drives FetchPage until the adapter reports no more pages, collecting every
// external id it returned. Mirrors how the syncer consumes the adapter.
func walkAllPages(t *testing.T, a *RetreaverAdapter, creds map[string]string) []string {
	t.Helper()
	seen := []string{}
	cursor := ""
	for i := 0; i < 20; i++ { // hard cap so a bug cannot hang the suite
		res, err := a.FetchPage(context.Background(), creds, cursor, "", 30)
		if err != nil {
			t.Fatalf("FetchPage page %d: %v", i+1, err)
		}
		for _, c := range res.Calls {
			seen = append(seen, c.ExternalCallID)
		}
		if !res.HasMore {
			return seen
		}
		cursor = res.NextCursor
	}
	t.Fatalf("page walk did not terminate")
	return nil
}

// TestRetreaverFrozenWindowLosesNothing_K1 is the R0 reproduction, inverted by the R1 fix.
//
// Realistic model of the failure. `updated_at` is a last-modified stamp: it only moves forward,
// and an update during the walk sets it to *now*, which is at or past the window end captured
// when the walk began. So under R1 the updated row leaves the frozen window mid-walk — and the
// next window, which starts at `end − overlap`, must pick it up.
//
// Before R1 there was no end bound: the row jumped to position 1 of a live result set, shifted
// every later row down, and the page walk stepped straight over it. It was never returned again.
//
// The assertion that matters is therefore not "window 1 returned everything" but
// "no call is lost across consecutive windows".
func TestRetreaverFrozenWindowLosesNothing_K1(t *testing.T) {
	base := time.Now().UTC().Add(-2 * time.Hour)
	calls := make([]k1Call, 0, 101)
	for i := 0; i < 101; i++ {
		calls = append(calls, k1Call{
			uuid:      fmt.Sprintf("C%03d", i+1),
			updatedAt: base.Add(-time.Duration(i) * time.Minute),
		})
	}
	victim := "C101"

	srv := k1Server(t, calls, func(served int, cs []k1Call) {
		if served != 1 {
			return
		}
		// The buyer confirms C101 while page 1 is in flight: updated_at becomes the current
		// instant, which is after the end frozen at the start of this walk and before the end
		// of the next one.
		now := time.Now().UTC()
		for i := range cs {
			if cs[i].uuid == victim {
				cs[i].updatedAt = now
			}
		}
	})
	defer srv.Close()

	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	a := NewRetreaverAdapter(NewHTTPClient())
	creds := map[string]string{"api_key": "k", "company_id": "k1co"}

	// --- window 1 ---
	seen1, cursor := walkWindow(t, a, creds, "")

	// Window bounds are RFC3339 (whole seconds), so let a full second pass to guarantee
	// window 2's end is strictly after the moment C101 was updated.
	time.Sleep(1200 * time.Millisecond)

	// --- window 2: resumes from window 1's end minus the overlap ---
	seen2, _ := walkWindow(t, a, creds, cursor)

	all := map[string]int{}
	for _, id := range append(append([]string{}, seen1...), seen2...) {
		all[id]++
	}

	if all[victim] == 0 {
		t.Fatalf("K1 STILL PRESENT: %s lost across both windows (w1=%d w2=%d distinct=%d)",
			victim, len(seen1), len(seen2), len(all))
	}
	if len(all) != 101 {
		missing := []string{}
		for i := 0; i < 101; i++ {
			id := fmt.Sprintf("C%03d", i+1)
			if all[id] == 0 {
				missing = append(missing, id)
			}
		}
		t.Fatalf("expected all 101 calls across both windows, got %d; missing: %v", len(all), missing)
	}
	t.Logf("K1 FIXED [PROVEN]: all 101 calls delivered across 2 windows "+
		"(w1=%d rows, w2=%d rows); %s was updated mid-walk and collected by window 2",
		len(seen1), len(seen2), victim)
}

// walkWindow pages one complete window and returns the ids seen plus the cursor for the next one.
func walkWindow(t *testing.T, a *RetreaverAdapter, creds map[string]string, cursor string) ([]string, string) {
	t.Helper()
	seen := []string{}
	for i := 0; i < 30; i++ {
		res, err := a.FetchPage(context.Background(), creds, cursor, "", 30)
		if err != nil {
			t.Fatalf("FetchPage: %v", err)
		}
		for _, c := range res.Calls {
			seen = append(seen, c.ExternalCallID)
		}
		cursor = res.NextCursor
		if !res.HasMore {
			return seen, cursor
		}
	}
	t.Fatalf("window walk did not terminate")
	return nil, ""
}

// TestRetreaverWindowIsBounded is the R0 root-cause test, inverted by the R1 fix.
//
// Before R1 it asserted updated_at_end was ABSENT (the defect). It now asserts the window is
// bounded on both edges — the frozen window that makes the page walk stable.
func TestRetreaverWindowIsBounded(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"calls":[]}`))
	}))
	defer srv.Close()

	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	a := NewRetreaverAdapter(NewHTTPClient())
	if _, err := a.FetchPage(context.Background(), map[string]string{
		"api_key": "k", "company_id": "c",
	}, "", "", 30); err != nil {
		t.Fatalf("FetchPage: %v", err)
	}

	if got := gotQuery.Get("updated_at_start"); got == "" {
		t.Fatalf("expected updated_at_start to be sent")
	}
	end := gotQuery.Get("updated_at_end")
	if end == "" {
		t.Fatalf("updated_at_end must be sent — without it the window is live and K1 returns")
	}
	start := gotQuery.Get("updated_at_start")
	ts, err1 := time.Parse(time.RFC3339, start)
	te, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		t.Fatalf("window bounds must be RFC3339: %q .. %q", start, end)
	}
	if !ts.Before(te) {
		t.Fatalf("window must be non-empty: %q .. %q", start, end)
	}
	if got := gotQuery.Get("order"); got != "desc" {
		t.Fatalf("expected order=desc (asc is ignored by the API), got %q", got)
	}
	if got := gotQuery.Get("per_page"); got != "100" {
		t.Fatalf("per_page must be 100 (API max; above it the API silently returns 25), got %q", got)
	}
	t.Logf("frozen window [PROVEN]: [%s , %s) order=desc per_page=100", start, end)
}
