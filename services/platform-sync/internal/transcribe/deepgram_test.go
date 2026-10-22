package transcribe

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Fake Deepgram: an httptest server that records the request and answers like /v1/listen.

const fakeListenBody = `{
  "metadata": {"request_id": "req-0001", "duration": 74.2, "models": ["m1"],
               "model_info": {"m1": {"name": "general", "version": "2025-01-01", "arch": "nova-3"}}},
  "results": {
    "channels": [{"detected_language": "en", "alternatives": [{"transcript": "…"}]}],
    "utterances": [
      {"start": 0.1, "end": 2.4, "speaker": 0, "transcript": "Thank you for calling, how can I help?"},
      {"start": 2.6, "end": 9.9, "speaker": 1, "transcript": "My card is 4111 1111 1111 1111 and my SSN is 123-45-6789."}
    ]
  }
}`

type capturedListen struct {
	query url.Values
	body  []byte
	auth  string
	ctype string
	hits  int
}

func fakeDeepgram(t *testing.T, status int, body string) (*Deepgram, *capturedListen) {
	c := &capturedListen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.hits++
		c.query = r.URL.Query()
		c.body, _ = io.ReadAll(r.Body)
		c.auth = r.Header.Get("Authorization")
		c.ctype = r.Header.Get("Content-Type")
		if r.URL.Path != "/v1/listen" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewDeepgram(srv.URL, "fake-deepgram-key"), c
}

func TestDeepgramEnglishRequest(t *testing.T) {
	dg, c := fakeDeepgram(t, 200, fakeListenBody)
	audio := Audio{Bytes: []byte("the-exact-audio-bytes"), ContentType: "audio/mpeg"}
	res, err := dg.Transcribe(context.Background(), audio, TranscribeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(c.body) != "the-exact-audio-bytes" || c.ctype != "audio/mpeg" || c.auth != "Token fake-deepgram-key" {
		t.Fatalf("request: body=%q ctype=%q auth=%q", c.body, c.ctype, c.auth)
	}
	want := map[string]string{"model": "nova-3", "mip_opt_out": "true", "detect_language": "true", "utterances": "true",
		"smart_format": "true", "punctuate": "true", "diarize_model": "latest"}
	for k, v := range want {
		if c.query.Get(k) != v {
			t.Errorf("%s=%q, want %q", k, c.query.Get(k), v)
		}
	}
	if got := strings.Join(c.query["redact"], ","); got != "pci,ssn" {
		t.Errorf("redact=%q, want pci,ssn", got)
	}
	if c.query.Get("language") != "" || c.query.Get("url") != "" {
		t.Error("English requests detect the language; no URL is ever sent")
	}
	if res.RequestID != "req-0001" || res.Duration != 74.2 || res.Language != "en" || len(res.Utterances) != 2 ||
		res.Utterances[1].Speaker != 1 || res.Utterances[1].StartMs != 2600 || res.Model == "" || res.Redaction != "deepgram:pci,ssn" {
		t.Fatalf("parsed %+v", res)
	}
}

func TestDeepgramSpanishRequest(t *testing.T) {
	dg, c := fakeDeepgram(t, 200, fakeListenBody)
	res, err := dg.Transcribe(context.Background(), Audio{Bytes: []byte("a"), ContentType: "audio/mpeg"}, TranscribeOptions{Spanish: true})
	if err != nil {
		t.Fatal(err)
	}
	if c.query.Get("language") != "multi" || strings.Join(c.query["redact"], ",") != "numbers" ||
		c.query.Get("detect_language") != "" || c.query.Get("mip_opt_out") != "true" || res.Redaction != "deepgram:numbers" {
		t.Fatalf("spanish query %v / %s", c.query, res.Redaction)
	}
}

func TestMipOptOutOnEveryRequest(t *testing.T) {
	for _, spanish := range []bool{false, true} {
		q, _ := ListenQuery(TranscribeOptions{Spanish: spanish})
		if q.Get("mip_opt_out") != "true" {
			t.Fatalf("mip_opt_out missing (spanish=%v)", spanish)
		}
	}
}

func TestDeepgramErrorClasses(t *testing.T) {
	dg, _ := fakeDeepgram(t, 429, `{}`)
	_, err := dg.Transcribe(context.Background(), Audio{Bytes: []byte("a"), ContentType: "audio/mpeg"}, TranscribeOptions{})
	var te *TransientError
	if !errors.As(err, &te) {
		t.Fatalf("429 must be transient, got %v", err)
	}
	dg, _ = fakeDeepgram(t, 400, `{"err_msg":"Bad Request: token=fake-deepgram-key is invalid"}`)
	_, err = dg.Transcribe(context.Background(), Audio{Bytes: []byte("a"), ContentType: "audio/mpeg"}, TranscribeOptions{})
	var fe *FinalError
	if !errors.As(err, &fe) || strings.Contains(err.Error(), "fake-deepgram-key") {
		t.Fatalf("400 must be final and redacted, got %v", err)
	}
	if _, err := NewDeepgram("http://127.0.0.1:1", "").Transcribe(context.Background(), Audio{}, TranscribeOptions{}); !errors.As(err, &fe) {
		t.Fatalf("no key must be final, got %v", err)
	}
}
