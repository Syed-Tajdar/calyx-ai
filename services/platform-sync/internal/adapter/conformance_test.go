package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
)

// Certification lists — every registry adapter MUST appear in exactly one list.
// Each phase moves exactly one adapter from pending → certified.
var certifiedAdapters = []string{
	"retreaver",
	"ringba",
}

var pendingAdapters = []string{
	"trackdrive", // TODO Phase 3: map occurred_at + certify (occurred_at NOT mapped yet)
	"callgrid",   // TODO Phase 4: map occurred_at + certify (occurred_at NOT mapped yet)
}

// occurredAtRequired: certified adapters that must populate OccurredAt from platform timestamp.
var occurredAtRequired = map[string]bool{
	"retreaver": true,
	"ringba":    true,
}

func TestRegistryMembershipComplete(t *testing.T) {
	reg := NewRegistry()
	listed := map[string]string{}
	for _, p := range certifiedAdapters {
		if _, dup := listed[p]; dup {
			t.Fatalf("adapter %q listed twice", p)
		}
		listed[p] = "certified"
	}
	for _, p := range pendingAdapters {
		if prev, dup := listed[p]; dup {
			t.Fatalf("adapter %q already listed as %s", p, prev)
		}
		listed[p] = "pending"
	}
	for _, p := range reg.Platforms() {
		if _, ok := listed[p]; !ok {
			t.Fatalf("adapter %q registered but missing from certifiedAdapters and pendingAdapters", p)
		}
	}
	for p, status := range listed {
		found := false
		for _, name := range reg.Platforms() {
			if name == p {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("adapter %q listed as %s but not in registry", p, status)
		}
	}
}

func TestCertifiedAdapterConformance(t *testing.T) {
	if len(certifiedAdapters) == 0 {
		t.Log("no certified adapters yet — harness only (Phase 0)")
		return
	}
	for _, platform := range certifiedAdapters {
		platform := platform
		t.Run(platform, func(t *testing.T) {
			runAdapterConformance(t, platform)
		})
	}
}

func runAdapterConformance(t *testing.T, platform string) {
	t.Helper()
	fixture := loadGoldenFixture(t, platform)

	valid := mapFixtureCall(t, platform, fixture.ValidCall)
	if strings.TrimSpace(valid.ExternalCallID) == "" {
		t.Fatal("valid call mapped to empty external_call_id")
	}

	missing := mapFixtureCall(t, platform, fixture.MissingIDCall)
	if strings.TrimSpace(missing.ExternalCallID) != "" {
		t.Fatalf("missing-id call should map empty, got %q", missing.ExternalCallID)
	}
	// Mirrors syncer.publishSyncCalls: empty id → skip, never publish fake id.
	published := make([]models.RemoteCall, 0, 1)
	skipped := 0
	for _, call := range []models.RemoteCall{missing, valid} {
		if strings.TrimSpace(call.ExternalCallID) == "" {
			skipped++
			continue
		}
		published = append(published, call)
	}
	if skipped != 1 {
		t.Fatalf("skipped empty ids=%d want 1", skipped)
	}
	if len(published) != 1 || published[0].ExternalCallID != valid.ExternalCallID {
		t.Fatalf("expected only valid call kept, got %+v", published)
	}

	if occurredAtRequired[platform] && valid.OccurredAt.IsZero() {
		t.Fatal("occurred_at required but zero")
	}

	for _, tc := range []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"texas", fixture.StateTexas, "TX"},
		{"lower_tx", fixture.StateLowerTX, "TX"},
		{"unknown", fixture.StateUnknown, ""},
	} {
		if len(tc.raw) == 0 {
			continue
		}
		c := mapFixtureCall(t, platform, tc.raw)
		if c.State != tc.want {
			t.Fatalf("%s: state got %q want %q", tc.name, c.State, tc.want)
		}
	}

	if len(fixture.RevenueNull) > 0 {
		c := mapFixtureCall(t, platform, fixture.RevenueNull)
		if c.Revenue != 0 || c.Payout != 0 {
			t.Fatalf("null revenue/payout want 0, got %v/%v", c.Revenue, c.Payout)
		}
	}
	if len(fixture.RevenueNumber) > 0 {
		c := mapFixtureCall(t, platform, fixture.RevenueNumber)
		if c.Revenue <= 0 {
			t.Fatalf("expected positive revenue from number fixture, got %v", c.Revenue)
		}
	}

	if len(fixture.ExtraFields) > 0 {
		_ = mapFixtureCall(t, platform, fixture.ExtraFields)
	}

	secret := "cred-value-MUST-NOT-LEAK"
	for _, msg := range []string{
		fmt.Sprintf(`Get "https://api.example.com/v1?api_key=%s&company_id=1": dial tcp: lookup`, secret),
		fmt.Sprintf(`Authorization: Bearer %s dial tcp timeout`, secret),
		fmt.Sprintf(`Authorization: Basic %s failed`, secret),
		fmt.Sprintf(`Authorization: %s dial tcp timeout`, secret),
	} {
		out := redact.SanitizeErr(fmt.Errorf("%s", msg))
		if strings.Contains(out, secret) {
			t.Fatalf("credential leaked in error: %s", out)
		}
		if !strings.Contains(out, "[REDACTED]") {
			t.Fatalf("expected [REDACTED] in: %s", out)
		}
	}

	c1, err := EncodeCursor(map[string]interface{}{"page": 1, "ts": "2026-07-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	c2, err := EncodeCursor(map[string]interface{}{"page": 2, "ts": "2026-07-01T00:01:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if c1 == "" || c2 == "" || c1 == c2 {
		t.Fatalf("cursor must strictly advance: %q vs %q", c1, c2)
	}
}

type goldenFixture struct {
	ValidCall     json.RawMessage `json:"valid_call"`
	MissingIDCall json.RawMessage `json:"missing_id_call"`
	StateTexas    json.RawMessage `json:"state_texas"`
	StateLowerTX  json.RawMessage `json:"state_lower_tx"`
	StateUnknown  json.RawMessage `json:"state_unknown"`
	RevenueNull   json.RawMessage `json:"revenue_null"`
	RevenueNumber json.RawMessage `json:"revenue_number"`
	ExtraFields   json.RawMessage `json:"extra_fields"`
}

func loadGoldenFixture(t *testing.T, platform string) goldenFixture {
	t.Helper()
	path := filepath.Join("testdata", platform+"_calls.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	lines := strings.Split(string(raw), "\n")
	var body []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "//") {
			continue
		}
		body = append(body, line)
	}
	var fix goldenFixture
	if err := json.Unmarshal([]byte(strings.Join(body, "\n")), &fix); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	if len(fix.ValidCall) == 0 || len(fix.MissingIDCall) == 0 {
		t.Fatalf("fixture %s missing required keys", path)
	}
	return fix
}

func mapFixtureCall(t *testing.T, platform string, raw json.RawMessage) models.RemoteCall {
	t.Helper()
	switch platform {
	case "retreaver":
		return mapRetreaverCall(raw)
	case "ringba":
		return mapRingbaCallLogEntry(raw)
	case "trackdrive":
		return mapTrackDriveCall(raw)
	case "callgrid":
		return mapCallGridCall(raw)
	default:
		t.Fatalf("no mapper for platform %q", platform)
		return models.RemoteCall{}
	}
}
