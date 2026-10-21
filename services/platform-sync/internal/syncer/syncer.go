package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
)

type pingPublisher interface {
	Publish(ctx context.Context, tenantID, platform string, call models.RemoteCall, isDuplicate bool, receivedAt int64) error
}

type contentDeduper interface {
	Evaluate(ctx context.Context, tenantID, platform string, content dedup.Content) (dedup.Decision, error)
	Commit(ctx context.Context, tenantID, platform string, content dedup.Content) error
}

// integrationStore is the part of store.IntegrationStore the syncer uses. An interface so the
// failure paths can be tested without a database.
type integrationStore interface {
	ListAPISync(ctx context.Context) ([]models.SyncIntegration, error)
	GetAPISync(ctx context.Context, tenantID, integrationID string) (*models.SyncIntegration, error)
	UpdateSyncStatus(ctx context.Context, id, status string, errMsg *string) error
	UpdateSyncState(ctx context.Context, id, cursor string, syncAt time.Time, status string, errMsg *string) error
	RecordSyncFailure(ctx context.Context, id, errMsg string, failedAfter int) (string, error)
}

type Service struct {
	store     integrationStore
	adapter   *adapter.Registry
	producer  pingPublisher
	dedup     contentDeduper
	decryptor *crypto.CredentialDecryptor
	logger    *slog.Logger
	lock      SyncLocker
}

func NewService(
	st *store.IntegrationStore,
	ad *adapter.Registry,
	prod *producer.Producer,
	d *dedup.Checker,
	dec *crypto.CredentialDecryptor,
	logger *slog.Logger,
	lock SyncLocker,
) *Service {
	return &Service{
		store: st, adapter: ad, producer: prod, dedup: d,
		decryptor: dec, logger: logger, lock: lock,
	}
}

func (s *Service) RunAll(ctx context.Context) error {
	items, err := s.store.ListAPISync(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := s.syncOne(ctx, item, false); err != nil {
			if errors.Is(err, ErrAlreadyRunning) {
				holder, _, _ := s.peekHolder(ctx, item.ID)
				s.logger.Info("api sync skipped — already running",
					"integration_id", item.ID, "platform", item.Platform,
					"lock_holder", holder)
				continue
			}
			s.logger.Error("sync failed",
				"integration_id", item.ID, "platform", item.Platform, "error", redact.SanitizeErr(err))
		}
	}
	return nil
}

// RunAllSweeps re-fetches the hourly sweep window for every API-sync integration whose adapter
// supports windowed fetch (Stage 4 R1 / D — was Ringba-only).
func (s *Service) RunAllSweeps(ctx context.Context) error {
	return s.runSweeps(ctx, "sweep")
}

// RunAllDeepSweeps re-fetches the deep (dispute-window) sweep for the same set.
func (s *Service) RunAllDeepSweeps(ctx context.Context) error {
	return s.runSweeps(ctx, "deep_sweep")
}

// sweepDaysFor returns the platform's sweep window. The tiers are the same shape for every
// platform (hourly/7d, daily/30d) but each keeps its own env vars so they can be tuned per
// vendor — Retreaver and Ringba have very different rate budgets.
func sweepDaysFor(platform, reason string) int {
	deep := reason == "deep_sweep"
	if strings.EqualFold(platform, "retreaver") {
		if deep {
			return adapter.RetreaverDeepSweepDays()
		}
		return adapter.RetreaverSweepDays()
	}
	if deep {
		return adapter.RingbaDeepSweepDays()
	}
	return adapter.RingbaSweepDays()
}

