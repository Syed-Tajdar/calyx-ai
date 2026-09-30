package syncer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
)

// Stage 4 R1-fix F1 — an incomplete window and an unreachable platform are different failures
// and must not write the same sync_status.
//
// `error` says the call failed. `degraded` says the calls succeeded but the window did not add
// up, so the cursor is being held deliberately. Collapsing the two would either hide a stuck
// sync behind a transport error or cry wolf about an outage that never happened.

func TestShortfallIsDegradedNotError(t *testing.T) {
	end := time.Now().UTC()
	shortfall := &adapter.WindowShortfallError{
		Platform:  "retreaver",
		Start:     end.Add(-time.Hour),
		End:       end,
		Collected: 40,
		Total:     45,
		Attempts:  3,
	}

	status, detail := classifySyncFailure(shortfall)
	if status != syncStatusDegraded {
		t.Fatalf("a short window must write %q, got %q", syncStatusDegraded, status)
	}
	if detail == nil || detail.Total != 45 || detail.Collected != 40 {
		t.Fatalf("the shortfall detail must survive classification, got %+v", detail)
	}
}

func TestWrappedShortfallIsStillDegraded(t *testing.T) {
	// The syncer wraps adapter errors as it unwinds; errors.As must still find the shortfall.
	wrapped := fmt.Errorf("sync tick failed: %w", &adapter.WindowShortfallError{
		Platform: "retreaver", Collected: 1, Total: 2, Attempts: 3,
	})
	if status, _ := classifySyncFailure(wrapped); status != syncStatusDegraded {
		t.Fatalf("a wrapped shortfall must still be %q, got %q", syncStatusDegraded, status)
	}
}

func TestOrdinaryFailureIsError(t *testing.T) {
	status, detail := classifySyncFailure(errors.New("dial tcp: no such host"))
	if status != syncStatusError {
		t.Fatalf("a transport failure must write %q, got %q", syncStatusError, status)
	}
	if detail != nil {
		t.Fatalf("a transport failure carries no shortfall detail, got %+v", detail)
	}
}

// The message written to last_sync_error must name the window and both counts, because that is
// all the owner sees in Settings when an integration goes degraded.
func TestDegradedMessageNamesTheWindowAndCounts(t *testing.T) {
	start := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	msg := (&adapter.WindowShortfallError{
		Platform: "retreaver", Start: start, End: end,
		Collected: 40, Total: 45, Attempts: 3,
	}).Error()

	for _, want := range []string{
		"2026-09-30T10:00:00Z", "2026-09-30T11:00:00Z",
		"40 of 45", "3 attempts", "cursor held",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("degraded message must mention %q, got: %s", want, msg)
		}
	}
}
