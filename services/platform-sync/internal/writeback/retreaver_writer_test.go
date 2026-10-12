package writeback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Stage 4b — Retreaver write-back end to end through the Service, the Retreaver writer and the
// real HTTP transport, against a stateful fake Retreaver API. No network, no credentials.

const (
	rtKey     = "SECRETKEY-svc-91ab3d"
	rtCompany = "48213"
	rtUUID    = "addcf985-017e-4962-be34-cf5d55e74afc"
)

type rtDec struct{}

func (rtDec) DecryptMap([]byte) (map[string]string, error) {
	return map[string]string{"api_key": rtKey, "company_id": rtCompany}, nil
}

// fakeRetreaverAPI keeps one call's state and applies PUTs the way the docs describe.
type fakeRetreaverAPI struct {
	mu        sync.Mutex
	revenue   *float64
	payout    *float64
	converted bool
	pixels    int
	company   string
	updated   int

	puts []string // PUT bodies
	gets int32

	// Behaviour switches.
	voidLeavesZero  bool                  // void stores 0 instead of null
	ignorePut       bool                  // accept the PUT but change nothing (read-back not yet matching)
	applyAs         func(float64) float64 // store a different amount (contradicting read-back)
	putStatuses     []int                 // statuses for successive PUTs before success
	putBody         string                // body for failing PUTs
	getStatus       int                   // non-zero: every GET answers this
	firePixelOnConv bool                  // simulate a campaign pixel firing on a new conversion
}

func newFakeRetreaverAPI() *fakeRetreaverAPI {
	return &fakeRetreaverAPI{company: rtCompany}
}

func f64(v float64) *float64 { return &v }

func (f *fakeRetreaverAPI) callJSON() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	rev := "null"
	if f.revenue != nil {
		rev = fmt.Sprintf("%v", *f.revenue)
	}
	pay := "null"
	if f.payout != nil {
		pay = fmt.Sprintf("%v", *f.payout)
	}
	receivable := f.revenue != nil && *f.revenue >= 0.01
	return fmt.Sprintf(`{"call":{"uuid":%q,"company_id":%s,"revenue":%s,"payout":%s,"converted":%t,
		"receivable":%t,"payable":false,"conversions_determined_at":"2026-10-12T10:00:00.000Z",
		"fired_pixels_count":%d,"postback_value":null,"updated_at":"2026-10-12T10:00:%02d.000Z",
		"system_campaign_id":9872}}`, rtUUID, f.company, rev, pay, f.converted, receivable, f.pixels, f.updated)
}

func (f *fakeRetreaverAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("api_key") != rtKey || r.URL.Query().Get("company_id") != rtCompany {
		w.WriteHeader(401)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v2/calls/"+rtUUID+".json":
		atomic.AddInt32(&f.gets, 1)
		if f.getStatus != 0 {
			w.WriteHeader(f.getStatus)
			return
		}
		_, _ = io.WriteString(w, f.callJSON())
	case r.Method == http.MethodPut && r.URL.Path == "/api/v5/calls/"+rtUUID+".json":
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.puts = append(f.puts, string(b))
		n := len(f.puts)
		var status int
		if n <= len(f.putStatuses) {
			status = f.putStatuses[n-1]
		}
		f.mu.Unlock()
		if status != 0 {
			if status == 429 {
				w.Header().Set("Retry-After", "0")
			}
			w.WriteHeader(status)
			_, _ = io.WriteString(w, strings.ReplaceAll(f.putBody, "{URL}", "https://api.retreaver.com"+r.URL.String()))
			return
		}
		var body struct {
			Call map[string]json.RawMessage `json:"call"`
		}
		_ = json.Unmarshal(b, &body)
		var amt float64
		_ = json.Unmarshal(body.Call["revenue"], &amt)
		f.mu.Lock()
		if !f.ignorePut {
			if f.applyAs != nil {
				amt = f.applyAs(amt)
			}
			if amt >= 0.01 {
				if !f.converted && f.firePixelOnConv {
					f.pixels++
				}
				f.revenue = f64(amt)
				f.converted = true
			} else if f.voidLeavesZero {
				f.revenue = f64(0)
				f.converted = false
			} else {
				f.revenue = nil
				f.converted = false
			}
			f.updated++
		}
		f.mu.Unlock()
		_, _ = io.WriteString(w, f.callJSON())
	default:
		w.WriteHeader(404)
	}
}

