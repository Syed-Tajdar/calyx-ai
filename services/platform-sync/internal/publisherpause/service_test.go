package publisherpause

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/writeback"
)

// fakeKey is injected as every credential value; no audit row, error, response or log may contain it.
const (
	fakeKey     = "FAKEKEY-0a1b2c3d4e5f6789"
	fakeAccount = "RAfakeacct77"
	fakeCompany = "998877"
	tenantID    = "11111111-1111-1111-1111-111111111111"
	intID       = "22222222-2222-2222-2222-222222222222"
	userID      = "33333333-3333-3333-3333-333333333333"
)

// ---- fakes -----------------------------------------------------------------------------------

type fakeStore struct {
	mu   sync.Mutex
	it   *store.PauseIntegration
	rows []store.PublisherAuditRow
}

func (f *fakeStore) GetPublisherPauseIntegration(_ context.Context, tenant, id string) (*store.PauseIntegration, error) {
	if f.it == nil || f.it.ID != id || f.it.TenantID != tenant {
		return nil, errors.New("integration not found")
	}
	cp := *f.it
	return &cp, nil
}

func (f *fakeStore) InsertPublisherActionAudit(_ context.Context, row store.PublisherAuditRow) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, row)
	return fmt.Sprintf("aaaaaaaa-0000-0000-0000-%012d", len(f.rows)), nil
}

// fakeDecryptor: the blob is the platform name.
type fakeDecryptor struct{}

func (fakeDecryptor) DecryptMap(blob []byte) (map[string]string, error) {
	switch string(blob) {
	case "ringba":
		return map[string]string{"api_token": fakeKey, "account_id": fakeAccount}, nil
	case "retreaver":
		return map[string]string{"api_key": fakeKey, "company_id": fakeCompany}, nil
	}
	return nil, errors.New("bad blob")
}

type req struct{ Method, Path, Query, Body, Auth string }

// fakeRingba is a fake Ringba Publishers API.
type fakeRingba struct {
	mu         sync.Mutex
	enabled    map[string]bool
	reqs       []req
	ignorePut  bool // PATCH accepted but nothing changes (contradiction)
	fail429    int  // first N requests answer 429
	patchCode  int  // non-zero: PATCH answers this status
	noEnabled  bool // GET has no "enabled" key
	bodyEchoes bool // error bodies echo the Authorization header
}

func (f *fakeRingba) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req{r.Method, r.URL.Path, r.URL.RawQuery, string(b), r.Header.Get("Authorization")})
	if r.Header.Get("Authorization") != "Token "+fakeKey {
		w.WriteHeader(401)
		return
	}
	if f.fail429 > 0 {
		f.fail429--
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(429)
		return
	}
	prefix := "/v2/" + fakeAccount + "/Publishers/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		w.WriteHeader(404)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, prefix)
	en, ok := f.enabled[id]
	if !ok {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
		return
	}
	switch r.Method {
	case http.MethodGet:
		if f.noEnabled {
			_, _ = fmt.Fprintf(w, `{"id":%q,"name":"Pub"}`, id)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":%q,"name":"Pub","enabled":%v}`, id, en)
	case http.MethodPatch:
		if f.patchCode != 0 {
			w.WriteHeader(f.patchCode)
			if f.bodyEchoes {
				_, _ = w.Write([]byte("rejected; Authorization: Token " + fakeKey))
			}
			return
		}
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		_ = json.Unmarshal(b, &body)
		if body.Enabled != nil && !f.ignorePut {
			f.enabled[id] = *body.Enabled
		}
		_, _ = w.Write([]byte(`{}`))
	default:
		w.WriteHeader(405)
	}
}

func (f *fakeRingba) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, q := range f.reqs {
		if method == "" || q.Method == method {
			n++
		}
	}
	return n
}

// fakeRetreaver is a fake Retreaver affiliates API.
type fakeRetreaver struct {
	mu          sync.Mutex
	byID        map[string]*aff
	afid404     bool // the afid lookup answers 404 (Phase 0 behaviour)
	showPaused  bool // GET includes a "paused" key
	ignorePut   bool // PUT accepted but state unchanged
	putCode     int
	reqs        []req
	echoKeyIn4x bool
}