func (s *Service) runSweeps(ctx context.Context, reason string) error {
	items, err := s.store.ListAPISync(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, item := range items {
		// Capability, not a platform name: any adapter that can fetch an explicit window gets
		// sweeps. TrackDrive and CallGrid do not implement RangeFetcher and are skipped.
		if !s.adapter.SupportsRange(item.Platform, item.SyncMode) {
			continue
		}
		start, end := adapter.BuildRingbaSweepWindow(now, sweepDaysFor(item.Platform, reason))
		if err := s.RunWindow(ctx, item.TenantID, item.ID, start, end, reason); err != nil {
			if errors.Is(err, ErrAlreadyRunning) {
				holder, _, _ := s.peekHolder(ctx, item.ID)
				s.logger.Info("ringba sweep skipped — already running",
					"integration_id", item.ID, "reason", reason, "lock_holder", holder)
				continue
			}
			s.logger.Error("ringba sweep failed",
				"integration_id", item.ID, "reason", reason, "error", redact.SanitizeErr(err))
		}
	}
	return nil
}

func (s *Service) peekHolder(ctx context.Context, integrationID string) (string, bool, error) {
	if s.lock == nil {
		return "", false, nil
	}
	return s.lock.Peek(ctx, integrationID)
}

func (s *Service) RunOne(ctx context.Context, tenantID, integrationID string) error {
	item, err := s.store.GetAPISync(ctx, tenantID, integrationID)
	if err != nil {
		return fmt.Errorf("api_sync integration not found")
	}
	return s.syncOne(ctx, *item, true)
}

// RunWindow fetches an explicit [start, end] through the windowed path (sweep / backfill).
// Does not advance last_sync_at. Uses the same single-flight lock as ticks.
func (s *Service) RunWindow(ctx context.Context, tenantID, integrationID string, start, end time.Time, reason string) error {
	item, err := s.store.GetAPISync(ctx, tenantID, integrationID)
	if err != nil {
		return fmt.Errorf("api_sync integration not found")
	}
	if !s.adapter.SupportsRange(item.Platform, item.SyncMode) {
		return fmt.Errorf("windowed sync (sweeps/backfill) is not supported for %s", item.Platform)
	}
	return s.runWindowLocked(ctx, *item, start, end, reason)
}

func (s *Service) TestAuth(ctx context.Context, tenantID, integrationID string) error {
	item, err := s.store.GetAPISync(ctx, tenantID, integrationID)
	if err != nil {
		return fmt.Errorf("api_sync integration not found")
	}
	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		return fmt.Errorf("Re-enter credentials")
	}
	return s.adapter.TestAuth(ctx, item.Platform, item.SyncMode, creds)
}

// withLock acquires the per-integration lock, starts a renew heartbeat, and returns a
// child context cancelled when ownership is lost (or parent cancelled). Caller must
// defer release(). Two runners must never publish at once — lost ownership cancels runCtx,
// and context.Cause(runCtx) then says why (lockLostError).
func (s *Service) withLock(ctx context.Context, integrationID, reason string) (runCtx context.Context, release func(), err error) {
	if s.lock == nil {
		return ctx, func() {}, nil
	}
	token, ok, err := s.lock.TryAcquire(ctx, integrationID, reason)
	if err != nil {
		return nil, nil, fmt.Errorf("sync lock: %w", err)
	}
	if !ok {
		return nil, nil, ErrAlreadyRunning
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	stopHB := make(chan struct{})
	go s.lockHeartbeat(runCtx, stopHB, integrationID, token, cancel)

	released := false
	release = func() {
		if released {
			return
		}
		released = true
		close(stopHB)
		cancel(nil)
		if relErr := s.lock.Release(context.Background(), integrationID, token); relErr != nil {
			s.logger.Error("sync lock release failed",
				"integration_id", integrationID, "error", redact.SanitizeErr(relErr))
		} else {
			s.logger.Info("sync lock released",
				"integration_id", integrationID, "reason", reason)
		}
	}
	return runCtx, release, nil
}

// lockLostError is the cancel cause when the heartbeat loses the lock mid-run. The run was
// cut short by redis, not by a shutdown, so it is recorded as a sync failure.
type lockLostError struct{ msg string }

func (e *lockLostError) Error() string { return e.msg }

func (s *Service) lockHeartbeat(runCtx context.Context, stop <-chan struct{}, integrationID, token string, cancel context.CancelCauseFunc) {
	ticker := time.NewTicker(syncLockRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-runCtx.Done():
			return
		case <-ticker.C:
			// Renew uses Background so a parent cancel mid-Eval still clears ownership check.
			ok, err := s.lock.Renew(context.Background(), integrationID, token)
			if err != nil {
				s.logger.Error("sync lock renew failed — cancelling run",
					"integration_id", integrationID, "error", redact.SanitizeErr(err))
				cancel(&lockLostError{msg: "sync lock renew failed: " + err.Error()})
				return
			}
			if !ok {
				s.logger.Error("sync lock ownership lost — cancelling run",
					"integration_id", integrationID)
				cancel(&lockLostError{msg: "sync lock ownership lost mid-run"})
				return
			}
			pttlMs := int64(-1)
			if peeker, ok := s.lock.(interface {
				PTTL(ctx context.Context, integrationID string) (time.Duration, error)
			}); ok {
				if d, err := peeker.PTTL(context.Background(), integrationID); err == nil {
					pttlMs = d.Milliseconds()
				}
			}
			s.logger.Info("sync lock renewed",
				"integration_id", integrationID,
				"pttl_ms", pttlMs,
				"ttl", syncLockTTL.String())
		}
	}
}