func (f *fakeRetreaverAPI) putCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.puts)
}

func retreaverItem() *models.SyncIntegration {
	return &models.SyncIntegration{
		ID: "int-r", TenantID: "ten-a", Platform: "retreaver",
		SyncMode: "real", WritebackEnabled: true, APICredentialsBlob: []byte("x"),
	}
}

func rtReq() ConvertRequest {
	return ConvertRequest{TenantID: "ten-a", UserID: "user-1", IntegrationID: "int-r", InboundCallID: rtUUID, Amount: 1}
}

func fastLimiters(t *testing.T) *adapter.RetreaverAccountLimiters {
	t.Setenv("RETREAVER_RATE_PER_SEC", "1000")
	t.Setenv("RETREAVER_RATE_PER_MIN", "100000")
	return adapter.NewRetreaverAccountLimiters()
}

// rtSvc wires a Service with Ringba (fake) and Retreaver (real transport → fake API).
func rtSvc(t *testing.T, api http.Handler, item *models.SyncIntegration, lock Locker, audit *memAudit, client *http.Client) *Service {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	tc := adapter.NewRetreaverCallClientWith(adapter.RetreaverCallClientOptions{
		Origin:     srv.URL,
		HTTPClient: client,
		Limiters:   fastLimiters(t),
		Sleep:      func(context.Context, time.Duration) error { return nil },
	})
	s := NewService(&fakeStore{item: item}, audit, rtDec{}, &fakeRingba{}, lock)
	s.settleEvery = 5 * time.Millisecond
	s.Register(NewRetreaverWriter(tc, 60*time.Millisecond))
	return s
}

func noSecret(t *testing.T, where, text string) {
	t.Helper()
	if strings.Contains(text, rtKey) {
		t.Fatalf("%s leaks the api_key: %s", where, text)
	}
}

func auditNoSecrets(t *testing.T, audit *memAudit) {
	t.Helper()
	for i, r := range audit.rows {
		noSecret(t, fmt.Sprintf("audit row %d message", i), r.Message)
		noSecret(t, fmt.Sprintf("audit row %d read_back", i), r.ReadBack)
	}
}

func assertOnlyRevenueBodies(t *testing.T, f *fakeRetreaverAPI) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.puts {
		var m map[string]map[string]any
		if err := json.Unmarshal([]byte(b), &m); err != nil {
			t.Fatalf("body %s: %v", b, err)
		}
		if len(m) != 1 || len(m["call"]) != 1 {
			t.Fatalf("PUT body must be {\"call\":{\"revenue\":x}} only, got %s", b)
		}
		if _, ok := m["call"]["revenue"]; !ok {
			t.Fatalf("no revenue in %s", b)
		}
	}
}

func decodeReadBack(t *testing.T, raw string) ReadBackSummary {
	t.Helper()
	var s ReadBackSummary
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("read_back %q: %v", raw, err)
	}
	return s
}

