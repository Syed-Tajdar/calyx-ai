package main

import (
	"net/http"
	"strings"
	"testing"
)

type failRT struct{ called bool }

func (f *failRT) RoundTrip(*http.Request) (*http.Response, error) {
	f.called = true
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

func TestGetOnlyTransportRefusesWrites(t *testing.T) {
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete} {
		next := &failRT{}
		req, _ := http.NewRequest(m, "https://api.ringba.com/v2/x/Publishers/y", strings.NewReader("{}"))
		if _, err := (getOnly{next}).RoundTrip(req); err == nil || next.called {
			t.Fatalf("%s was not refused", m)
		}
	}
	for _, u := range []string{"https://evil.example.com/x", "http://api.retreaver.com/x"} {
		next := &failRT{}
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		if _, err := (getOnly{next}).RoundTrip(req); err == nil || next.called {
			t.Fatalf("%s was not refused", u)
		}
	}
	for _, u := range []string{"https://api.ringba.com/v2/x/Publishers", "https://api.retreaver.com/api/v1/affiliates.json"} {
		next := &failRT{}
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		if _, err := (getOnly{next}).RoundTrip(req); err != nil || !next.called {
			t.Fatalf("%s GET refused: %v", u, err)
		}
	}
}
