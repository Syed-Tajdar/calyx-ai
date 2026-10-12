package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Stage 4b — Retreaver write-back transport against a fake Retreaver API. No network.

const (
	wbTestKey     = "SECRETKEY-wb-7c1f9e"
	wbTestCompany = "48213"
	wbTestUUID    = "addcf985-017e-4962-be34-cf5d55e74afc"
)

func wbCreds() map[string]string {
	return map[string]string{"api_key": wbTestKey, "company_id": wbTestCompany}
}

type recordedReq struct {
	Method      string
	Path        string
	Query       map[string]string
	ContentType string
	Body        string
}

type fakeRetreaver struct {
	mu   sync.Mutex
	reqs []recordedReq
	// respond returns status, body and headers for the n-th request (0-based).
	respond func(n int, r recordedReq) (int, string, map[string]string)
}

func (f *fakeRetreaver) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	q := map[string]string{}
	for k, v := range r.URL.Query() {
		q[k] = v[0]
	}
	rec := recordedReq{Method: r.Method, Path: r.URL.Path, Query: q, ContentType: r.Header.Get("Content-Type"), Body: string(b)}
	f.mu.Lock()
	n := len(f.reqs)
	f.reqs = append(f.reqs, rec)
	f.mu.Unlock()
	status, body, hdr := f.respond(n, rec)
	for k, v := range hdr {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (f *fakeRetreaver) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func newWBClient(t *testing.T, f *fakeRetreaver) (*RetreaverCallClient, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	var sleeps []time.Duration
	c := NewRetreaverCallClientWith(RetreaverCallClientOptions{
		Origin: srv.URL,
		Sleep: func(_ context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		},
	})
	return c, &sleeps
}

func assertNoSecret(t *testing.T, where, text string) {
	t.Helper()
	if strings.Contains(text, wbTestKey) {
		t.Fatalf("%s leaks the api_key: %s", where, text)
	}
	if strings.Contains(text, "company_id="+wbTestCompany) {
		t.Fatalf("%s leaks the company_id: %s", where, text)
	}
}

const callJSON = `{"call":{"uuid":"addcf985-017e-4962-be34-cf5d55e74afc","company_id":48213,
 "revenue":%s,"payout":null,"converted":%s,"receivable":%s,"payable":false,
 "conversions_determined_at":"2026-10-12T10:00:00.000Z","fired_pixels_count":0,
 "postback_value":null,"updated_at":"2026-10-12T10:00:01.000Z","system_campaign_id":9872}}`

func makeCall(revenue, converted, receivable string) string {
	out := callJSON
	out = strings.Replace(out, "%s", revenue, 1)
	out = strings.Replace(out, "%s", converted, 1)
	out = strings.Replace(out, "%s", receivable, 1)
	return out
}

func TestRetreaverSetRevenuePutsOnlyRevenueToV5(t *testing.T) {
	f := &fakeRetreaver{respond: func(int, recordedReq) (int, string, map[string]string) {
		return 200, makeCall("1.0", "true", "true"), nil
	}}
	c, _ := newWBClient(t, f)
	if err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 1); err != nil {
		t.Fatal(err)
	}
	if f.count() != 1 {
		t.Fatalf("requests=%d", f.count())
	}
	r := f.reqs[0]
	if r.Method != http.MethodPut || r.Path != "/api/v5/calls/"+wbTestUUID+".json" {
		t.Fatalf("%s %s", r.Method, r.Path)
	}
	if r.Query["api_key"] != wbTestKey || r.Query["company_id"] != wbTestCompany {
		t.Fatalf("query %v", r.Query)
	}
	if r.ContentType != "application/json" {
		t.Fatalf("content-type %q", r.ContentType)
	}
	if r.Body != `{"call":{"revenue":1}}` {
		t.Fatalf("body %s", r.Body)
	}
	var parsed map[string]map[string]any
	if err := json.Unmarshal([]byte(r.Body), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || len(parsed["call"]) != 1 {
		t.Fatalf("body must carry call.revenue only: %s", r.Body)
	}
	for _, banned := range []string{"payout", "session_notes", "receivable", "payable"} {
		if _, ok := parsed["call"][banned]; ok {
			t.Fatalf("body sends %s: %s", banned, r.Body)
		}
	}
}

func TestRetreaverVoidSendsRevenueZero(t *testing.T) {
	b, err := RetreaverRevenueBody(0)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"call":{"revenue":0}}` {
		t.Fatalf("void body %s — revenue 0 must be sent, not omitted", b)
	}
	b, _ = RetreaverRevenueBody(18.4)
	if string(b) != `{"call":{"revenue":18.4}}` {
		t.Fatalf("body %s", b)
	}
}

func TestRetreaverGetCallReadsV2AndParsesReadBack(t *testing.T) {
	f := &fakeRetreaver{respond: func(int, recordedReq) (int, string, map[string]string) {
		return 200, makeCall(`"1.00"`, "true", "true"), nil
	}}
	c, _ := newWBClient(t, f)
	rb, err := c.GetCall(context.Background(), wbCreds(), wbTestUUID)
	if err != nil {
		t.Fatal(err)
	}
	r := f.reqs[0]
	if r.Method != http.MethodGet || r.Path != "/api/v2/calls/"+wbTestUUID+".json" || r.Body != "" {
		t.Fatalf("%s %s body=%q", r.Method, r.Path, r.Body)
	}
	if rb.Revenue == nil || *rb.Revenue != 1 || rb.Converted == nil || !*rb.Converted {
		t.Fatalf("%+v", rb)
	}
	if rb.Payout != nil {
		t.Fatalf("payout null must stay nil, got %v", *rb.Payout)
	}
	if rb.FiredPixelsCount == nil || *rb.FiredPixelsCount != 0 || rb.PostbackValue != nil {
		t.Fatalf("pixels/postback %+v", rb)
	}
	if rb.CompanyID != wbTestCompany || rb.UpdatedAt == nil || rb.ConversionsDeterminedAt == nil {
		t.Fatalf("%+v", rb)
	}
	if rb.Call.ExternalCallID != wbTestUUID || rb.Call.CampaignID != "9872" || rb.Call.Revenue != 1 || !rb.Call.HasConverted {
		t.Fatalf("mapped %+v", rb.Call)
	}
}

func TestRetreaverReadBackDoesNotMoveSyncC3Counters(t *testing.T) {
	ResetRetreaverC3()
	f := &fakeRetreaver{respond: func(int, recordedReq) (int, string, map[string]string) {
		// converted=true with revenue 0 would count as ConvertedZeroRevenue during a sync.
		return 200, makeCall("0", "true", "false"), nil
	}}
	c, _ := newWBClient(t, f)
	if _, err := c.GetCall(context.Background(), wbCreds(), wbTestUUID); err != nil {
		t.Fatal(err)
	}
	if checked, _, _, zero := RetreaverC3Snapshot(); checked != 0 || zero != 0 {
		t.Fatalf("read-back moved C3 counters checked=%d zero=%d", checked, zero)
	}
}

func TestRetreaver429ThenSuccess(t *testing.T) {
	f := &fakeRetreaver{respond: func(n int, _ recordedReq) (int, string, map[string]string) {
		if n == 0 {
			return 429, `{"error":"slow down"}`, map[string]string{"Retry-After": "3"}
		}
		return 200, makeCall("1.0", "true", "true"), nil
	}}
	c, sleeps := newWBClient(t, f)
	if err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 1); err != nil {
		t.Fatal(err)
	}
	if f.count() != 2 {
		t.Fatalf("requests=%d", f.count())
	}
	if len(*sleeps) != 1 || (*sleeps)[0] != 3*time.Second {
		t.Fatalf("Retry-After not honoured: %v", *sleeps)
	}
	if f.reqs[1].Body != `{"call":{"revenue":1}}` {
		t.Fatalf("retry must resend the same body, got %q", f.reqs[1].Body)
	}
}

func TestRetreaver5xxRetriedWithBackoffThenSuccess(t *testing.T) {
	f := &fakeRetreaver{respond: func(n int, _ recordedReq) (int, string, map[string]string) {
		if n < 2 {
			return 503, "maintenance", nil
		}
		return 200, makeCall("0", "false", "false"), nil
	}}
	c, sleeps := newWBClient(t, f)
	if err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 0); err != nil {
		t.Fatal(err)
	}
	if f.count() != 3 || len(*sleeps) != 2 || (*sleeps)[1] <= (*sleeps)[0] {
		t.Fatalf("requests=%d sleeps=%v", f.count(), *sleeps)
	}
}

func TestRetreaver4xxNeverRetried(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 405, 406, 410, 422} {
		f := &fakeRetreaver{respond: func(int, recordedReq) (int, string, map[string]string) {
			return status, `{"error":"no"}`, nil
		}}
		c, sleeps := newWBClient(t, f)
		err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 1)
		if err == nil {
			t.Fatalf("%d: want error", status)
		}
		if f.count() != 1 || len(*sleeps) != 0 {
			t.Fatalf("%d retried: requests=%d sleeps=%v", status, f.count(), *sleeps)
		}
		if status == 404 && !strings.Contains(err.Error(), "not found") {
			t.Fatalf("404 must read as not found: %v", err)
		}
		assertNoSecret(t, "4xx error", err.Error())
	}
}

// Rule 1: a failed PUT (4xx, 5xx, network error) never carries the key — the upstream body
// echoing the request URL included.
func TestRetreaverFailedPutNeverLeaksKey(t *testing.T) {
	echo := func(_ int, r recordedReq) string {
		return `{"error":"bad request for https://api.retreaver.com/api/v5/calls/` + wbTestUUID +
			`.json?api_key=` + r.Query["api_key"] + `&company_id=` + r.Query["company_id"] + `"}`
	}
	cases := map[string]func(n int, r recordedReq) (int, string, map[string]string){
		"400 echo": func(n int, r recordedReq) (int, string, map[string]string) { return 400, echo(n, r), nil },
		"401 echo": func(n int, r recordedReq) (int, string, map[string]string) { return 401, echo(n, r), nil },
		"422 echo": func(n int, r recordedReq) (int, string, map[string]string) { return 422, echo(n, r), nil },
		"500 x3 echo": func(n int, r recordedReq) (int, string, map[string]string) {
			return 500, echo(n, r), map[string]string{"Location": "https://api.retreaver.com/x?api_key=" + r.Query["api_key"]}
		},
		"429 x3": func(n int, r recordedReq) (int, string, map[string]string) { return 429, echo(n, r), nil },
	}
	for name, respond := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeRetreaver{respond: respond}
			c, _ := newWBClient(t, f)
			err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 1)
			if err == nil {
				t.Fatal("want error")
			}
			assertNoSecret(t, name, err.Error())
			if !strings.Contains(err.Error(), "[REDACTED]") {
				t.Fatalf("expected a redacted URL in %q", err.Error())
			}
		})
	}

	t.Run("network error", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		origin := srv.URL
		srv.Close() // connection refused; the transport error quotes the full URL
		c := NewRetreaverCallClientWith(RetreaverCallClientOptions{Origin: origin})
		err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 1)
		if err == nil {
			t.Fatal("want error")
		}
		assertNoSecret(t, "network error", err.Error())
		if !strings.Contains(err.Error(), "api_key=[REDACTED]") {
			t.Fatalf("expected the URL to survive redacted: %q", err.Error())
		}
		_, gerr := c.GetCall(context.Background(), wbCreds(), wbTestUUID)
		if gerr == nil {
			t.Fatal("want GET error")
		}
		assertNoSecret(t, "GET network error", gerr.Error())
	})
}

func TestRetreaverNetworkErrorNotRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	origin := srv.URL
	srv.Close()
	var sleeps int
	c := NewRetreaverCallClientWith(RetreaverCallClientOptions{Origin: origin, Sleep: func(context.Context, time.Duration) error {
		sleeps++
		return nil
	}})
	if err := c.SetRevenue(context.Background(), wbCreds(), wbTestUUID, 1); err == nil {
		t.Fatal("want error")
	}
	if sleeps != 0 {
		t.Fatalf("a transport error on PUT must not be retried (the read-back decides), sleeps=%d", sleeps)
	}
}

func TestRetreaverInvalidUUIDSendsNothing(t *testing.T) {
	f := &fakeRetreaver{respond: func(int, recordedReq) (int, string, map[string]string) { return 200, "{}", nil }}
	c, _ := newWBClient(t, f)
	for _, id := range []string{"", "RGBTEST01", "../../companies/1", wbTestUUID + "?x=1", "addcf985-017e-4962-be34-cf5d55e74afcX"} {
		if err := c.SetRevenue(context.Background(), wbCreds(), id, 1); err == nil {
			t.Fatalf("%q accepted", id)
		}
		if _, err := c.GetCall(context.Background(), wbCreds(), id); err == nil {
			t.Fatalf("%q accepted by GET", id)
		}
	}
	if f.count() != 0 {
		t.Fatalf("requests=%d", f.count())
	}
}