func TestRetreaverConvertConfirmed(t *testing.T) {
	api := newFakeRetreaverAPI()
	audit := &memAudit{}
	res, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" || res.Platform != "retreaver" || !res.HasConverted || res.ConversionAmount != 1 {
		t.Fatalf("%+v", res)
	}
	if api.putCount() != 1 || api.puts[0] != `{"call":{"revenue":1}}` {
		t.Fatalf("puts %v", api.puts)
	}
	if len(audit.rows) != 1 {
		t.Fatalf("rows %+v", audit.rows)
	}
	row := audit.rows[0]
	if row.Outcome != "confirmed" || row.Platform != "retreaver" || row.Action != "convert" || row.Amount != 1 || row.CampaignID != "9872" {
		t.Fatalf("row %+v", row)
	}
	rb := decodeReadBack(t, row.ReadBack)
	if rb.Before == nil || rb.After == nil || rb.Before.Revenue != nil || rb.After.Revenue == nil || *rb.After.Revenue != 1 {
		t.Fatalf("read_back %s", row.ReadBack)
	}
	if rb.After.Converted == nil || !*rb.After.Converted || rb.After.FiredPixelsCount == nil || rb.After.UpdatedAt == nil ||
		rb.After.ConversionsDeterminedAt == nil || rb.After.Receivable == nil || rb.After.Payable == nil {
		t.Fatalf("read-back fields missing: %s", row.ReadBack)
	}
	if rb.PayoutChanged || rb.FiredPixelsDelta == nil || *rb.FiredPixelsDelta != 0 {
		t.Fatalf("payout/pixels %s", row.ReadBack)
	}
	auditNoSecrets(t, audit)
}

func TestRetreaverVoidConfirmedNullAndZero(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprintf("voidLeavesZero=%t", zero), func(t *testing.T) {
			api := newFakeRetreaverAPI()
			api.revenue, api.converted, api.voidLeavesZero = f64(1), true, zero
			audit := &memAudit{}
			res, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Void(context.Background(), rtReq())
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != "confirmed" || res.HasConverted || res.ConversionAmount != 0 || !res.VoidReversible || res.Amount != 1 {
				t.Fatalf("%+v", res)
			}
			if api.putCount() != 1 || api.puts[0] != `{"call":{"revenue":0}}` {
				t.Fatalf("void body %v", api.puts)
			}
			if audit.rows[0].Outcome != "confirmed" || audit.rows[0].Action != "void" || audit.rows[0].Platform != "retreaver" {
				t.Fatalf("row %+v", audit.rows[0])
			}
		})
	}
}

func TestRetreaverReconvertAfterVoid(t *testing.T) {
	api := newFakeRetreaverAPI()
	audit := &memAudit{}
	svc := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil)
	ctx := context.Background()
	if _, err := svc.Convert(ctx, rtReq()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Void(ctx, rtReq()); err != nil {
		t.Fatal(err)
	}
	req := rtReq()
	req.Amount = 0.75
	res, err := svc.Convert(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" || res.ConversionAmount != 0.75 {
		t.Fatalf("%+v", res)
	}
	want := []string{`{"call":{"revenue":1}}`, `{"call":{"revenue":0}}`, `{"call":{"revenue":0.75}}`}
	if strings.Join(api.puts, "|") != strings.Join(want, "|") {
		t.Fatalf("puts %v", api.puts)
	}
	if len(audit.rows) != 3 {
		t.Fatalf("rows %d", len(audit.rows))
	}
	for _, r := range audit.rows {
		if r.Outcome != "confirmed" {
			t.Fatalf("row %+v", r)
		}
	}
	assertOnlyRevenueBodies(t, api)
}

func TestRetreaverConvertRefusedWhenConverted(t *testing.T) {
	api := newFakeRetreaverAPI()
	api.revenue, api.converted = f64(5), true
	audit := &memAudit{}
	_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
	if !errors.Is(err, ErrAlreadyConverted) || HTTPStatus(err) != 409 {
		t.Fatalf("err=%v", err)
	}
	if api.putCount() != 0 || len(audit.rows) != 1 || audit.rows[0].Outcome != "failed" {
		t.Fatalf("puts=%d rows=%+v", api.putCount(), audit.rows)
	}
}

func TestRetreaverVoidRefusedWhenNotConverted(t *testing.T) {
	api := newFakeRetreaverAPI()
	audit := &memAudit{}
	_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Void(context.Background(), rtReq())
	if !errors.Is(err, ErrNotConverted) || HTTPStatus(err) != 409 {
		t.Fatalf("err=%v", err)
	}
	if api.putCount() != 0 || len(audit.rows) != 1 {
		t.Fatalf("puts=%d rows=%d", api.putCount(), len(audit.rows))
	}
}

