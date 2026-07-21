package adapter

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
)

func TestHTTPClientDoRedactsTransportErrorURL(t *testing.T) {
	client := NewHTTPClient()
	client.client = &http.Client{Timeout: 5 * time.Second}

	rawURL := "https://this-host-does-not-exist-xyz.invalid/api/v2/calls.json?api_key=BOGUS_KEY_ABC123&company_id=999999"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = client.Do(req)
	if err == nil {
		t.Fatal("expected DNS/transport error")
	}

	msg := err.Error()
	if strings.Contains(msg, "BOGUS_KEY_ABC123") {
		t.Fatalf("secret leaked in Do error: %s", msg)
	}
	if !strings.Contains(msg, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker in Do error: %s", msg)
	}
	if !strings.Contains(msg, "this-host-does-not-exist-xyz.invalid") {
		t.Fatalf("expected host preserved for debugging: %s", msg)
	}

	classified := redact.ClassifySyncError(err)
	if !strings.HasPrefix(classified, "network error (transient, will retry):") {
		t.Fatalf("expected network prefix: %s", classified)
	}
	if strings.Contains(classified, "BOGUS_KEY_ABC123") {
		t.Fatalf("secret leaked in classified error: %s", classified)
	}
}

func TestFormatHTTPStatusErrorIncludesFullBodyAndLocation(t *testing.T) {
	body := strings.Repeat("x", 250) + " END"
	hdr := http.Header{}
	hdr.Set("Location", "https://developers.ringba.com/docs/new-calllogs-endpoint")
	hdr.Set("Content-Type", "application/json")
	out := formatHTTPStatusError(301, []byte(body), hdr, "ringba")
	if !strings.Contains(out, "ringba HTTP 301") {
		t.Fatalf("missing status: %s", out)
	}
	if !strings.Contains(out, "Location=") {
		t.Fatalf("expected Location header: %s", out)
	}
	if !strings.Contains(out, "developers.ringba.com") {
		t.Fatalf("expected Location URL: %s", out)
	}
	if !strings.Contains(out, " END") || !strings.Contains(out, strings.Repeat("x", 250)) {
		t.Fatalf("body truncated: len=%d out_len=%d", len(body), len(out))
	}
}

func TestHTTPClientDoesNotFollowRedirects(t *testing.T) {
	client := NewHTTPClient()
	if client.client.CheckRedirect == nil {
		t.Fatal("expected CheckRedirect to disable auto-follow")
	}
	err := client.client.CheckRedirect(nil, nil)
	if err != http.ErrUseLastResponse {
		t.Fatalf("want ErrUseLastResponse, got %v", err)
	}
}

// Header-auth tokens never appear in Go transport errors (URL only). REDACTED=0 on
// that path is expected — do not inject Authorization into errors to force a marker.
func TestHTTPClientDoTransportErrorOmitsAuthorizationHeader(t *testing.T) {
	client := NewHTTPClient()
	client.client = &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://this-host-does-not-exist-xyz.invalid/999999/calllogs", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "BOGUS_RINGBA_TOKEN_XYZ")

	_, _, err = client.Do(req)
	if err == nil {
		t.Fatal("expected DNS/transport error")
	}
	msg := err.Error()
	if strings.Contains(msg, "BOGUS_RINGBA_TOKEN_XYZ") {
		t.Fatalf("Authorization token leaked: %s", msg)
	}
	if strings.Contains(strings.ToLower(msg), "authorization:") {
		t.Fatalf("must not inject Authorization into transport errors: %s", msg)
	}
}
