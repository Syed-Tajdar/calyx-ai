package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
)

// Stage 4 R1 — B2(3): the Retreaver API key travels in the URL query string and cannot be moved
// to a header (B2(1): Authorization Bearer/Token, X-Api-Key and Api-Key all return 401; only
// `?api_key=` returns 200 — https://retreaver.github.io/core-api-docs/#authentication).
//
// The owner has decided the key will NOT be rotated (amendments A1), so a leak is permanent.
// Redaction is therefore the only defence, and these tests are its proof.
//
// Go's *url.Error embeds the full request URL, so any timeout / DNS / TLS failure carries the key.
// Retreaver's `Link` response header echoes the whole query string, so does every paged response.

const canaryKey = "CANARY_APIKEY_MUST_NEVER_APPEAR"
const canaryCompany = "CANARY_COMPANY_9999"

func retreaverCanaryCreds() map[string]string {
	return map[string]string{"api_key": canaryKey, "company_id": canaryCompany}
}

// assertNoCredential fails loudly without ever printing the offending text verbatim in a way
// that would re-leak a real key (the canary is synthetic, so naming it here is safe).
func assertNoCredential(t *testing.T, where, s string) {
	t.Helper()
	if strings.Contains(s, canaryKey) {
		t.Fatalf("%s leaked the api_key", where)
	}
	if strings.Contains(s, url.QueryEscape(canaryKey)) {
		t.Fatalf("%s leaked the url-encoded api_key", where)
	}
	if strings.Contains(s, canaryCompany) {
		t.Fatalf("%s leaked the company_id", where)
	}
}

// TestRetreaverTransportErrorDoesNotLeakCredentials forces a real transport failure
// (unresolvable host) and asserts the resulting error carries no credential.
func TestRetreaverTransportErrorDoesNotLeakCredentials(t *testing.T) {
	t.Setenv("RETREAVER_BASE_URL", "https://this-host-does-not-exist-calyx-r1.invalid/api/v2/calls.json")
	a := NewRetreaverAdapter(NewHTTPClient())

	_, err := a.FetchPage(context.Background(), retreaverCanaryCreds(), "", "", 30)
	if err == nil {
		t.Fatalf("expected a transport error from an unresolvable host")
	}
	assertNoCredential(t, "transport error", err.Error())
	t.Logf("transport error is clean [PROVEN]: %s", err.Error())
}

// TestRetreaverStatusErrorDoesNotLeakLinkHeader serves a non-2xx whose `Link` header echoes the
// full query string — exactly what the live API does on every paged response — and asserts the
// credential does not reach the error string.
func TestRetreaverStatusErrorDoesNotLeakLinkHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reproduce Retreaver's documented behaviour: Link echoes the request URL verbatim.
		echo := "https://api.retreaver.com/api/v2/calls.json?" + r.URL.RawQuery + "&page=2"
		w.Header().Set("Link", "<"+echo+`>; rel="next"`)
		w.Header().Set("Total", "500")
		w.WriteHeader(http.StatusBadRequest)
		// A body that also echoes the URL, as some APIs do on error.
		_, _ = w.Write([]byte(`{"error":"bad request for ` + echo + `"}`))
	}))
	defer srv.Close()

	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	a := NewRetreaverAdapter(NewHTTPClient())

	_, err := a.FetchPage(context.Background(), retreaverCanaryCreds(), "", "", 30)
	if err == nil {
		t.Fatalf("expected an error for HTTP 400")
	}
	assertNoCredential(t, "status error", err.Error())
}

// TestSafeResponseHeadersExcludeLink pins the allowlist: `Link` must never be logged, because for
// Retreaver it is credential-bearing. `Total` and `Per-Page` are safe and are used by the R1
// window logic.
func TestSafeResponseHeadersExcludeLink(t *testing.T) {
	for _, key := range safeHeaderKeys {
		if strings.EqualFold(key, "Link") {
			t.Fatalf("Link must not be in safeHeaderKeys — it echoes the api_key for Retreaver")
		}
	}

	hdr := http.Header{}
	hdr.Set("Link", "<https://api.retreaver.com/api/v2/calls.json?api_key="+canaryKey+">; rel=\"next\"")
	hdr.Set("Total", "500")
	hdr.Set("Content-Type", "application/json")

	out := formatSafeResponseHeaders(hdr)
	assertNoCredential(t, "formatSafeResponseHeaders", out)
	if strings.Contains(out, "Link") {
		t.Fatalf("Link header was formatted for logging")
	}
}

// TestRetreaverAuthErrorIsClassifiedWithoutCredentials covers the path that reaches the
// integration's stored last_error column.
func TestRetreaverAuthErrorIsClassifiedWithoutCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	t.Setenv("RETREAVER_BASE_URL", srv.URL)
	a := NewRetreaverAdapter(NewHTTPClient())

	_, err := a.FetchPage(context.Background(), retreaverCanaryCreds(), "", "", 30)
	if err == nil {
		t.Fatalf("expected an error for HTTP 401")
	}
	assertNoCredential(t, "raw error", err.Error())
	// last_sync_error is written from ClassifySyncError.
	assertNoCredential(t, "classified sync error", redact.ClassifySyncError(err))
}