// Owner decision (2026-10-12): the PUT succeeded but the read-back has not matched within the
// settle window → pending / unconfirmed, exactly like Ringba. The lock is held as "pending" so a
// second click cannot re-send while the first is unconfirmed.
func TestRetreaverUnsettledReadBackIsPendingNotFailed(t *testing.T) {
	cases := map[string]struct {
		setup func(*fakeRetreaverAPI)
		void  bool
	}{
		"convert: PUT accepted, read-back still old": {setup: func(f *fakeRetreaverAPI) { f.ignorePut = true }},
		"void: PUT accepted, read-back still converted": {setup: func(f *fakeRetreaverAPI) {
			f.revenue, f.converted, f.ignorePut = f64(1), true, true
		}, void: true},
		"convert: PUT accepted, every read-back GET fails": {setup: nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			api := newFakeRetreaverAPI()
			var handler http.Handler = api
			if c.setup != nil {
				c.setup(api)
			} else {
				// First GET (lookup) succeeds; every read-back GET after the PUT answers 503.
				handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet && api.putCount() > 0 {
						w.WriteHeader(503)
						return
					}
					api.ServeHTTP(w, r)
				})
			}
			audit := &memAudit{}
			lock := NewMemoryLocker()
			svc := rtSvc(t, handler, retreaverItem(), lock, audit, nil)
			action := "convert"
			var res Result
			var err error
			if c.void {
				action = "void"
				res, err = svc.Void(context.Background(), rtReq())
			} else {
				res, err = svc.Convert(context.Background(), rtReq())
			}
			if err != nil {
				t.Fatalf("pending must not be an error: %v", err)
			}
			if res.Outcome != "pending" || !strings.Contains(res.Message, "unconfirmed") {
				t.Fatalf("%+v", res)
			}
			if api.putCount() != 1 {
				t.Fatalf("puts=%d", api.putCount())
			}
			last := audit.rows[len(audit.rows)-1]
			if last.Outcome != "pending" || last.Platform != "retreaver" || last.Action != action {
				t.Fatalf("row %+v", last)
			}
			if ok, _ := lock.Acquire(context.Background(), "ten-a", action, rtUUID); ok {
				t.Fatal("pending must keep the lock (MarkPending), like Ringba")
			}
		})
	}
}

// A read-back that shows a value which is neither what was written nor what the call had
// before is a contradiction → audited failure (502), lock released so the operator can act.
func TestRetreaverContradictingReadBackIsAuditedFailure(t *testing.T) {
	cases := map[string]struct {
		setup func(*fakeRetreaverAPI)
		void  bool
	}{
		"convert $1 stored as $2": {setup: func(f *fakeRetreaverAPI) { f.applyAs = func(v float64) float64 { return v * 2 } }},
		"void stored as $0.50": {setup: func(f *fakeRetreaverAPI) {
			f.revenue, f.converted = f64(1), true
			f.applyAs = func(float64) float64 { return 0.5 }
		}, void: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			api := newFakeRetreaverAPI()
			c.setup(api)
			audit := &memAudit{}
			lock := NewMemoryLocker()
			svc := rtSvc(t, api, retreaverItem(), lock, audit, nil)
			action := "convert"
			var res Result
			var err error
			if c.void {
				action = "void"
				res, err = svc.Void(context.Background(), rtReq())
			} else {
				res, err = svc.Convert(context.Background(), rtReq())
			}
			if !errors.Is(err, ErrReadBackMismatch) || HTTPStatus(err) != 502 {
				t.Fatalf("err=%v", err)
			}
			if res.Outcome != "failed" || !strings.Contains(res.Message, "read-back contradicts the write") {
				t.Fatalf("%+v", res)
			}
			last := audit.rows[len(audit.rows)-1]
			if last.Outcome != "failed" || !strings.Contains(last.Message, "contradicts") || last.ReadBack == "" {
				t.Fatalf("row %+v", last)
			}
			if ok, _ := lock.Acquire(context.Background(), "ten-a", action, rtUUID); !ok {
				t.Fatal("lock must be released after a contradiction")
			}
		})
	}
}

