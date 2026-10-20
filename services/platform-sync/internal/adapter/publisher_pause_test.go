package adapter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const ppFakeKey = "FAKEKEY-9f8e7d6c5b4a"

func noSleep(context.Context, time.Duration) error { return nil }

type recorded struct {
	Method, Path, RawQuery, Body, Auth string
}

type recorder struct {
	mu   sync.Mutex
	reqs []recorded
}

func (r *recorder) add(req *http.Request) recorded {
	b, _ := io.ReadAll(req.Body)
	rec := recorded{req.Method, req.URL.Path, req.URL.RawQuery, string(b), req.Header.Get("Authorization")}
	r.mu.Lock()
	r.reqs = append(r.reqs, rec)
	r.mu.Unlock()
	return rec
}

func (r *recorder) count(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, q := range r.reqs {
		if method == "" || q.Method == method {
			n++
		}
	}
	return n
}

func ringbaTestClient(base string) *RingbaPublisherClient {
	return NewRingbaPublisherClientWith(RingbaPublisherClientOptions{
		BaseURL: base, Sleep: noSleep, Limiters: NewRingbaAccountLimiters(),
	})
}

func retreaverTestClient(origin string) *RetreaverAffiliateClient {
	return NewRetreaverAffiliateClientWith(RetreaverAffiliateClientOptions{
		Origin: origin, Sleep: noSleep, Limiters: NewRetreaverAccountLimiters(),
	})
}

var ringbaCredsT = map[string]string{"api_token": ppFakeKey, "account_id": "RA123"}
var retreaverCredsT = map[string]string{"api_key": ppFakeKey, "company_id": "4242"}

func TestValidPlatformID(t *testing.T) {
	good := []string{"PI1234abcd", "11111", "a-b_c.d"}
	bad := []string{"", "a/b", "a?b", "a#b", "..", "a..b", "a b", " a", "a\tb", "a%2Fb", strings.Repeat("x", 65), "ü"}
	for _, g := range good {
		if !ValidPlatformID(g) {
			t.Errorf("%q should be valid", g)
		}
	}
	for _, b := range bad {
		if ValidPlatformID(b) {
			t.Errorf("%q should be invalid", b)
		}
	}
}

func TestPauseBodiesExact(t *testing.T) {
	b, _ := RingbaPublisherEnabledBody(false)
	if string(b) != `{"enabled":false}` {
		t.Fatalf("ringba body %s", b)
	}
	b, _ = RingbaPublisherEnabledBody(true)
	if string(b) != `{"enabled":true}` {
		t.Fatalf("ringba body %s", b)
	}
	b, _ = RetreaverAffiliatePausedBody(true)
	if string(b) != `{"affiliate":{"paused":true}}` {
		t.Fatalf("retreaver body %s", b)
	}
	b, _ = RetreaverAffiliatePausedBody(false)
	if string(b) != `{"affiliate":{"paused":false}}` {
		t.Fatalf("retreaver body %s", b)
	}
}

func TestRingbaPublisherGetAndPatch(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := rec.add(r)
		if q.Path != "/v2/RA123/Publishers/PI1" {
			w.WriteHeader(404)
			return
		}
		if r.Method == http.MethodPatch {
			w.WriteHeader(200)
			return
		}
		_, _ = w.Write([]byte(`{"publisher":{"id":"PI1","name":"Pub One","enabled":true}}`))
	}))
	defer srv.Close()
	c := ringbaTestClient(srv.URL + "/v2")
	p, err := c.GetPublisher(context.Background(), ringbaCredsT, "PI1")
	if err != nil || p.Enabled == nil || !*p.Enabled || p.Name != "Pub One" {
		t.Fatalf("get: %+v %v", p, err)
	}
	if err := c.SetPublisherEnabled(context.Background(), ringbaCredsT, "PI1", false); err != nil {
		t.Fatal(err)
	}
	last := rec.reqs[len(rec.reqs)-1]
	if last.Method != http.MethodPatch || last.Body != `{"enabled":false}` || last.Auth != "Token "+ppFakeKey {
		t.Fatalf("patch request %+v", last)
	}
	if last.RawQuery != "" {
		t.Fatalf("ringba must not use query auth: %q", last.RawQuery)
	}
	if _, err := c.GetPublisher(context.Background(), ringbaCredsT, "PI2"); !errors.Is(err, ErrPublisherNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestRingbaPublisherInvalidIDNoRequest(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { rec.add(r) }))
	defer srv.Close()
	c := ringbaTestClient(srv.URL + "/v2")
	for _, id := range []string{"../x", "a/b", "a?b", "a#b", "a b"} {
		if err := c.SetPublisherEnabled(context.Background(), ringbaCredsT, id, false); !errors.Is(err, ErrInvalidPlatformID) {
			t.Fatalf("%q: %v", id, err)
		}
	}
	if rec.count("") != 0 {
		t.Fatalf("no request expected, got %d", rec.count(""))
	}
}