type aff struct {
	ID, AFID string
	Paused   bool
}

func (f *fakeRetreaver) render(a *aff) string {
	if f.showPaused {
		return fmt.Sprintf(`{"affiliate":{"id":%s,"afid":%q,"company_name":"X","paused":%v}}`, a.ID, a.AFID, a.Paused)
	}
	return fmt.Sprintf(`{"affiliate":{"id":%s,"afid":%q,"company_name":"X"}}`, a.ID, a.AFID)
}

func (f *fakeRetreaver) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req{r.Method, r.URL.Path, r.URL.RawQuery, string(b), r.Header.Get("Authorization")})
	q := r.URL.Query()
	if q.Get("api_key") != fakeKey || q.Get("company_id") != fakeCompany {
		w.WriteHeader(401)
		return
	}
	p := r.URL.Path
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/v1/affiliates/afid/"):
		afid := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/affiliates/afid/"), ".json")
		if f.afid404 {
			w.WriteHeader(404)
			return
		}
		for _, a := range f.byID {
			if a.AFID == afid {
				_, _ = w.Write([]byte(f.render(a)))
				return
			}
		}
		w.WriteHeader(404)
	case r.Method == http.MethodGet && p == "/api/v1/affiliates.json":
		if q.Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		var parts []string
		for _, a := range f.byID {
			parts = append(parts, f.render(a))
		}
		_, _ = w.Write([]byte("[" + strings.Join(parts, ",") + "]"))
	case strings.HasPrefix(p, "/api/v1/affiliates/") && strings.HasSuffix(p, ".json"):
		id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v1/affiliates/"), ".json")
		a, ok := f.byID[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(f.render(a)))
			return
		}
		if r.Method != http.MethodPut {
			w.WriteHeader(405)
			return
		}
		if f.putCode != 0 {
			w.WriteHeader(f.putCode)
			if f.echoKeyIn4x {
				_, _ = w.Write([]byte("error for " + r.URL.String()))
			}
			return
		}
		var body struct {
			Affiliate struct {
				Paused *bool `json:"paused"`
			} `json:"affiliate"`
		}
		_ = json.Unmarshal(b, &body)
		if body.Affiliate.Paused != nil && !f.ignorePut {
			a.Paused = *body.Affiliate.Paused
		}
		// Documented response: the affiliate without "paused".
		_, _ = fmt.Fprintf(w, `{"affiliate":{"id":%s,"afid":%q}}`, a.ID, a.AFID)
	default:
		w.WriteHeader(404)
	}
}

func (f *fakeRetreaver) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, q := range f.reqs {
		if method == "" || q.Method == method {
			n++
		}
	}
	return n
}

// ---- harness ---------------------------------------------------------------------------------

type env struct {
	t         *testing.T
	st        *fakeStore
	lock      *MemoryLocker
	ringba    *fakeRingba
	retreaver *fakeRetreaver
	svc       *Service
	logs      *bytes.Buffer
}

func newEnv(t *testing.T, platform string, cfg Config) *env {
	t.Helper()
	t.Setenv("RETREAVER_RATE_PER_SEC", "1000")
	t.Setenv("RETREAVER_RATE_PER_MIN", "100000")
	t.Setenv("RINGBA_RATE_PER_SEC", "1000")
	t.Setenv("RINGBA_RATE_PER_MIN", "100000")
	e := &env{
		t:    t,
		lock: NewMemoryLocker(),
		ringba: &fakeRingba{enabled: map[string]bool{
			"PIlive1": true, "PIoff": false,
		}},
		retreaver: &fakeRetreaver{byID: map[string]*aff{
			"71582": {ID: "71582", AFID: "11111"},
		}},
		logs: &bytes.Buffer{},
	}
	e.st = &fakeStore{it: &store.PauseIntegration{
		ID: intID, TenantID: tenantID, Platform: platform, Name: "x", SyncMode: "real",
		IsActive: true, AllowPublisherPause: true, APICredentialsBlob: []byte(platform),
	}}
	rs := httptest.NewServer(http.HandlerFunc(e.ringba.handler))
	ts := httptest.NewServer(http.HandlerFunc(e.retreaver.handler))
	t.Cleanup(rs.Close)
	t.Cleanup(ts.Close)
	noSleep := func(context.Context, time.Duration) error { return nil }
	rc := adapter.NewRingbaPublisherClientWith(adapter.RingbaPublisherClientOptions{
		BaseURL: rs.URL + "/v2", Sleep: noSleep, Limiters: adapter.NewRingbaAccountLimiters(),
	})
	tc := adapter.NewRetreaverAffiliateClientWith(adapter.RetreaverAffiliateClientOptions{
		Origin: ts.URL, Sleep: noSleep, Limiters: adapter.NewRetreaverAccountLimiters(),
	})
	e.svc = NewService(cfg, e.st, e.st, fakeDecryptor{}, e.lock, rc, tc)
	e.svc.SetReadBackPolicy(2, 0)
	return e
}