func TestRetreaver429ThenSuccessThroughService(t *testing.T) {
	api := newFakeRetreaverAPI()
	api.putStatuses = []int{429}
	audit := &memAudit{}
	res, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
	if err != nil || res.Outcome != "confirmed" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if api.putCount() != 2 {
		t.Fatalf("puts=%d", api.putCount())
	}
}

func TestRetreaver4xxPutNotRetriedAndAudited(t *testing.T) {
	api := newFakeRetreaverAPI()
	api.putStatuses = []int{422, 422, 422}
	api.putBody = `{"errors":{"revenue":["is invalid"]}}`
	audit := &memAudit{}
	_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
	if !errors.Is(err, ErrFireRejected) || HTTPStatus(err) != 502 {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "Retreaver rejected the write") {
		t.Fatalf("message should name Retreaver: %v", err)
	}
	if api.putCount() != 1 {
		t.Fatalf("4xx retried: puts=%d", api.putCount())
	}
	if len(audit.rows) != 1 || audit.rows[0].Outcome != "failed" || audit.rows[0].Platform != "retreaver" {
		t.Fatalf("rows %+v", audit.rows)
	}
}

// failingPutTransport fails every PUT at the transport layer, quoting the full URL the way
// net/http does, and passes GETs through.
type failingPutTransport struct{ next http.RoundTripper }

func (f failingPutTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodPut {
		return nil, &url.Error{Op: "Put", URL: r.URL.String(), Err: fmt.Errorf("connection reset by peer")}
	}
	return f.next.RoundTrip(r)
}

// Rule 1 at the Service level: every failure path — 4xx, 5xx, 429 exhausted, transport
// error, GET failure — leaves no key in the returned error, the Result message or any audit row.
func TestRetreaverRedactionOnEveryFailurePath(t *testing.T) {
	echo := `{"error":"rejected {URL}"}`
	type tc struct {
		setup  func(*fakeRetreaverAPI)
		client *http.Client
		void   bool
	}
	cases := map[string]tc{
		"PUT 400 echo":        {setup: func(f *fakeRetreaverAPI) { f.putStatuses, f.putBody = []int{400}, echo }},
		"PUT 401 echo":        {setup: func(f *fakeRetreaverAPI) { f.putStatuses, f.putBody = []int{401}, echo }},
		"PUT 403 echo":        {setup: func(f *fakeRetreaverAPI) { f.putStatuses, f.putBody = []int{403}, echo }},
		"PUT 500 x3 echo":     {setup: func(f *fakeRetreaverAPI) { f.putStatuses, f.putBody = []int{500, 500, 500}, echo }},
		"PUT 429 x3":          {setup: func(f *fakeRetreaverAPI) { f.putStatuses, f.putBody = []int{429, 429, 429}, echo }},
		"PUT transport error": {setup: func(*fakeRetreaverAPI) {}, client: &http.Client{Transport: failingPutTransport{next: http.DefaultTransport}}},
		"void PUT 500 echo": {setup: func(f *fakeRetreaverAPI) {
			f.revenue, f.converted = f64(1), true
			f.putStatuses, f.putBody = []int{500, 500, 500}, echo
		}, void: true},
		"GET 500": {setup: func(f *fakeRetreaverAPI) { f.getStatus = 500 }},
		"GET 404": {setup: func(f *fakeRetreaverAPI) { f.getStatus = 404 }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			api := newFakeRetreaverAPI()
			c.setup(api)
			audit := &memAudit{}
			svc := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, c.client)
			var res Result
			var err error
			if c.void {
				res, err = svc.Void(context.Background(), rtReq())
			} else {
				res, err = svc.Convert(context.Background(), rtReq())
			}
			if err == nil {
				t.Fatal("want error")
			}
			noSecret(t, "returned error", err.Error())
			noSecret(t, "result message", res.Message)
			b, _ := json.Marshal(res)
			noSecret(t, "result JSON", string(b))
			auditNoSecrets(t, audit)
		})
	}
}

