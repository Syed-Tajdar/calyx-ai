package transcribe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
)

// Fake recording sources only: httptest servers stand in for Ringba's recording host and the
// pre-signed S3 link it redirects to. No real platform is called.

type fakeRingba struct {
	info adapter.RingbaRecordingInfo
	err  error
}

func (f fakeRingba) LookupRecording(context.Context, map[string]string, string, time.Time) (adapter.RingbaRecordingInfo, error) {
	return f.info, f.err
}

type fakeRetreaver struct {
	info adapter.RetreaverRecordingInfo
	err  error
}

func (f fakeRetreaver) RecordingLink(context.Context, map[string]string, string) (adapter.RetreaverRecordingInfo, error) {
	return f.info, f.err
}

// ringbaHosts: /rec needs Basic Auth and redirects to /s3?Signature=…; /s3 refuses any Authorization
// header (like a pre-signed S3 link: "Only one auth mechanism allowed").
func ringbaHosts(t *testing.T, audio []byte) (*httptest.Server, *int32, *int32) {
	var recHits, s3Hits int32
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	mux.HandleFunc("/rec/call-1", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&recHits, 1)
		u, p, ok := r.BasicAuth()
		if !ok || u != "calyxrecordings" || p != "s3cret-pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, srv.URL+"/s3/call-1.mp3?X-Amz-Signature=SIGNEDSECRET&Expires=1", http.StatusFound)
	})
	mux.HandleFunc("/s3/call-1.mp3", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s3Hits, 1)
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Only one auth mechanism allowed"))
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write(audio)
	})
	t.Cleanup(srv.Close)
	return srv, &recHits, &s3Hits
}

func TestRingbaRedirectWithRecordingAccess(t *testing.T) {
	audio := []byte("ID3fake-mp3-bytes")
	srv, recHits, s3Hits := ringbaHosts(t, audio)
	src := NewRingbaSource(fakeRingba{info: adapter.RingbaRecordingInfo{HasConnected: true, HasRecording: true, URL: srv.URL + "/rec/call-1"}})
	a, err := src.Fetch(context.Background(), Call{Platform: "ringba", ExternalCallID: "call-1", SyncMode: "real",
		RecordingAccess: map[string]string{"username": "calyxrecordings", "password": "s3cret-pass"}})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(a.Bytes) != string(audio) || a.ContentType != "audio/mpeg" {
		t.Fatalf("got %q %q", a.Bytes, a.ContentType)
	}
	if *recHits != 1 || *s3Hits != 1 {
		t.Fatalf("each hop must be fetched exactly once, got rec=%d s3=%d", *recHits, *s3Hits)
	}
}

func TestRingbaWithoutRecordingAccessIsNoAccess(t *testing.T) {
	srv, _, _ := ringbaHosts(t, []byte("x"))
	src := NewRingbaSource(fakeRingba{info: adapter.RingbaRecordingInfo{HasConnected: true, HasRecording: true, URL: srv.URL + "/rec/call-1"}})
	_, err := src.Fetch(context.Background(), Call{Platform: "ringba", ExternalCallID: "call-1", SyncMode: "real"})
	var nr *NoRecordingError
	if !errors.As(err, &nr) || nr.Reason != NoRecNoAccess || !strings.Contains(nr.Detail, "Recording Access") {
		t.Fatalf("want no_access naming the missing credentials, got %v", err)
	}
}