var allOn = Config{ActionsEnabled: true, RingbaPauseVerified: true}

func (e *env) request(publisher string) Request {
	return Request{
		TenantID: tenantID, IntegrationID: intID, PublisherID: publisher, UserID: userID,
		Mode: "manual", Numbers: json.RawMessage(`{"calls":42,"conversion_rate":1.5}`),
	}
}

// expectErr asserts status/outcome and that exactly one audit row was written with that outcome.
func (e *env) expectErr(err error, status int, outcome string) *Error {
	e.t.Helper()
	var pe *Error
	if !errors.As(err, &pe) {
		e.t.Fatalf("want *Error, got %v", err)
	}
	if pe.Status != status || pe.Outcome != outcome {
		e.t.Fatalf("got status=%d outcome=%s msg=%q, want %d %s", pe.Status, pe.Outcome, pe.Message, status, outcome)
	}
	if pe.AuditID == "" {
		e.t.Fatal("error has no audit id")
	}
	e.expectOneAudit(outcome)
	return pe
}

func (e *env) expectOneAudit(outcome string) store.PublisherAuditRow {
	e.t.Helper()
	if len(e.st.rows) != 1 {
		e.t.Fatalf("want exactly 1 audit row, got %d", len(e.st.rows))
	}
	row := e.st.rows[0]
	if row.Outcome != outcome {
		e.t.Fatalf("audit outcome %s, want %s", row.Outcome, outcome)
	}
	if row.TenantID != tenantID || row.Mode == "" || row.Action == "" || row.Platform == "" {
		e.t.Fatalf("audit row incomplete: %+v", row)
	}
	return row
}

// noSecrets asserts the fake key (and account / company ids) appear nowhere.
func (e *env) noSecrets(extra ...string) {
	e.t.Helper()
	var all []string
	for _, r := range e.st.rows {
		b, _ := json.Marshal(r)
		all = append(all, string(b))
	}
	all = append(all, e.logs.String())
	all = append(all, extra...)
	joined := strings.Join(all, "\n")
	for _, s := range []string{fakeKey, fakeAccount, "api_key=" + fakeKey, "company_id=" + fakeCompany} {
		if strings.Contains(joined, s) {
			e.t.Fatalf("secret %q leaked:\n%s", s, joined)
		}
	}
	for _, r := range e.st.rows {
		if strings.Contains(r.RequestRedacted, "?") || strings.Contains(r.RequestRedacted, "http") {
			e.t.Fatalf("request_redacted carries a URL or query: %q", r.RequestRedacted)
		}
	}
}

// ---- Ringba ----------------------------------------------------------------------------------

