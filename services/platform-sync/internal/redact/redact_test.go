package redact

import (
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeErrRedactsAPIKey(t *testing.T) {
	err := fmt.Errorf(`Get "https://api.retreaver.com/api/v2/calls.json?api_key=SECRET&company_id=999": dial tcp: lookup api.retreaver.com`)
	out := SanitizeErr(err)
	if strings.Contains(out, "SECRET") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "api_key=[REDACTED]") {
		t.Fatalf("expected api_key redaction, got: %s", out)
	}
	if !strings.Contains(out, "company_id=[REDACTED]") {
		t.Fatalf("expected company_id redaction, got: %s", out)
	}
}

func TestRedactURL(t *testing.T) {
	raw := "https://api.retreaver.com/api/v2/calls.json?api_key=abc&company_id=42&page=1"
	out := RedactURL(raw)
	if strings.Contains(out, "abc") {
		t.Fatalf("api_key not redacted: %s", out)
	}
	if !strings.Contains(out, "page=1") {
		t.Fatalf("non-secret param should remain: %s", out)
	}
}

func TestClassifySyncErrorNetwork(t *testing.T) {
	err := fmt.Errorf(`Get "https://api.example.com?api_key=x": dial tcp: lookup example.com: no such host`)
	out := ClassifySyncError(err)
	if !strings.HasPrefix(out, "network error") {
		t.Fatalf("expected network prefix, got: %s", out)
	}
	if strings.Contains(out, "api_key=x") {
		t.Fatalf("credential leaked in classified error")
	}
}

func TestClassifySyncErrorAuth(t *testing.T) {
	out := ClassifySyncError(fmt.Errorf("retreaver HTTP 401: unauthorized"))
	if !strings.HasPrefix(out, "authentication error") {
		t.Fatalf("expected auth prefix, got: %s", out)
	}
}

func TestSanitizeErrRedactsBearerHeader(t *testing.T) {
	err := fmt.Errorf(`request failed: Authorization: Bearer super-secret-token-xyz dial tcp timeout`)
	out := SanitizeErr(err)
	if strings.Contains(out, "super-secret-token-xyz") {
		t.Fatalf("bearer token leaked: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker: %s", out)
	}
}

func TestSanitizeErrRedactsBasicAuth(t *testing.T) {
	err := fmt.Errorf(`Authorization: Basic dXNlcjpwYXNz failed`)
	out := SanitizeErr(err)
	if strings.Contains(out, "dXNlcjpwYXNz") {
		t.Fatalf("basic auth leaked: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker: %s", out)
	}
}

func TestSanitizeErrRedactsBareAuthorizationToken(t *testing.T) {
	err := fmt.Errorf(`ringba request failed: Authorization: ringba-access-token-SECRET dial tcp timeout`)
	out := SanitizeErr(err)
	if strings.Contains(out, "ringba-access-token-SECRET") {
		t.Fatalf("api access token leaked: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker: %s", out)
	}
}

func TestSanitizeErrRedactsTokenAuthorizationHeader(t *testing.T) {
	err := fmt.Errorf(`ringba request failed: Authorization: Token ringba-access-token-SECRET dial tcp timeout`)
	out := SanitizeErr(err)
	if strings.Contains(out, "ringba-access-token-SECRET") {
		t.Fatalf("Token scheme api access token leaked: %s", out)
	}
	if !strings.Contains(out, "Authorization: Token [REDACTED]") {
		t.Fatalf("expected Authorization: Token [REDACTED], got: %s", out)
	}
}
