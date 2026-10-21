package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

// Step 0 runs against the server's database before 2D is deployed: there the workflows migration
// (000035) is missing, and the probe must fall back instead of stopping.
func TestUndefinedFunctionIsRecognised(t *testing.T) {
	if !isUndefinedFunction(fmt.Errorf("query: %w", &pgconn.PgError{Code: "42883", Message: "function get_publisher_pause_integration(uuid, uuid) does not exist"})) {
		t.Fatal("SQLSTATE 42883 must trigger the fallback")
	}
	if isUndefinedFunction(&pgconn.PgError{Code: "42501"}) || isUndefinedFunction(errors.New("no rows in result set")) || isUndefinedFunction(nil) {
		t.Fatal("only 'function does not exist' may trigger the fallback")
	}
}