func TestPauseRetry429ThenSuccess(t *testing.T) {
	rec := &recorder{}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		n++
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	var slept []time.Duration
	c := NewRingbaPublisherClientWith(RingbaPublisherClientOptions{
		BaseURL: srv.URL + "/v2", Limiters: NewRingbaAccountLimiters(),
		Sleep: func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil },
	})
	if err := c.SetPublisherEnabled(context.Background(), ringbaCredsT, "PI1", false); err != nil {
		t.Fatal(err)
	}
	if rec.count(http.MethodPatch) != 2 || len(slept) != 1 || slept[0] != time.Second {
		t.Fatalf("patches=%d slept=%v", rec.count(http.MethodPatch), slept)
	}
}

func TestPause4xxNotRetried(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`bad request for ` + r.URL.String()))
	}))
	defer srv.Close()
	c := retreaverTestClient(srv.URL)
	_, err := c.SetPaused(context.Background(), retreaverCredsT, "71582", true)
	if err == nil || rec.count(http.MethodPut) != 1 {
		t.Fatalf("err=%v puts=%d", err, rec.count(http.MethodPut))
	}
	if strings.Contains(err.Error(), ppFakeKey) {
		t.Fatalf("key leaked in error: %v", err)
	}
}

func TestPause5xxExhaustsThreeAttempts(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		w.WriteHeader(503)
		_, _ = w.Write([]byte("down api_key=" + ppFakeKey))
	}))
	defer srv.Close()
	c := retreaverTestClient(srv.URL)
	_, err := c.SetPaused(context.Background(), retreaverCredsT, "71582", true)
	if err == nil || rec.count(http.MethodPut) != 3 {
		t.Fatalf("err=%v puts=%d", err, rec.count(http.MethodPut))
	}
	if strings.Contains(err.Error(), ppFakeKey) {
		t.Fatalf("key leaked: %v", err)
	}
}

func TestPauseTransportErrorNotRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused
	c := retreaverTestClient(url)
	_, err := c.SetPaused(context.Background(), retreaverCredsT, "71582", true)
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), ppFakeKey) {
		t.Fatalf("key leaked: %v", err)
	}
}

func TestRetreaverResolveAFIDDirect(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := rec.add(r)
		if !strings.Contains(q.RawQuery, "api_key=") || !strings.Contains(q.RawQuery, "company_id=4242") {
			w.WriteHeader(401)
			return
		}
		if q.Path == "/api/v1/affiliates/afid/11111.json" {
			_, _ = w.Write([]byte(`{"affiliate":{"id":71582,"afid":"11111","company_name":"Sandbox","paused":false}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c := retreaverTestClient(srv.URL)
	a, steps, err := c.ResolveAffiliate(context.Background(), retreaverCredsT, "11111")
	if err != nil || a.ID != "71582" || a.AFID != "11111" || a.Paused == nil || *a.Paused {
		t.Fatalf("%+v %v", a, err)
	}
	if len(steps) != 1 || strings.Contains(strings.Join(steps, ""), "?") {
		t.Fatalf("steps %v", steps)
	}
}

func TestRetreaverResolveAFID404FallsBackToList(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := rec.add(r)
		switch {
		case q.Path == "/api/v1/affiliates/afid/11111.json":
			w.WriteHeader(404)
		case q.Path == "/api/v1/affiliates.json" && strings.Contains(q.RawQuery, "page=1&"):
			// a full first page without the afid
			var sb strings.Builder
			sb.WriteString("[")
			for i := 0; i < 100; i++ {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(`{"affiliate":{"id":` + strconv.Itoa(1000+i) + `,"afid":"x` + strconv.Itoa(i) + `"}}`)
			}
			sb.WriteString("]")
			_, _ = w.Write([]byte(sb.String()))
		case q.Path == "/api/v1/affiliates.json" && strings.Contains(q.RawQuery, "page=2&"):
			_, _ = w.Write([]byte(`[{"affiliate":{"id":71582,"afid":"11111"}}]`))
		case q.Path == "/api/v1/affiliates/71582.json":
			_, _ = w.Write([]byte(`{"affiliate":{"id":71582,"afid":"11111","first_name":"A","last_name":"B"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := retreaverTestClient(srv.URL)
	a, steps, err := c.ResolveAffiliate(context.Background(), retreaverCredsT, "11111")
	if err != nil || a.ID != "71582" || a.Name != "A B" || a.Paused != nil {
		t.Fatalf("%+v %v", a, err)
	}
	if len(steps) != 4 {
		t.Fatalf("steps %v", steps)
	}
	// Not found anywhere.
	_, _, err = c.ResolveAffiliate(context.Background(), retreaverCredsT, "99999")
	if !errors.Is(err, ErrPublisherNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestRetreaverSetPausedRequest(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r)
		_, _ = w.Write([]byte(`{"affiliate":{"id":71582,"afid":"11111"}}`))
	}))
	defer srv.Close()
	c := retreaverTestClient(srv.URL)
	a, err := c.SetPaused(context.Background(), retreaverCredsT, "71582", true)
	if err != nil || a.Paused != nil {
		t.Fatalf("%+v %v", a, err)
	}
	q := rec.reqs[0]
	if q.Method != http.MethodPut || q.Path != "/api/v1/affiliates/71582.json" || q.Body != `{"affiliate":{"paused":true}}` {
		t.Fatalf("%+v", q)
	}
	if q.Auth != "" {
		t.Fatal("retreaver must not send an Authorization header")
	}
}