func TestRetreaverSharedGuards(t *testing.T) {
	t.Run("kill switch", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		audit := &memAudit{}
		svc := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil)
		svc.SetGlobalEnabled(false)
		_, err := svc.Convert(context.Background(), rtReq())
		if !errors.Is(err, ErrGloballyDisabled) || HTTPStatus(err) != 403 {
			t.Fatalf("err=%v", err)
		}
		_, err = svc.Void(context.Background(), rtReq())
		if !errors.Is(err, ErrGloballyDisabled) {
			t.Fatalf("void err=%v", err)
		}
		if len(audit.rows) != 2 || audit.rows[0].Outcome != "rejected" || atomic.LoadInt32(&api.gets) != 0 || api.putCount() != 0 {
			t.Fatalf("rows=%+v gets=%d puts=%d", audit.rows, api.gets, api.putCount())
		}
	})
	t.Run("tenant flag off", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		audit := &memAudit{}
		item := retreaverItem()
		item.WritebackEnabled = false
		_, err := rtSvc(t, api, item, NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
		if !errors.Is(err, ErrTenantDisabled) || HTTPStatus(err) != 403 || len(audit.rows) != 1 || api.gets != 0 {
			t.Fatalf("err=%v rows=%d gets=%d", err, len(audit.rows), api.gets)
		}
	})
	t.Run("not live", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		audit := &memAudit{}
		item := retreaverItem()
		item.SyncMode = "mock"
		_, err := rtSvc(t, api, item, NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
		if !errors.Is(err, ErrNotLive) || HTTPStatus(err) != 403 || len(audit.rows) != 1 || api.gets != 0 {
			t.Fatalf("err=%v rows=%d gets=%d", err, len(audit.rows), api.gets)
		}
	})
	t.Run("409 lock held", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		audit := &memAudit{}
		lock := NewMemoryLocker()
		_, _ = lock.Acquire(context.Background(), "ten-a", "convert", rtUUID)
		_, err := rtSvc(t, api, retreaverItem(), lock, audit, nil).Convert(context.Background(), rtReq())
		if !errors.Is(err, ErrAlreadyProcessing) || HTTPStatus(err) != 409 || len(audit.rows) != 1 || audit.rows[0].Outcome != "rejected" {
			t.Fatalf("err=%v rows=%+v", err, audit.rows)
		}
		if api.gets != 0 || api.putCount() != 0 {
			t.Fatal("no Retreaver request while locked")
		}
	})
	t.Run("double click one PUT", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		audit := &memAudit{}
		slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				time.Sleep(80 * time.Millisecond)
			}
			api.ServeHTTP(w, r)
		})
		svc := rtSvc(t, slow, retreaverItem(), NewMemoryLocker(), audit, nil)
		var wg sync.WaitGroup
		var first, second error
		wg.Add(2)
		go func() { defer wg.Done(); _, first = svc.Convert(context.Background(), rtReq()) }()
		time.Sleep(20 * time.Millisecond)
		go func() { defer wg.Done(); _, second = svc.Convert(context.Background(), rtReq()) }()
		wg.Wait()
		if first != nil || !errors.Is(second, ErrAlreadyProcessing) || api.putCount() != 1 {
			t.Fatalf("first=%v second=%v puts=%d", first, second, api.putCount())
		}
	})
	t.Run("amount must be finite and positive, no cap", func(t *testing.T) {
		for _, amt := range []float64{0, -1, 0.004} {
			api := newFakeRetreaverAPI()
			audit := &memAudit{}
			req := rtReq()
			req.Amount = amt
			_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), req)
			if !errors.Is(err, ErrInvalidAmount) || HTTPStatus(err) != 400 || len(audit.rows) != 1 || api.putCount() != 0 {
				t.Fatalf("amount %v: err=%v rows=%d puts=%d", amt, err, len(audit.rows), api.putCount())
			}
		}
		api := newFakeRetreaverAPI()
		req := rtReq()
		req.Amount = 1000
		res, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), &memAudit{}, nil).Convert(context.Background(), req)
		if err != nil || res.Outcome != "confirmed" || api.puts[0] != `{"call":{"revenue":1000}}` {
			t.Fatalf("no cap: res=%+v err=%v puts=%v", res, err, api.puts)
		}
	})
	t.Run("invalid uuid", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		audit := &memAudit{}
		req := rtReq()
		req.InboundCallID = "RGBTEST01"
		_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), req)
		if !errors.Is(err, ErrInvalidCallID) || HTTPStatus(err) != 400 || len(audit.rows) != 1 || api.gets != 0 {
			t.Fatalf("err=%v rows=%d gets=%d", err, len(audit.rows), api.gets)
		}
	})
	t.Run("call of another company", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		api.company = "99999"
		audit := &memAudit{}
		_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
		if !errors.Is(err, ErrAccountMismatch) || HTTPStatus(err) != 403 || api.putCount() != 0 || len(audit.rows) != 1 {
			t.Fatalf("err=%v puts=%d rows=%d", err, api.putCount(), len(audit.rows))
		}
		if strings.Contains(err.Error(), "Ringba") {
			t.Fatalf("message names the wrong platform: %v", err)
		}
	})
	t.Run("call not found", func(t *testing.T) {
		api := newFakeRetreaverAPI()
		api.getStatus = 404
		audit := &memAudit{}
		_, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
		if !errors.Is(err, ErrCallNotFound) || HTTPStatus(err) != 404 || !strings.Contains(err.Error(), "Retreaver") {
			t.Fatalf("err=%v", err)
		}
		if len(audit.rows) != 1 || audit.rows[0].Outcome != "failed" {
			t.Fatalf("rows %+v", audit.rows)
		}
	})
	t.Run("missing company_id", func(t *testing.T) {
		w := NewRetreaverWriter(nil, 0)
		if err := w.CheckCredentials(map[string]string{"api_key": "k"}); err == nil {
			t.Fatal("missing company_id accepted")
		}
	})
}