func TestRetreaverMissingCredsSendsNothing(t *testing.T) {
	f := &fakeRetreaver{respond: func(int, recordedReq) (int, string, map[string]string) { return 200, "{}", nil }}
	c, _ := newWBClient(t, f)
	if err := c.SetRevenue(context.Background(), map[string]string{"api_key": wbTestKey}, wbTestUUID, 1); err == nil {
		t.Fatal("missing company_id accepted")
	}
	if f.count() != 0 {
		t.Fatalf("requests=%d", f.count())
	}
}

// Write-back uses the same per-company limiter instance as Retreaver sync, so the two share
// the R1 budget (2/s, 60/min) instead of each having its own.
func TestRetreaverWritebackSharesSyncLimiter(t *testing.T) {
	c := NewRetreaverCallClient()
	if c.limiters != retreaverLimiters {
		t.Fatal("write-back must use the shared retreaverLimiters")
	}
	if c.limiters.ForAccount("company-shared-check") != retreaverLimiters.ForAccount("company-shared-check") {
		t.Fatal("same company_id must map to one limiter")
	}
	lim := NewDualWindowLimiter(RetreaverRatePerSec(), RetreaverRatePerMin())
	if lim.perSec != 2 || lim.perMin != 60 {
		t.Fatalf("defaults %d/s %d/min", lim.perSec, lim.perMin)
	}
}
