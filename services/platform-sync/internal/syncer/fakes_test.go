package syncer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// fakeStore mirrors the SQL of migration 000034 in memory: record_integration_sync_failure
// counts consecutive failures (degraded, then failed), update_integration_sync_state with `ok`
// resets the count, clears the error and moves last_success_at. Nothing else moves them.
type fakeStore struct {
	mu    sync.Mutex
	items map[string]*fakeRow
	now   func() time.Time
}

type fakeRow struct {
	item                models.SyncIntegration
	status              string
	lastError           *string
	consecutiveFailures int
	lastSuccessAt       *time.Time
	lastSyncAt          *time.Time
	cursor              string
}

func newFakeStore(items ...models.SyncIntegration) *fakeStore {
	st := &fakeStore{items: map[string]*fakeRow{}, now: time.Now}
	for _, it := range items {
		st.items[it.ID] = &fakeRow{item: it, status: "ok"}
	}
	return st
}

func (f *fakeStore) row(id string) *fakeRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.items[id]
	cp := *r
	return &cp
}

func (f *fakeStore) ListAPISync(context.Context) ([]models.SyncIntegration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.SyncIntegration
	for _, r := range f.items {
		out = append(out, r.item)
	}
	return out, nil
}

func (f *fakeStore) GetAPISync(_ context.Context, _, id string) (*models.SyncIntegration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.items[id]
	if !ok {
		return nil, errors.New("integration not found")
	}
	it := r.item
	return &it, nil
}

func (f *fakeStore) UpdateSyncStatus(_ context.Context, id, status string, errMsg *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.items[id]
	r.status = status
	r.lastError = errMsg
	return nil
}

func (f *fakeStore) UpdateSyncState(_ context.Context, id, cursor string, syncAt time.Time, status string, errMsg *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.items[id]
	r.cursor = cursor
	r.lastSyncAt = &syncAt
	r.status = status
	r.lastError = errMsg
	if status == "ok" {
		now := f.now()
		r.lastSuccessAt = &now
		r.consecutiveFailures = 0
	}
	return nil
}

func (f *fakeStore) RecordSyncFailure(_ context.Context, id, errMsg string, failedAfter int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.items[id]
	if !ok {
		return "", nil
	}
	r.consecutiveFailures++
	if failedAfter < 1 {
		failedAfter = 1
	}
	if r.consecutiveFailures >= failedAfter {
		r.status = syncStatusFailed
	} else {
		r.status = syncStatusDegraded
	}
	msg := errMsg
	r.lastError = &msg
	return r.status, nil
}

// failingLocker is a lock backend that is down: every call fails like redis did on 2026-10-18.
type failingLocker struct{ err error }

func (l failingLocker) TryAcquire(context.Context, string, string) (string, bool, error) {
	return "", false, l.err
}
func (l failingLocker) Renew(context.Context, string, string) (bool, error) { return false, l.err }
func (l failingLocker) Release(context.Context, string, string) error       { return l.err }
func (l failingLocker) Peek(context.Context, string) (string, bool, error)  { return "", false, l.err }