func TestRingbaPauseConfirmedByReadBack(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	res, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" || res.ReadBack != "ok" || !res.Paused || res.AuditID == "" {
		t.Fatalf("%+v", res)
	}
	if e.ringba.enabled["PIlive1"] {
		t.Fatal("publisher still enabled")
	}
	if e.ringba.count(http.MethodPatch) != 1 || e.ringba.count(http.MethodGet) != 2 {
		t.Fatalf("patch=%d get=%d", e.ringba.count(http.MethodPatch), e.ringba.count(http.MethodGet))
	}
	for _, q := range e.ringba.reqs {
		if q.Method == http.MethodPatch && q.Body != `{"enabled":false}` {
			t.Fatalf("patch body %s", q.Body)
		}
	}
	row := e.expectOneAudit("confirmed")
	if row.ReadBack != "ok" || row.Action != "pause" || row.PublisherID != "PIlive1" || row.UserID != userID ||
		row.Numbers != `{"calls":42,"conversion_rate":1.5}` {
		t.Fatalf("%+v", row)
	}
	want := `GET /v2/{accountId}/Publishers/PIlive1 ; PATCH /v2/{accountId}/Publishers/PIlive1 {"enabled":false} ; GET /v2/{accountId}/Publishers/PIlive1`
	if row.RequestRedacted != want {
		t.Fatalf("request_redacted %q", row.RequestRedacted)
	}
	e.noSecrets()
}