// manual=true → ErrAlreadyRunning on lock miss (HTTP 409). manual=false → skip via same error for RunAll.
func demoSyncAllowed() bool {
	if os.Getenv("APP_ENV") == "production" {
		return false
	}
	v := os.Getenv("DEMO_MODE_ENABLED")
	return v != "false" && v != "0"
}

func (s *Service) syncOne(ctx context.Context, item models.SyncIntegration, manual bool) error {
	if item.SyncMode != "real" && !demoSyncAllowed() {
		if s.logger != nil {
			s.logger.Info("demo sync skipped",
				"integration_id", item.ID, "platform", item.Platform,
				"sync_mode", item.SyncMode, "reason", "demo mode off")
		}
		return nil
	}
	runCtx, release, err := s.withLock(ctx, item.ID, "tick")
	if err != nil {
		// A lock that cannot be taken is a failed sync, not a skipped one: from 2026-10-18 to
		// 2026-10-21 every tick failed right here and Settings still showed `ok`.
		if !errors.Is(err, ErrAlreadyRunning) {
			s.recordFailure(item, lockFailureText(err))
		}
		return err
	}
	defer release()

	// Status-only: do not advance last_sync_at on start (S1A A1).
	_ = s.store.UpdateSyncStatus(runCtx, item.ID, "syncing", nil)

	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		s.recordFailure(item, "authentication error: re-enter credentials")
		return fmt.Errorf("Re-enter credentials")
	}
	creds["platform"] = item.Platform

	lastSync := ""
	if item.LastSyncAt != nil {
		lastSync = item.LastSyncAt.UTC().Format(time.RFC3339)
	}

	lookbackDays := item.InitialLookbackDays
	if lookbackDays < 1 {
		lookbackDays = 30
	}
	result, err := s.adapter.FetchAllPages(runCtx, item.Platform, item.SyncMode, creds, item.SyncCursor, lastSync, lookbackDays)
	if err != nil {
		// F1: an incomplete window is logged with its counts. The cursor stays put either way,
		// because UpdateSyncState is only reached on success.
		s.failRun(ctx, runCtx, item, err, "incremental")
		return err
	}

	if !result.WindowStart.IsZero() || !result.WindowEnd.IsZero() {
		s.logger.Info("api sync window",
			"integration_id", item.ID, "platform", item.Platform,
			"window_start", result.WindowStart.UTC().Format(time.RFC3339),
			"window_end", result.WindowEnd.UTC().Format(time.RFC3339),
			"window_capped", result.WindowCapped,
			"manual", manual)
	}

	receivedAt := time.Now().UTC().UnixMilli()
	skippedEmptyID, published, unchanged, publishErr := publishSyncCalls(runCtx, item.TenantID, item.Platform, result.Calls, s.dedup, s.producer, receivedAt)
	if publishErr != nil {
		s.failRun(ctx, runCtx, item, publishErr, "incremental")
		return publishErr
	}

	if skippedEmptyID > 0 {
		errMsg := fmt.Sprintf("%d calls skipped: missing external id", skippedEmptyID)
		s.recordFailure(item, redact.ClassifySyncError(fmt.Errorf("%s", errMsg)))
		return fmt.Errorf("%s", errMsg)
	}

	// Cancelled/lost-lock runs must not advance last_sync_at (S1A). A lost lock is a redis
	// failure and is recorded; a shutdown is not.
	if runCtx.Err() != nil {
		s.logger.Info("api sync aborted before cursor advance",
			"integration_id", item.ID, "error", context.Cause(runCtx))
		s.failRun(ctx, runCtx, item, runCtx.Err(), "incremental")
		return runCtx.Err()
	}

	nextCursor := result.NextCursor
	if nextCursor == "" {
		nextCursor = item.SyncCursor
	}
	// last_sync_at = window end actually fetched, only after full success (A1).
	syncAt := result.WindowEnd
	if syncAt.IsZero() {
		syncAt = time.Now().UTC()
	}
	if err := s.store.UpdateSyncState(runCtx, item.ID, nextCursor, syncAt, "ok", nil); err != nil {
		return err
	}
	s.logger.Info("api sync completed",
		"integration_id", item.ID, "platform", item.Platform,
		"manual", manual,
		"window_start", result.WindowStart.UTC().Format(time.RFC3339),
		"window_end", result.WindowEnd.UTC().Format(time.RFC3339),
		"window_capped", result.WindowCapped,
		"calls", len(result.Calls),
		"published", published,
		"unchanged", unchanged,
		"page1_refresh", result.Page1Refresh,
		"requests", result.RequestCount,
		"offset_fallback_windows", result.OffsetFallbackWindows,
		"offset_fallback_rows", result.OffsetFallbackRows)
	s.logRetreaverC3(item)
	return nil
}

