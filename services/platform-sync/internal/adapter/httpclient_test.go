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
