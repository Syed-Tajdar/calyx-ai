package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Fake Ringba Get Call Log and fake Retreaver call API: httptest servers, no real platform.

func TestRingbaLookupRecordingNarrowWindow(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/acct-1/calllogs") || r.Header.Get("Authorization") != "Token tok" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"isSuccessful":true,"report":{"records":[
			{"inboundCallId":"OTHER","recordingUrl":"https://media.ringba.com/recording-public?v=other"},
			{"inboundCallId":"RGB123","recordingUrl":"https://media.ringba.com/recording-public?v=v1&k=SECRET","hasRecording":true,"hasConnected":true,"wasBlocked":false}
		]}}`))
	}))
	defer srv.Close()
	t.Setenv("RINGBA_BASE_URL", srv.URL)
	around := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	info, err := NewRingbaAdapter(NewHTTPClient()).LookupRecording(context.Background(),
		map[string]string{"api_token": "tok", "account_id": "acct-1"}, "RGB123", around)
	if err != nil {
		t.Fatal(err)
	}
	if !info.HasRecording || !info.HasConnected || info.WasBlocked || !strings.Contains(info.URL, "v=v1") {
		t.Fatalf("info %+v", info)
	}
	if body["reportStart"] != "2026-10-21T11:00:00Z" || body["reportEnd"] != "2026-10-21T17:00:00Z" {
		t.Fatalf("window must be ±3h around the call, got %v → %v", body["reportStart"], body["reportEnd"])
	}
	cols, _ := json.Marshal(body["valueColumns"])
	if !strings.Contains(string(cols), "recordingUrl") {
		t.Fatalf("recordingUrl not requested: %s", cols)
	}
}

func TestRingbaLookupRecordingNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"isSuccessful":true,"report":{"records":[]}}`))
	}))
	defer srv.Close()
	t.Setenv("RINGBA_BASE_URL", srv.URL)
	_, err := NewRingbaAdapter(NewHTTPClient()).LookupRecording(context.Background(),
		map[string]string{"api_token": "tok", "account_id": "acct-1"}, "RGB404", time.Now())
	if !errors.Is(err, ErrRecordingCallNotFound) {
		t.Fatalf("want ErrRecordingCallNotFound, got %v", err)
	}
}

func TestRetreaverRecordingLink(t *testing.T) {
	const uuid = "a82a7a80-f1fd-46a5-a018-379dd7893ea7"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/calls/"+uuid+".json" || r.URL.Query().Get("api_key") != "k" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"call":{"uuid":"` + uuid + `","status":"completed","connected":true,"recording_url":"https://callpixels.s3.amazonaws.com/x.mp3"}}`))
	}))
	defer srv.Close()
	c := NewRetreaverCallClientWith(RetreaverCallClientOptions{Origin: srv.URL, Limiters: NewRetreaverAccountLimiters()})
	info, err := c.RecordingLink(context.Background(), map[string]string{"api_key": "k", "company_id": "c"}, uuid)
	if err != nil || info.URL != "https://callpixels.s3.amazonaws.com/x.mp3" || !info.Connected || info.Blocked {
		t.Fatalf("info %+v err %v", info, err)
	}
	if _, err := c.RecordingLink(context.Background(), map[string]string{"api_key": "k", "company_id": "c"}, "not-a-uuid"); err == nil {
		t.Fatal("a non-UUID call id must be refused before any request")
	}
}