// logRetreaverC3 surfaces any break in the C3 correlation that made the Nullable(Decimal)
// migration unnecessary (F3). Silent while the assumption holds; loud the moment it does not,
// because from that point a stored 0.00 is ambiguous and the owner has a decision to make.
func (s *Service) logRetreaverC3(item models.SyncIntegration) {
	if !strings.EqualFold(item.Platform, "retreaver") {
		return
	}
	checked, convertedNoRevenue, revenueNoConversion, convertedZero := adapter.RetreaverC3Snapshot()
	if convertedNoRevenue+revenueNoConversion+convertedZero == 0 {
		return
	}
	s.logger.Warn("retreaver C3 assumption violated — revisit the Nullable(Decimal) migration",
		"integration_id", item.ID,
		"calls_checked", checked,
		"converted_without_revenue", convertedNoRevenue,
		"revenue_without_conversion", revenueNoConversion,
		"converted_zero_revenue", convertedZero)
}

func (s *Service) runWindowLocked(ctx context.Context, item models.SyncIntegration, start, end time.Time, reason string) error {
	lockReason := reason
	if lockReason == "" {
		lockReason = "window"
	}
	runCtx, release, err := s.withLock(ctx, item.ID, lockReason)
	if err != nil {
		if !errors.Is(err, ErrAlreadyRunning) {
			s.recordFailure(item, lockReason+": "+lockFailureText(err))
		}
		return err
	}
	defer release()

	began := time.Now().UTC()
	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		s.recordFailure(item, lockReason+": authentication error: re-enter credentials")
		return fmt.Errorf("Re-enter credentials")
	}
	creds["platform"] = item.Platform

	result, err := s.adapter.FetchRange(runCtx, item.Platform, item.SyncMode, creds, start, end)
	if err != nil {
		// F1: the RangeFetcher path (sweeps and backfill) follows the same rule as the
		// incremental one. Sweeps never move last_sync_at, so holding the cursor is implicit.
		s.failRun(ctx, runCtx, item, err, lockReason)
		return err
	}

	receivedAt := time.Now().UTC().UnixMilli()
	skippedEmptyID, published, unchanged, publishErr := publishSyncCalls(runCtx, item.TenantID, item.Platform, result.Calls, s.dedup, s.producer, receivedAt)
	if publishErr != nil {
		s.failRun(ctx, runCtx, item, publishErr, lockReason)
		return publishErr
	}
	if skippedEmptyID > 0 {
		errMsg := fmt.Sprintf("%d calls skipped: missing external id", skippedEmptyID)
		s.recordFailure(item, lockReason+": "+redact.ClassifySyncError(fmt.Errorf("%s", errMsg)))
		return fmt.Errorf("%s", errMsg)
	}
	if lostLock(ctx, runCtx) {
		s.failRun(ctx, runCtx, item, runCtx.Err(), lockReason)
		return context.Cause(runCtx)
	}

	// Sweeps/backfills must not touch last_sync_at (incremental ticks only).
	platformName := item.Platform
	if platformName == "" {
		platformName = "unknown"
	}
	s.logger.Info(platformName+" window fetch completed",
		"platform", platformName,
		"integration_id", item.ID,
		"reason", reason,
		"window_start", start.UTC().Format(time.RFC3339),
		"window_end", end.UTC().Format(time.RFC3339),
		"requests", result.RequestCount,
		"calls", len(result.Calls),
		"published", published,
		"unchanged", unchanged,
		"duration_ms", time.Since(began).Milliseconds(),
		"last_sync_at_unchanged", true,
		"cancelled", runCtx.Err() != nil)
	return nil
}

