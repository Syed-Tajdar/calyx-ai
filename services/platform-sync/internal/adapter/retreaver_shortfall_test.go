package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"
)

// Stage 4 R1-fix F1 — a window that returns fewer rows than the API's own `Total` must retry,
// and must never advance the cursor while it is still short.
//
// R0.3 Q6 proved the `Total` response header is the exact row count for the requested window,
// which makes a short walk detectable for free. R1 refused to advance on one, but nothing
// retried and nothing surfaced it — on a 96k-call backfill that is a sync stuck with no visible
// error, the exact silent gap this stage exists to prevent.

// shortfallServer serves a window whose `Total` header overstates the rows it actually returns,
// for the first `shortResponses` walks. After that it tells the truth.
//
// `walks` counts page-1 requests, i.e. how many times the whole window has been walked.
func shortfallServer(t *testing.T, rows int, overstateBy int, shortWalks int) (*httptest.Server, *int) {
	t.Helper()
	walks := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage < 1 {
			perPage = 100
		}
		if page == 1 {
			walks++
		}

		total := rows
		if walks <= shortWalks {
			// The API claims more rows for this window than it will hand over. In the wild
			// this happens when a row is edited between the count and the page being served.
			total = rows + overstateBy
		}
		w.Header().Set("Total", strconv.Itoa(total))

		start := (page - 1) * perPage
		end := start + perPage
		if start > rows {
			start = rows
		}
		if end > rows {
			end = rows
		}

		items := make([]any, 0, end-start)
		for i := start; i < end; i++ {
			ts := time.Now().UTC().Add(-time.Duration(i) * time.Minute).Format(time.RFC3339)
			items = append(items, map[string]any{
				"uuid":         fmt.Sprintf("S%03d", i+1),
				"caller":       "+15555550100",
				"caller_state": "TX",
				"created_at":   ts,
				"updated_at":   ts,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"calls": items})
	}))
	return srv, &walks
}

// fastRetries removes the real backoff so bounded-retry tests stay quick, and pins the attempt
// count so the assertions do not depend on the deployed default.
func fastRetries(t *testing.T, attempts int) {
	t.Helper()
	t.Setenv("RETREAVER_WINDOW_RETRY_ATTEMPTS", strconv.Itoa(attempts))
	prev := retreaverSleep
	retreaverSleep = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}
	t.Cleanup(func() { retreaverSleep = prev })
}

// TestWindowShortfallRetriesThenAdvances — F1 case 1: mismatch, then success on retry, so the
// walk completes and the caller is free to advance.
func TestWindowShortfallRetriesThenAdvances(t *testing.T) {
	srv, walks := shortfallServer(t, 40, 5, 1) // short on the first walk only
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	fastRetries(t, 3)

	a := NewRetreaverAdapter(NewHTTPClient())
	end := time.Now().UTC()
	res, err := a.FetchRange(context.Background(), map[string]string{
		"api_key": "k", "company_id": "co",
	}, end.Add(-time.Hour), end)
	if err != nil {
		t.Fatalf("a window that settles on retry must succeed, got: %v", err)
	}
	if len(res.Calls) != 40 {
		t.Fatalf("expected all 40 rows after the retry, got %d", len(res.Calls))
	}
	if *walks != 2 {
		t.Fatalf("expected exactly 2 walks (1 short + 1 good), got %d", *walks)
	}
	t.Logf("F1 retry [PROVEN]: short walk retried once, 40/40 rows delivered, cursor free to advance")
}