func TestRetreaverPixelAndPayoutWatchRecorded(t *testing.T) {
	api := newFakeRetreaverAPI()
	api.firePixelOnConv = true
	audit := &memAudit{}
	res, err := rtSvc(t, api, retreaverItem(), NewMemoryLocker(), audit, nil).Convert(context.Background(), rtReq())
	if err != nil {
		t.Fatal(err)
	}
	if res.ReadBack == nil || res.ReadBack.FiredPixelsDelta == nil || *res.ReadBack.FiredPixelsDelta != 1 {
		t.Fatalf("pixel delta not recorded: %+v", res.ReadBack)
	}
	rb := decodeReadBack(t, audit.rows[0].ReadBack)
	if rb.FiredPixelsDelta == nil || *rb.FiredPixelsDelta != 1 || rb.PayoutChanged {
		t.Fatalf("audit read_back %s", audit.rows[0].ReadBack)
	}
}

func TestRetreaverSettleWindowFromEnv(t *testing.T) {
	if NewRetreaverWriter(nil, 0).SettleWindow() != 20*time.Second {
		t.Fatal("default must be 20s until Phase 3 measures it")
	}
	if NewRetreaverWriter(nil, ParseSettleWindow("7")).SettleWindow() != 7*time.Second {
		t.Fatal("override")
	}
	s := NewService(&fakeStore{}, nil, rtDec{}, &fakeRingba{}, NewMemoryLocker())
	s.Register(NewRetreaverWriter(nil, 9*time.Second))
	if s.windowForPlatform("retreaver") != 9*time.Second || s.windowForPlatform("ringba") != SettleWindow {
		t.Fatal("per-platform settle window")
	}
}