func remoteCallContent(call models.RemoteCall) dedup.Content {
	return dedup.Content{
		ExternalCallID:             call.ExternalCallID,
		Revenue:                    call.Revenue,
		Payout:                     call.Payout,
		State:                      call.State,
		EventType:                  call.EventType,
		OccurredAt:                 call.OccurredAt,
		CampaignID:                 call.CampaignID,
		PublisherID:                call.PublisherID,
		TargetID:                   call.TargetID,
		PlatformDuplicate:          call.PlatformDuplicate,
		CampaignName:               call.CampaignName,
		PublisherName:              call.PublisherName,
		TargetName:                 call.TargetName,
		BuyerID:                    call.BuyerID,
		BuyerName:                  call.BuyerName,
		DurationSeconds:            call.DurationSeconds,
		CallLengthSeconds:          call.CallLengthSeconds,
		HasRecording:               call.HasRecording,
		ProfitGross:                call.ProfitGross,
		HasConverted:               call.HasConverted,
		HasConnected:               call.HasConnected,
		WasBlocked:                 call.WasBlocked,
		BlockReason:                call.BlockReason,
		WasConversionAdjusted:      call.WasConversionAdjusted,
		WasPayoutAdjusted:          call.WasPayoutAdjusted,
		RequiredDurationSeconds:    call.RequiredDurationSeconds,
		MaxRequiredDurationSeconds: call.MaxRequiredDurationSeconds,
	}
}

func publishSyncCalls(
	ctx context.Context,
	tenantID, platform string,
	calls []models.RemoteCall,
	dedupChecker contentDeduper,
	pub pingPublisher,
	receivedAt int64,
) (skippedEmptyID, published, unchanged int, err error) {
	for _, call := range calls {
		if strings.TrimSpace(call.ExternalCallID) == "" {
			skippedEmptyID++
			continue
		}
		content := remoteCallContent(call)
		decision, err := dedupChecker.Evaluate(ctx, tenantID, platform, content)
		if err != nil {
			return skippedEmptyID, published, unchanged, err
		}
		if decision == dedup.Unchanged {
			unchanged++
			continue
		}
		if err := pub.Publish(ctx, tenantID, platform, call, false, receivedAt); err != nil {
			return skippedEmptyID, published, unchanged, err
		}
		// F029: hash only after successful publish. Commit failure → next run retries (safe duplicate).
		if err := dedupChecker.Commit(ctx, tenantID, platform, content); err != nil {
			// Fail-open: do not abort the sync; leave hash absent so the call is retried.
			_ = err
		}
		published++
	}
	return skippedEmptyID, published, unchanged, nil
}

