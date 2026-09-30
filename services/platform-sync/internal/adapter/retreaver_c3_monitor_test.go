package adapter

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Stage 4 R1-fix F3 — one test per way the C3 correlation can break.
//
// C3 lets Retreaver's `null` revenue be stored as 0.00 because, across a 100-call sample,
// `revenue = null` happened on exactly the unconverted calls and never as a literal 0. Mapping
// is only lossless while that holds, so each violation must be counted and logged.

// retreaverCallJSON builds a minimal Retreaver call payload. `converted` and `revenue` are
// pointers so a missing key is distinguishable from false / zero — the distinction C3 rests on.
func retreaverCallJSON(t *testing.T, uuid string, converted *bool, revenue *float64) json.RawMessage {
	t.Helper()
	call := map[string]any{
		"uuid":         uuid,
		"caller_state": "TX",
		"created_at":   "2026-09-30T10:00:00.000Z",
		"updated_at":   "2026-09-30T10:00:05.000Z",
	}
	if converted != nil {
		call["converted"] = *converted
	}
	if revenue != nil {
		call["revenue"] = *revenue
	}
	b, err := json.Marshal(map[string]any{"call": call})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	return b
}

func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float64) *float64 { return &f }

// The healthy shapes: converted with money, and unconverted with no money key at all.
func TestC3HoldingCallsRaiseNoViolation(t *testing.T) {
	ResetRetreaverC3()

	mapRetreaverCall(retreaverCallJSON(t, "aaaaaaaa-conv", boolPtr(true), floatPtr(37.50)))
	mapRetreaverCall(retreaverCallJSON(t, "bbbbbbbb-unco", boolPtr(false), nil))

	checked, noRev, revNoConv, zeroConv := RetreaverC3Snapshot()
	if checked != 2 {
		t.Fatalf("expected 2 calls checked, got %d", checked)
	}
	if noRev+revNoConv+zeroConv != 0 {
		t.Fatalf("the C3-conforming sample must raise no violation, got %d/%d/%d",
			noRev, revNoConv, zeroConv)
	}
}

// Violation 1 — converted=true with revenue null. The stored 0.00 now means two things.
func TestC3ConvertedWithoutRevenueIsCounted(t *testing.T) {
	ResetRetreaverC3()

	call := mapRetreaverCall(retreaverCallJSON(t, "cccccccc-lost", boolPtr(true), nil))
	if !call.HasConverted {
		t.Fatalf("fixture must map as converted")
	}
	if call.Revenue != 0 {
		t.Fatalf("a null revenue still maps to 0.00, got %v", call.Revenue)
	}

	_, noRev, _, _ := RetreaverC3Snapshot()
	if noRev != 1 {
		t.Fatalf("expected 1 converted-without-revenue violation, got %d", noRev)
	}
}

// Violation 2 — converted=false with a revenue value present.
func TestC3RevenueWithoutConversionIsCounted(t *testing.T) {
	ResetRetreaverC3()

	mapRetreaverCall(retreaverCallJSON(t, "dddddddd-paid", boolPtr(false), floatPtr(12.25)))

	_, _, revNoConv, _ := RetreaverC3Snapshot()
	if revNoConv != 1 {
		t.Fatalf("expected 1 revenue-without-conversion violation, got %d", revNoConv)
	}
}

// Violation 3 — converted=true with revenue exactly 0. This is the single case the null → 0.00
// mapping cannot express, because it collides with "not converted, no money".
func TestC3ConvertedZeroRevenueIsCounted(t *testing.T) {
	ResetRetreaverC3()

	mapRetreaverCall(retreaverCallJSON(t, "eeeeeeee-zero", boolPtr(true), floatPtr(0)))

	_, _, _, zeroConv := RetreaverC3Snapshot()
	if zeroConv != 1 {
		t.Fatalf("expected 1 converted-zero-revenue violation, got %d", zeroConv)
	}
}

// The counters must accumulate across a page of calls, not just report the last one.
func TestC3ViolationsAccumulateAcrossAPage(t *testing.T) {
	ResetRetreaverC3()

	for i := 0; i < 3; i++ {
		mapRetreaverCall(retreaverCallJSON(t, fmt.Sprintf("ffffffff-%02d", i), boolPtr(true), nil))
	}
	for i := 0; i < 2; i++ {
		mapRetreaverCall(retreaverCallJSON(t, fmt.Sprintf("99999999-%02d", i), boolPtr(false), floatPtr(1)))
	}
	// Two conforming calls in the same page must not be counted as violations.
	mapRetreaverCall(retreaverCallJSON(t, "11111111-ok1", boolPtr(true), floatPtr(5)))
	mapRetreaverCall(retreaverCallJSON(t, "22222222-ok2", boolPtr(false), nil))

	checked, noRev, revNoConv, zeroConv := RetreaverC3Snapshot()
	if checked != 7 {
		t.Fatalf("expected 7 calls checked, got %d", checked)
	}
	if noRev != 3 || revNoConv != 2 || zeroConv != 0 {
		t.Fatalf("expected 3/2/0 violations, got %d/%d/%d", noRev, revNoConv, zeroConv)
	}
}

// A payload with no `converted` key is the "never reported" case, not a violation: it is what
// HasConvertedPresent records, and revenue is absent too.
func TestC3AbsentConvertedKeyIsNotAViolation(t *testing.T) {
	ResetRetreaverC3()

	mapRetreaverCall(retreaverCallJSON(t, "33333333-none", nil, nil))

	_, noRev, revNoConv, zeroConv := RetreaverC3Snapshot()
	if noRev+revNoConv+zeroConv != 0 {
		t.Fatalf("an absent converted key must not be a violation, got %d/%d/%d",
			noRev, revNoConv, zeroConv)
	}
}

// Rule 5 — the log identifier is a UUID prefix, never a full id and never a caller number.
func TestC3LogIdentifierIsAPrefixOnly(t *testing.T) {
	full := "a31df5bd-1234-4321-9999-0123456789ab"
	got := callUUIDPrefix(full)
	if got != "a31df5bd" {
		t.Fatalf("expected an 8-character prefix, got %q", got)
	}
	if len(got) >= len(full) {
		t.Fatalf("the prefix must be shorter than the full uuid")
	}
}
