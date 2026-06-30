package connectors

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestRetreaverBuildPing_SetsCallKey(t *testing.T) {
	values := url.Values{
		"call_uuid": {"uuid-abc"},
		"call_key":  {"secret-key-xyz"},
		"caller":    {"15551234567"},
		"state":     {"CA"},
		"event":     {"incoming"},
		"revenue":   {"9.75"},
	}
	req, err := http.NewRequest(http.MethodGet, "/webhooks/retreaver/token?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	var c RetreaverConnector
	ping, err := c.BuildPing("tenant-1", req, nil, testEncryptionKey(), time.Now().UTC())
	if err != nil {
		t.Fatalf("BuildPing: %v", err)
	}
	if ping.CallKey != "secret-key-xyz" {
		t.Fatalf("CallKey: got %q want %q", ping.CallKey, "secret-key-xyz")
	}
	if ping.ExternalCallID != "uuid-abc" {
		t.Fatalf("ExternalCallID: got %q", ping.ExternalCallID)
	}
}

func TestRetreaverBuildPing_NoCallKey(t *testing.T) {
	values := url.Values{
		"call_uuid": {"uuid-abc"},
		"caller":    {"15551234567"},
		"state":     {"CA"},
		"event":     {"incoming"},
	}
	req, err := http.NewRequest(http.MethodGet, "/webhooks/retreaver/token?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	var c RetreaverConnector
	ping, err := c.BuildPing("tenant-1", req, nil, testEncryptionKey(), time.Now().UTC())
	if err != nil {
		t.Fatalf("BuildPing: %v", err)
	}
	if ping.CallKey != "" {
		t.Fatalf("CallKey: expected empty, got %q", ping.CallKey)
	}
}

func testEncryptionKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}