// Sync statuses a failed run ends in (migration 000034). Every failure — lock, redis, DNS,
// network, auth, API, an incomplete window — writes `degraded`; syncFailedAfter() consecutive
// failures write `failed`. The next successful incremental sync writes `ok`, clears
// last_sync_error, resets the count and moves last_success_at. (`error` is the pre-000034
// status and is no longer written.)
const (
	syncStatusDegraded = "degraded"
	syncStatusFailed   = "failed"
)

// syncFailedAfter — `SYNC_FAILED_AFTER` (default 3): consecutive failures before `failed`.
// With the 120 s tick that is about 6 minutes of failing syncs.
func syncFailedAfter() int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("SYNC_FAILED_AFTER"))); err == nil && n >= 1 {
		return n
	}
	return 3
}

// statusWriteTimeout bounds the failure write. It uses its own context: the run's context is
// often the thing that failed (lost lock), and the outage still has to reach the database.
const statusWriteTimeout = 5 * time.Second

// recordFailure writes one failure for the integration: short, redacted text in last_sync_error
// and `degraded` or `failed` in sync_status. last_sync_at, the cursor and last_success_at stay.
func (s *Service) recordFailure(item models.SyncIntegration, msg string) {
	if s.store == nil {
		return
	}
	text := redact.SyncErrorText(msg)
	ctx, cancel := context.WithTimeout(context.Background(), statusWriteTimeout)
	defer cancel()
	status, err := s.store.RecordSyncFailure(ctx, item.ID, text, syncFailedAfter())
	if err != nil {
		s.logger.Error("sync failure could not be recorded",
			"integration_id", item.ID, "platform", item.Platform, "error", redact.SanitizeErr(err))
		return
	}
	s.logger.Warn("sync failure recorded",
		"integration_id", item.ID, "platform", item.Platform,
		"sync_status", status, "last_sync_error", text)
}

// failRun records a failed run with the right reason. A shutdown (the parent context was
// cancelled) is not a sync failure and writes nothing. A lost lock is reported as such, not as
// the "context canceled" it surfaces as. An incomplete window (F1) is logged with its counts.
func (s *Service) failRun(parent, runCtx context.Context, item models.SyncIntegration, err error, path string) {
	if parent.Err() != nil {
		return
	}
	if lostLock(parent, runCtx) {
		s.recordFailure(item, failurePrefix(path)+"redis unavailable (sync lock): "+context.Cause(runCtx).Error())
		return
	}
	if shortfall := adapter.AsWindowShortfall(err); shortfall != nil {
		s.logger.Warn("api sync window incomplete — cursor held, integration degraded",
			"integration_id", item.ID,
			"platform", item.Platform,
			"path", path,
			"window_start", shortfall.Start.UTC().Format(time.RFC3339),
			"window_end", shortfall.End.UTC().Format(time.RFC3339),
			"total", shortfall.Total,
			"collected", shortfall.Collected,
			"attempts", shortfall.Attempts)
		s.recordFailure(item, failurePrefix(path)+shortfall.Error())
		return
	}
	s.recordFailure(item, failurePrefix(path)+redact.ClassifySyncError(err))
}

// failurePrefix names a sweep or backfill in last_sync_error; incremental ticks need no prefix.
func failurePrefix(path string) string {
	if path == "" || path == "incremental" || path == "tick" {
		return ""
	}
	return path + ": "
}

// lostLock reports whether runCtx was cancelled by the lock heartbeat (not by the parent).
func lostLock(parent, runCtx context.Context) bool {
	if parent.Err() != nil || runCtx.Err() == nil {
		return false
	}
	var lost *lockLostError
	return errors.As(context.Cause(runCtx), &lost)
}

// lockFailureText is last_sync_error for a lock that could not be taken. The lock lives in
// redis, so this is how a redis or DNS outage shows up in Settings.
func lockFailureText(err error) string {
	return "redis unavailable (sync lock): " + strings.TrimPrefix(redact.SanitizeErr(err), "sync lock: ")
}