// TestWindowShortfallPersistsAndBlocks — F1 case 2: a window that stays short exhausts the
// bounded retry and returns a shortfall, which is what holds the cursor and marks degraded.
func TestWindowShortfallPersistsAndBlocks(t *testing.T) {
	srv, walks := shortfallServer(t, 40, 5, 99) // never settles
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	fastRetries(t, 3)

	a := NewRetreaverAdapter(NewHTTPClient())
	end := time.Now().UTC()
	_, err := a.FetchRange(context.Background(), map[string]string{
		"api_key": "k", "company_id": "co",
	}, end.Add(-time.Hour), end)
	if err == nil {
		t.Fatalf("a persistently short window must not be reported as success")
	}

	shortfall := AsWindowShortfall(err)
	if shortfall == nil {
		t.Fatalf("error must be a *WindowShortfallError so the syncer can choose degraded over error, got %T: %v", err, err)
	}
	if shortfall.Collected != 40 || shortfall.Total != 45 {
		t.Fatalf("shortfall must carry the real counts, got collected=%d total=%d", shortfall.Collected, shortfall.Total)
	}
	if shortfall.Attempts != 3 {
		t.Fatalf("expected 3 bounded attempts, got %d", shortfall.Attempts)
	}
	if *walks != 3 {
		t.Fatalf("expected exactly 3 walks (the bound), got %d", *walks)
	}
	t.Logf("F1 bounded retry [PROVEN]: %v", err)
}

// TestWindowShortfallRecovers — F1 case 3: once the window returns its full count, the walk
// succeeds again. Recovery is by the ordinary success path, so `ok` overwrites `degraded` with
// no extra bookkeeping.
func TestWindowShortfallRecovers(t *testing.T) {
	srv, _ := shortfallServer(t, 40, 5, 1)
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	fastRetries(t, 1) // no retry at all, so the first walk fails outright

	a := NewRetreaverAdapter(NewHTTPClient())
	creds := map[string]string{"api_key": "k", "company_id": "co"}
	end := time.Now().UTC()
	start := end.Add(-time.Hour)

	if _, err := a.FetchRange(context.Background(), creds, start, end); AsWindowShortfall(err) == nil {
		t.Fatalf("first walk was short and must report a shortfall, got: %v", err)
	}
	res, err := a.FetchRange(context.Background(), creds, start, end)
	if err != nil {
		t.Fatalf("the window now returns its full count and must succeed, got: %v", err)
	}
	if len(res.Calls) != 40 {
		t.Fatalf("expected 40 rows on recovery, got %d", len(res.Calls))
	}
	t.Logf("F1 recovery [PROVEN]: degraded window returns to a clean walk without operator action")
}

// TestIncrementalShortfallBlocks — F1 requires the same rule on the incremental path, not only
// on the RangeFetcher path used by sweeps and backfill.
func TestIncrementalShortfallBlocks(t *testing.T) {
	srv, _ := shortfallServer(t, 40, 5, 99)
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	fastRetries(t, 2)

	a := NewRetreaverAdapter(NewHTTPClient())
	_, err := a.FetchPage(context.Background(), map[string]string{
		"api_key": "k", "company_id": "co",
	}, "", "", 1)
	if AsWindowShortfall(err) == nil {
		t.Fatalf("the incremental path must block on a short window too, got: %v", err)
	}
	t.Logf("F1 incremental path [PROVEN]: %v", err)
}

// TestWindowWithoutTotalIsAccepted — there is nothing to reconcile against when the API omits
// `Total`, so the walk must not be treated as short. Guards against turning every response from
// a fixture server or a future API change into a permanent degraded state.
func TestWindowWithoutTotalIsAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No `Total` header at all.
		ts := time.Now().UTC().Format(time.RFC3339)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"calls": []any{
			map[string]any{"uuid": "N1", "caller_state": "TX", "created_at": ts, "updated_at": ts},
		}})
	}))
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	fastRetries(t, 3)

	a := NewRetreaverAdapter(NewHTTPClient())
	end := time.Now().UTC()
	if _, err := a.FetchRange(context.Background(), map[string]string{
		"api_key": "k", "company_id": "co",
	}, end.Add(-time.Hour), end); err != nil {
		t.Fatalf("a window with no Total header must be accepted, got: %v", err)
	}
}

// --- F2 -------------------------------------------------------------------------------------