func TestRingbaUnpause(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	res, err := e.svc.Unpause(context.Background(), e.request("PIoff"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" || res.Paused || res.ReadBack != "ok" {
		t.Fatalf("%+v", res)
	}
	if !e.ringba.enabled["PIoff"] {
		t.Fatal("not re-enabled")
	}
	if e.st.rows[0].Action != "unpause" {
		t.Fatal("action")
	}
	e.noSecrets()
}

func TestRingbaContradictionIs502(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	e.ringba.ignorePut = true
	_, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	e.expectErr(err, 502, "failed")
	if e.st.rows[0].ReadBack != "mismatch" {
		t.Fatalf("read_back %s", e.st.rows[0].ReadBack)
	}
	e.noSecrets(err.Error())
}

func TestRingbaAlreadyPausedNoWrite(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	res, err := e.svc.Pause(context.Background(), e.request("PIoff"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed" || !strings.Contains(res.Message, "already paused") {
		t.Fatalf("%+v", res)
	}
	if e.ringba.count(http.MethodPatch) != 0 {
		t.Fatal("a write was sent for an already-paused publisher")
	}
	e.expectOneAudit("confirmed")
}

func TestRingbaUnverifiedRefusedNoRequest(t *testing.T) {
	e := newEnv(t, "ringba", Config{ActionsEnabled: true, RingbaPauseVerified: false})
	_, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	e.expectErr(err, 403, "rejected")
	if e.ringba.count("") != 0 {
		t.Fatal("request sent while unverified")
	}
}

func TestRingbaPublisherNotFound404(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	_, err := e.svc.Pause(context.Background(), e.request("PInope"))
	e.expectErr(err, 404, "failed")
	if e.ringba.count(http.MethodPatch) != 0 {
		t.Fatal("write sent for unknown publisher")
	}
}

func TestRingba429ThenSuccess(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	e.ringba.fail429 = 1
	res, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	if err != nil || res.Outcome != "confirmed" {
		t.Fatalf("%+v %v", res, err)
	}
	e.expectOneAudit("confirmed")
}

func TestRingba4xxNotRetried(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	e.ringba.patchCode = 403
	e.ringba.bodyEchoes = true
	_, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	e.expectErr(err, 502, "failed")
	if e.ringba.count(http.MethodPatch) != 1 {
		t.Fatalf("4xx retried: %d patches", e.ringba.count(http.MethodPatch))
	}
	e.noSecrets(err.Error())
}

func TestRingba5xxAfterRetriesIs502(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	e.ringba.patchCode = 500
	_, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	e.expectErr(err, 502, "failed")
	if e.ringba.count(http.MethodPatch) != 3 {
		t.Fatalf("want 3 attempts, got %d", e.ringba.count(http.MethodPatch))
	}
}

func TestRingbaNoEnabledFieldNoWrite(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	e.ringba.noEnabled = true
	_, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	e.expectErr(err, 502, "failed")
	if e.ringba.count(http.MethodPatch) != 0 {
		t.Fatal("wrote without a readable state")
	}
}

// ---- Retreaver -------------------------------------------------------------------------------

func TestRetreaverPauseConfirmedByResponse(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	res, err := e.svc.Pause(context.Background(), e.request("11111"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "confirmed_by_response" || res.ReadBack != "none" || !res.Paused {
		t.Fatalf("%+v", res)
	}
	if !e.retreaver.byID["71582"].Paused {
		t.Fatal("affiliate not paused")
	}
	for _, q := range e.retreaver.reqs {
		if q.Method == http.MethodPut && (q.Body != `{"affiliate":{"paused":true}}` || q.Path != "/api/v1/affiliates/71582.json") {
			t.Fatalf("put %+v", q)
		}
	}
	row := e.expectOneAudit("confirmed_by_response")
	want := `GET /api/v1/affiliates/afid/11111.json ; PUT /api/v1/affiliates/71582.json {"affiliate":{"paused":true}} ; GET /api/v1/affiliates/71582.json`
	if row.RequestRedacted != want {
		t.Fatalf("request_redacted %q", row.RequestRedacted)
	}
	e.noSecrets()
}

func TestRetreaverPausedKeyOK(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.retreaver.showPaused = true
	res, err := e.svc.Pause(context.Background(), e.request("11111"))
	if err != nil || res.Outcome != "confirmed" || res.ReadBack != "ok" {
		t.Fatalf("%+v %v", res, err)
	}
	e.expectOneAudit("confirmed")
}

func TestRetreaverPausedKeyMismatch502(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.retreaver.showPaused = true
	e.retreaver.ignorePut = true
	_, err := e.svc.Pause(context.Background(), e.request("11111"))
	e.expectErr(err, 502, "failed")
	if e.st.rows[0].ReadBack != "mismatch" {
		t.Fatal("read_back should be mismatch")
	}
}

func TestRetreaverAFID404FallsBackToList(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.retreaver.afid404 = true
	res, err := e.svc.Pause(context.Background(), e.request("11111"))
	if err != nil || res.Outcome != "confirmed_by_response" {
		t.Fatalf("%+v %v", res, err)
	}
	if !e.retreaver.byID["71582"].Paused {
		t.Fatal("not paused")
	}
	row := e.expectOneAudit("confirmed_by_response")
	if !strings.Contains(row.RequestRedacted, "GET /api/v1/affiliates.json page=1") {
		t.Fatalf("fallback not recorded: %q", row.RequestRedacted)
	}
	e.noSecrets()
}

func TestRetreaverUnpause(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.retreaver.byID["71582"].Paused = true
	res, err := e.svc.Unpause(context.Background(), e.request("11111"))
	if err != nil || res.Paused || res.Outcome != "confirmed_by_response" {
		t.Fatalf("%+v %v", res, err)
	}
	if e.retreaver.byID["71582"].Paused {
		t.Fatal("still paused")
	}
}

func TestRetreaverAlreadyPausedNoWrite(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.retreaver.showPaused = true
	e.retreaver.byID["71582"].Paused = true
	res, err := e.svc.Pause(context.Background(), e.request("11111"))
	if err != nil || res.Outcome != "confirmed" || !strings.Contains(res.Message, "already paused") {
		t.Fatalf("%+v %v", res, err)
	}
	if e.retreaver.count(http.MethodPut) != 0 {
		t.Fatal("write sent")
	}
	e.expectOneAudit("confirmed")
}

func TestRetreaverNotFound404(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	_, err := e.svc.Pause(context.Background(), e.request("55555"))
	e.expectErr(err, 404, "failed")
	if e.retreaver.count(http.MethodPut) != 0 {
		t.Fatal("write sent")
	}
}

func TestRetreaver4xxNoKeyLeak(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.retreaver.putCode = 422
	e.retreaver.echoKeyIn4x = true
	_, err := e.svc.Pause(context.Background(), e.request("11111"))
	pe := e.expectErr(err, 502, "failed")
	if e.retreaver.count(http.MethodPut) != 1 {
		t.Fatal("4xx retried")
	}
	e.noSecrets(pe.Message, err.Error())
}

// ---- refusals (no platform request, one rejected audit row) -----------------------------------

func TestKillSwitchOff403NoRequest(t *testing.T) {
	for _, p := range []string{"ringba", "retreaver"} {
		e := newEnv(t, p, Config{ActionsEnabled: false, RingbaPauseVerified: true})
		_, err := e.svc.Pause(context.Background(), e.request("11111"))
		e.expectErr(err, 403, "rejected")
		if e.ringba.count("")+e.retreaver.count("") != 0 {
			t.Fatalf("%s: request sent with kill switch off", p)
		}
	}
}

func TestToggleOff403(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.st.it.AllowPublisherPause = false
	_, err := e.svc.Pause(context.Background(), e.request("11111"))
	pe := e.expectErr(err, 403, "rejected")
	if !strings.Contains(pe.Message, "Allow Calyx to pause publishers") || e.retreaver.count("") != 0 {
		t.Fatalf("%q", pe.Message)
	}
}

func TestTestIntegration403(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.st.it.SyncMode = "mock"
	_, err := e.svc.Pause(context.Background(), e.request("11111"))
	e.expectErr(err, 403, "rejected")
	if e.retreaver.count("") != 0 {
		t.Fatal("request sent")
	}
}

func TestPausedIntegration403(t *testing.T) {
	e := newEnv(t, "ringba", allOn)
	e.st.it.IsActive = false
	_, err := e.svc.Pause(context.Background(), e.request("PIlive1"))
	e.expectErr(err, 403, "rejected")
	if e.ringba.count("") != 0 {
		t.Fatal("request sent")
	}
}

func TestUnsupportedPlatform403(t *testing.T) {
	e := newEnv(t, "trackdrive", allOn)
	_, err := e.svc.Pause(context.Background(), e.request("x1"))
	e.expectErr(err, 403, "rejected")
}

func TestIntegrationMissing403AuditWithoutIntegrationFK(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	r := e.request("11111")
	r.IntegrationID = "44444444-4444-4444-4444-444444444444"
	_, err := e.svc.Pause(context.Background(), r)
	e.expectErr(err, 403, "rejected")
	if e.st.rows[0].IntegrationID != "" || e.st.rows[0].Platform != "unknown" {
		t.Fatalf("%+v", e.st.rows[0])
	}
}

func TestInvalidPublisherID400NoRequest(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	_, err := e.svc.Pause(context.Background(), e.request("../../calls"))
	e.expectErr(err, 400, "rejected")
	if e.retreaver.count("") != 0 {
		t.Fatal("request sent")
	}
	if e.st.rows[0].PublisherID != "(invalid)" {
		t.Fatalf("publisher id stored raw: %q", e.st.rows[0].PublisherID)
	}
}

func TestLockHeld409(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	e.lock.Hold(tenantID, "retreaver", "11111")
	_, err := e.svc.Unpause(context.Background(), e.request("11111"))
	e.expectErr(err, 409, "rejected")
	if e.retreaver.count("") != 0 {
		t.Fatal("request sent while locked")
	}
}

func TestLockReleasedAfterRun(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	if _, err := e.svc.Pause(context.Background(), e.request("11111")); err != nil {
		t.Fatal(err)
	}
	ok, _ := e.lock.Acquire(context.Background(), tenantID, "retreaver", "11111")
	if !ok {
		t.Fatal("lock not released")
	}
}

// ---- HTTP ------------------------------------------------------------------------------------

func TestDecodeRequestStrict(t *testing.T) {
	good := `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"11111","user_id":null,"mode":"manual","rule_id":null,"alert_event_id":null,"numbers":{}}`
	if _, err := DecodeRequest(strings.NewReader(good), ActionPause); err != nil {
		t.Fatalf("good request refused: %v", err)
	}
	bad := map[string]string{
		"unknown field":   `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":"manual","payout":5}`,
		"trailing data":   good + `{}`,
		"bad tenant":      `{"tenant_id":"x","integration_id":"` + intID + `","publisher_id":"1","mode":"manual"}`,
		"missing pub":     `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","mode":"manual"}`,
		"bad mode":        `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":"yolo"}`,
		"timer on pause":  `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":"timer"}`,
		"numbers array":   `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":"manual","numbers":[1]}`,
		"bad user":        `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":"manual","user_id":"bob"}`,
		"not json":        `nope`,
		"wrong type mode": `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":1}`,
	}
	for name, body := range bad {
		if _, err := DecodeRequest(strings.NewReader(body), ActionPause); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	timer := `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"1","mode":"timer"}`
	if _, err := DecodeRequest(strings.NewReader(timer), ActionUnpause); err != nil {
		t.Fatalf("timer unpause refused: %v", err)
	}
}

func newRouter(e *env, token string) http.Handler {
	logger := slog.New(slog.NewJSONHandler(e.logs, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /publisher/pause", writeback.RequireBearer(token, Handler(e.svc, ActionPause, logger)))
	mux.HandleFunc("POST /publisher/unpause", writeback.RequireBearer(token, Handler(e.svc, ActionUnpause, logger)))
	return mux
}

func body(publisher string) string {
	return `{"tenant_id":"` + tenantID + `","integration_id":"` + intID + `","publisher_id":"` + publisher +
		`","user_id":"` + userID + `","mode":"manual","rule_id":null,"alert_event_id":null,"numbers":{"calls":3}}`
}

func TestRouteRequiresBearer(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	h := newRouter(e, "internal-token")
	for _, auth := range []string{"", "Bearer wrong", "internal-token"} {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/publisher/pause", strings.NewReader(body("11111")))
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		h.ServeHTTP(rr, r)
		if rr.Code != 401 {
			t.Fatalf("auth %q: status %d", auth, rr.Code)
		}
	}
	if e.retreaver.count("") != 0 || len(e.st.rows) != 0 {
		t.Fatal("unauthenticated request reached the service")
	}
	// Empty configured token fails closed.
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/publisher/pause", strings.NewReader(body("11111")))
	r.Header.Set("Authorization", "Bearer ")
	newRouter(e, "").ServeHTTP(rr, r)
	if rr.Code != 401 {
		t.Fatalf("empty token: %d", rr.Code)
	}
}

func TestHTTPPauseAndErrorBodies(t *testing.T) {
	e := newEnv(t, "retreaver", allOn)
	h := newRouter(e, "internal-token")
	do := func(path, b string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(b))
		r.Header.Set("Authorization", "Bearer internal-token")
		h.ServeHTTP(rr, r)
		return rr
	}
	rr := do("/publisher/pause", body("11111"))
	if rr.Code != 200 {
		t.Fatalf("status %d %s", rr.Code, rr.Body)
	}
	var ok map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &ok)
	for _, k := range []string{"outcome", "paused", "read_back", "message", "audit_id"} {
		if _, has := ok[k]; !has {
			t.Fatalf("200 body lacks %s: %s", k, rr.Body)
		}
	}
	if ok["outcome"] != "confirmed_by_response" || ok["paused"] != true {
		t.Fatalf("%s", rr.Body)
	}

	// 404 from the platform.
	rr = do("/publisher/unpause", body("55555"))
	var eb map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &eb)
	if rr.Code != 404 || eb["outcome"] != "failed" || eb["audit_id"] == nil || eb["message"] == "" {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}

	// Strict decoding: unknown field → 400, nothing audited, no platform call.
	before := len(e.st.rows)
	calls := e.retreaver.count("")
	rr = do("/publisher/pause", strings.Replace(body("11111"), `"mode"`, `"payout":1,"mode"`, 1))
	if rr.Code != 400 || len(e.st.rows) != before || e.retreaver.count("") != calls {
		t.Fatalf("unknown field: %d rows=%d", rr.Code, len(e.st.rows))
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &eb)
	if _, has := eb["audit_id"]; !has || eb["audit_id"] != nil {
		t.Fatalf("400 body should carry audit_id null: %s", rr.Body)
	}

	// Kill switch through HTTP.
	e.svc.cfg.ActionsEnabled = false
	rr = do("/publisher/pause", body("11111"))
	if rr.Code != 403 {
		t.Fatalf("kill switch: %d", rr.Code)
	}
	e.noSecrets(rr.Body.String())
}
