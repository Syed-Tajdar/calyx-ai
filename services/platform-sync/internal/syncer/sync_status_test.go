package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Stage 4 R1-fix F1, revised by v1 sync health (migration 000034). Every failed run writes
// `degraded` first — an incomplete window and an unreachable platform alike — and the text in
// last_sync_error tells them apart. Repeated failures become `failed`.

func failRunStatus(t *testing.T, err error) (string, string) {
	t.Helper()
	item := models.SyncIntegration{ID: "int-1", Platform: "retreaver", SyncMode: "real"}
	st := newFakeStore(item)
	svc := &Service{store: st, logger: slog.Default()}
	svc.failRun(context.Background(), context.Background(), item, err, "incremental")
	r := st.row("int-1")
	if r.lastError == nil {
		t.Fatal("a failed run must write last_sync_error")
	}
	return r.status, *r.lastError
}

func TestShortfallIsRecordedWithItsCounts(t *testing.T) {
	end := time.Now().UTC()
	status, msg := failRunStatus(t, &adapter.WindowShortfallError{
		Platform:  "retreaver",
		Start:     end.Add(-time.Hour),
		End:       end,
		Collected: 40,
		Total:     45,
		Attempts:  3,
	})
	if status != syncStatusDegraded {
		t.Fatalf("a short window must write %q, got %q", syncStatusDegraded, status)
	}
	if !strings.Contains(msg, "40 of 45") {
		t.Fatalf("the shortfall counts must reach last_sync_error, got %q", msg)
	}
}

func TestWrappedShortfallIsStillRecognised(t *testing.T) {
	// The syncer wraps adapter errors as it unwinds; errors.As must still find the shortfall.
	_, msg := failRunStatus(t, fmt.Errorf("sync tick failed: %w", &adapter.WindowShortfallError{
		Platform: "retreaver", Collected: 1, Total: 2, Attempts: 3,
	}))
	if !strings.Contains(msg, "1 of 2") {
		t.Fatalf("a wrapped shortfall must still be described, got %q", msg)
	}
}

func TestOrdinaryFailureIsDegradedWithItsCategory(t *testing.T) {
	status, msg := failRunStatus(t, errors.New("dial tcp: no such host"))
	if status != syncStatusDegraded {
		t.Fatalf("a transport failure must write %q, got %q", syncStatusDegraded, status)
	}
	if !strings.HasPrefix(msg, "network error") {
		t.Fatalf("a transport failure must be labelled as one, got %q", msg)
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