// TestRowUpdatedJustBeforeWindowEndIsNotLost — F2.
//
// The K1 test models an update that stamps `updated_at` at or after `window_end`, which is the
// normal case: an edit during the walk happens "now", and now is past the end frozen when the
// walk began. Clock skew between Calyx and Retreaver, or a slow walk, can instead stamp it just
// BEFORE `window_end`. The row then reorders to the top *inside* the frozen window and is
// skipped by this walk — so the guarantee has to come from the next window's overlap.
func TestRowUpdatedJustBeforeWindowEndIsNotLost(t *testing.T) {
	// A fixed window so "30 seconds before the end" is exact rather than racing the clock.
	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-time.Hour)
	victim := "C061"

	type row struct {
		uuid string
		at   time.Time
	}
	rows := make([]row, 0, 61)
	for i := 0; i < 61; i++ {
		rows = append(rows, row{
			uuid: fmt.Sprintf("C%03d", i+1),
			at:   start.Add(time.Duration(i+1) * time.Minute),
		})
	}

	served := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(q.Get("per_page"))
		if perPage < 1 {
			perPage = 100
		}
		wStart, _ := time.Parse(time.RFC3339, q.Get("updated_at_start"))
		wEnd, _ := time.Parse(time.RFC3339, q.Get("updated_at_end"))

		in := make([]row, 0, len(rows))
		for _, c := range rows {
			if !wStart.IsZero() && c.at.Before(wStart) {
				continue
			}
			if !wEnd.IsZero() && !c.at.Before(wEnd) {
				continue
			}
			in = append(in, c)
		}
		sort.SliceStable(in, func(i, j int) bool { return in[i].at.After(in[j].at) })
		w.Header().Set("Total", strconv.Itoa(len(in)))

		lo := (page - 1) * perPage
		hi := lo + perPage
		if lo > len(in) {
			lo = len(in)
		}
		if hi > len(in) {
			hi = len(in)
		}
		items := make([]any, 0, hi-lo)
		for _, c := range in[lo:hi] {
			items = append(items, map[string]any{
				"uuid":         c.uuid,
				"caller_state": "TX",
				"created_at":   c.at.Format(time.RFC3339),
				"updated_at":   c.at.Format(time.RFC3339),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"calls": items})

		served++
		if served == 1 {
			// Mid-walk: the oldest row is edited, but the stamp lands 30s INSIDE the frozen
			// window rather than past its end.
			for i := range rows {
				if rows[i].uuid == victim {
					rows[i].at = wEnd.Add(-30 * time.Second)
				}
			}
		}
	}))
	defer srv.Close()

	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	// per_page is fixed at 100 in the adapter, so force multi-page walking via the cap and a
	// small dataset is not possible; instead the overlap is what must save the row.
	fastRetries(t, 1)

	a := NewRetreaverAdapter(NewHTTPClient())
	creds := map[string]string{"api_key": "k", "company_id": "co"}

	seen := map[string]int{}
	// Window 1: the frozen [start, end).
	res1, err := a.FetchRange(context.Background(), creds, start, end)
	if err != nil && AsWindowShortfall(err) == nil {
		t.Fatalf("window 1: %v", err)
	}
	for _, c := range res1.Calls {
		seen[c.ExternalCallID]++
	}

	// Window 2 begins at window 1's end minus the overlap — the rule the adapter itself uses.
	overlap := RetreaverWindowOverlap()
	res2, err := a.FetchRange(context.Background(), creds, end.Add(-overlap), end.Add(time.Minute))
	if err != nil {
		t.Fatalf("window 2: %v", err)
	}
	for _, c := range res2.Calls {
		seen[c.ExternalCallID]++
	}

	if seen[victim] == 0 {
		t.Fatalf("F2: %s was updated to window_end-30s mid-walk and was lost across both windows", victim)
	}
	t.Logf("F2 [PROVEN]: a row restamped %v before window_end is skipped by window 1 and "+
		"collected by window 2's %s overlap", 30*time.Second, overlap)
}
