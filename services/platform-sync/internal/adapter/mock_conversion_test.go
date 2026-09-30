package adapter

import (
	"context"
	"testing"
)

// Stage 4 R1-fix F6 — Test-mode data must obey the rule the product enforces.
//
// The generator previously emitted revenue on every row with converted=false, which is the exact
// shape the "should have converted" badge exists to flag. 57 mock calls therefore showed
// "Review" in every demo — false alarms that look like a product bug rather than mock data.

func mockCallFor(t *testing.T, platform string) (revenue float64, converted, reported bool) {
	t.Helper()
	m := &mockAdapter{}
	res, err := m.FetchPage(context.Background(), map[string]string{"platform": platform}, "", "", 30)
	if err != nil {
		t.Fatalf("mock FetchPage(%s): %v", platform, err)
	}
	if len(res.Calls) != 1 {
		t.Fatalf("expected 1 mock call for %s, got %d", platform, len(res.Calls))
	}
	c := res.Calls[0]
	return c.Revenue, c.HasConverted, c.HasConvertedPresent
}

// The invariant, whichever side of the clock the tick lands on: money implies converted.
func TestMockRevenueOnlyOnConvertedCalls(t *testing.T) {
	for _, platform := range []string{"retreaver", "ringba"} {
		revenue, converted, reported := mockCallFor(t, platform)

		if !reported {
			t.Fatalf("%s reports conversion state, so the mock must mark it present", platform)
		}
		if revenue > 0 && !converted {
			t.Fatalf("%s: mock produced revenue %.2f on an unconverted call — this is the "+
				"false-positive shape F6 exists to remove", platform, revenue)
		}
		if converted && revenue <= 0 {
			t.Fatalf("%s: a converted mock call must carry revenue, got %.2f", platform, revenue)
		}
	}
}

// Platforms that never report conversion state must be left exactly as they were: their rows are
// already excluded from the badge by `reports_conversion_state` (C6), and inventing a conversion
// flag for them would contradict that.
func TestMockLeavesNonReportingPlatformsAlone(t *testing.T) {
	for _, platform := range []string{"trackdrive", "callgrid"} {
		revenue, converted, reported := mockCallFor(t, platform)

		if reported || converted {
			t.Fatalf("%s does not report conversion state; the mock must not claim it does "+
				"(converted=%v present=%v)", platform, converted, reported)
		}
		if revenue <= 0 {
			t.Fatalf("%s mock revenue is unchanged by F6 and must stay non-zero, got %.2f",
				platform, revenue)
		}
	}
}