func TestNoRecordingReasons(t *testing.T) {
	cases := []struct {
		name string
		src  Source
		want string
	}{
		{"ringba not answered", NewRingbaSource(fakeRingba{info: adapter.RingbaRecordingInfo{}}), NoRecNotAnswered},
		{"ringba blocked", NewRingbaSource(fakeRingba{info: adapter.RingbaRecordingInfo{WasBlocked: true}}), NoRecBlockedCut},
		{"ringba recording off", NewRingbaSource(fakeRingba{info: adapter.RingbaRecordingInfo{HasConnected: true}}), NoRecDisabled},
		{"ringba call gone", NewRingbaSource(fakeRingba{err: adapter.ErrRecordingCallNotFound}), NoRecFetchFailed},
		{"retreaver blocked", NewRetreaverSource(fakeRetreaver{info: adapter.RetreaverRecordingInfo{Connected: true, Blocked: true}}), NoRecBlockedCut},
		{"retreaver not answered", NewRetreaverSource(fakeRetreaver{info: adapter.RetreaverRecordingInfo{Connected: false}}), NoRecNotAnswered},
		{"retreaver recording off", NewRetreaverSource(fakeRetreaver{info: adapter.RetreaverRecordingInfo{Connected: true}}), NoRecDisabled},
	}
	for _, c := range cases {
		_, err := c.src.Fetch(context.Background(), Call{ExternalCallID: "x", SyncMode: "real"})
		var nr *NoRecordingError
		if !errors.As(err, &nr) || nr.Reason != c.want {
			t.Errorf("%s: want %s, got %v", c.name, c.want, err)
		}
	}
}

func TestRetreaverPlainGet(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("retreaver-mp3"))
	}))
	defer srv.Close()
	src := NewRetreaverSource(fakeRetreaver{info: adapter.RetreaverRecordingInfo{Connected: true, URL: srv.URL + "/callpixels/abc.mp3"}})
	a, err := src.Fetch(context.Background(), Call{ExternalCallID: "uuid", SyncMode: "real"})
	if err != nil || string(a.Bytes) != "retreaver-mp3" || auth != "" {
		t.Fatalf("got %q err=%v auth=%q", a.Bytes, err, auth)
	}
}

func TestDownloadStatusClasses(t *testing.T) {
	for status, want := range map[int]string{500: "transient", 429: "transient", 404: NoRecFetchFailed, 403: NoRecNoAccess} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		_, err := download(context.Background(), noRedirectClient(5*time.Second), srv.URL+"/x.mp3", "", "")
		srv.Close()
		var te *TransientError
		var nr *NoRecordingError
		got := ""
		switch {
		case errors.As(err, &te):
			got = "transient"
		case errors.As(err, &nr):
			got = nr.Reason
		}
		if got != want {
			t.Errorf("HTTP %d: want %s, got %v", status, want, err)
		}
	}
}

func TestSafeErrNeverShowsTheURL(t *testing.T) {
	secret := "https://callpixels.s3.amazonaws.com/recordings/SECRETPATH.mp3?X-Amz-Signature=SIG"
	err := &url.Error{Op: "Get", URL: secret, Err: errors.New("dial tcp: connection refused")}
	wrapped := fmt.Errorf("fetch: %w (%s)", err, secret)
	for _, e := range []error{err, wrapped} {
		if s := safeErr(e); strings.Contains(s, "SECRETPATH") || strings.Contains(s, "SIG") || strings.Contains(s, "amazonaws") {
			t.Fatalf("URL leaked: %s", s)
		}
	}
	// A real failed fetch: the error text must not carry the link either.
	_, ferr := download(context.Background(), noRedirectClient(time.Second), "http://127.0.0.1:1/SECRETPATH.mp3", "", "")
	if ferr == nil || strings.Contains(ferr.Error(), "SECRETPATH") {
		t.Fatalf("fetch error leaks the URL: %v", ferr)
	}
}

func TestMockSourceIsDeterministicPerCall(t *testing.T) {
	a, _ := MockSource{}.Fetch(context.Background(), Call{ExternalCallID: "apisync-1"})
	b, _ := MockSource{}.Fetch(context.Background(), Call{ExternalCallID: "apisync-1"})
	c, _ := MockSource{}.Fetch(context.Background(), Call{ExternalCallID: "apisync-2"})
	if a.SHA256() != b.SHA256() || a.SHA256() == c.SHA256() || !strings.HasPrefix(string(a.Bytes), "RIFF") {
		t.Fatal("mock audio must be a WAV, the same per call and different between calls")
	}
}
