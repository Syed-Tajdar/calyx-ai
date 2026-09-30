package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

type Service struct {
	store     *store.IntegrationStore
	adapter   *adapter.Registry
	producer  pingPublisher
	dedup     *dedup.Checker
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
		return fmt.Errorf("decrypt credentials: %w", err)
	}
	return s.adapter.TestAuth(ctx, item.Platform, item.SyncMode, creds)
}

// withLock acquires the per-integration lock, starts a renew heartbeat, and returns a
// child context cancelled when ownership is lost (or parent cancelled). Caller must
// defer release(). Two runners must never publish at once — lost ownership cancels runCtx.
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
	runCtx, cancel := context.WithCancel(ctx)
	stopHB := make(chan struct{})
	go s.lockHeartbeat(runCtx, stopHB, integrationID, token, cancel)

	released := false
	release = func() {
		if released {
			return
		}
		released = true
		close(stopHB)
		cancel()
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

func (s *Service) lockHeartbeat(runCtx context.Context, stop <-chan struct{}, integrationID, token string, cancel context.CancelFunc) {
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
				cancel()
				return
			}
			if !ok {
				s.logger.Error("sync lock ownership lost — cancelling run",
					"integration_id", integrationID)
				cancel()
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
func (s *Service) syncOne(ctx context.Context, item models.SyncIntegration, manual bool) error {
	runCtx, release, err := s.withLock(ctx, item.ID, "tick")
	if err != nil {
		return err
	}
	defer release()

	// Status-only: do not advance last_sync_at on start (S1A A1).
	_ = s.store.UpdateSyncStatus(runCtx, item.ID, "syncing", nil)

	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		s.markError(runCtx, item.ID, "decrypt credentials failed")
		return fmt.Errorf("decrypt credentials: %w", err)
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
		// F1: an incomplete window is degraded, not error — and the cursor stays put either
		// way, because UpdateSyncState is only reached on success.
		if !s.handleWindowShortfall(runCtx, item, err, "incremental") {
			s.markError(runCtx, item.ID, redact.ClassifySyncError(err))
		}
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
		s.markError(runCtx, item.ID, redact.SanitizeErr(publishErr))
		return publishErr
	}

	if skippedEmptyID > 0 {
		errMsg := fmt.Sprintf("%d calls skipped: missing external id", skippedEmptyID)
		s.markError(runCtx, item.ID, redact.ClassifySyncError(fmt.Errorf("%s", errMsg)))
		return fmt.Errorf("%s", errMsg)
	}

	// Cancelled/lost-lock runs must not advance last_sync_at (S1A).
	if runCtx.Err() != nil {
		s.logger.Info("api sync aborted before cursor advance",
			"integration_id", item.ID, "error", runCtx.Err())
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
		return err
	}
	defer release()

	began := time.Now().UTC()
	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		return fmt.Errorf("decrypt credentials: %w", err)
	}
	creds["platform"] = item.Platform

	result, err := s.adapter.FetchRange(runCtx, item.Platform, item.SyncMode, creds, start, end)
	if err != nil {
		// F1: the RangeFetcher path (sweeps and backfill) follows the same rule as the
		// incremental one. Sweeps never move last_sync_at, so holding the cursor is implicit.
		s.handleWindowShortfall(runCtx, item, err, reason)
		return err
	}

	receivedAt := time.Now().UTC().UnixMilli()
	skippedEmptyID, published, unchanged, publishErr := publishSyncCalls(runCtx, item.TenantID, item.Platform, result.Calls, s.dedup, s.producer, receivedAt)
	if publishErr != nil {
		return publishErr
	}
	if skippedEmptyID > 0 {
		return fmt.Errorf("%d calls skipped: missing external id", skippedEmptyID)
	}

	// Sweeps/backfills must not touch last_sync_at (incremental ticks only).
	s.logger.Info("ringba window fetch completed",
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

func (s *Service) markError(ctx context.Context, id, msg string) {
	errMsg := redact.RedactMessage(msg)
	_ = s.store.UpdateSyncStatus(ctx, id, syncStatusError, &errMsg)
}

// markDegraded records an incomplete window (F1). Distinct from `error`: the API answered
// correctly, the rows did not add up. The cursor is left untouched by the caller, so the next
// tick re-walks the same window; a successful walk writes `ok` and clears this automatically.
func (s *Service) markDegraded(ctx context.Context, id, msg string) {
	errMsg := redact.RedactMessage(msg)
	_ = s.store.UpdateSyncStatus(ctx, id, syncStatusDegraded, &errMsg)
}

// Sync statuses a failed run can write. `degraded` is F1's: the requests succeeded but the
// window was incomplete, which is a different thing from the platform being unreachable.
const (
	syncStatusError    = "error"
	syncStatusDegraded = "degraded"
)

// classifySyncFailure decides which sync_status a failed run writes, and returns the shortfall
// detail when there is one. Pure so the routing can be tested without a database.
func classifySyncFailure(err error) (string, *adapter.WindowShortfallError) {
	if shortfall := adapter.AsWindowShortfall(err); shortfall != nil {
		return syncStatusDegraded, shortfall
	}
	return syncStatusError, nil
}

// handleWindowShortfall marks the integration degraded and logs the window that failed.
// Returns true when err was a shortfall and has been handled.
func (s *Service) handleWindowShortfall(ctx context.Context, item models.SyncIntegration, err error, path string) bool {
	status, shortfall := classifySyncFailure(err)
	if status != syncStatusDegraded {
		return false
	}
	s.logger.Warn("api sync window incomplete — cursor held, integration degraded",
		"integration_id", item.ID,
		"platform", item.Platform,
		"path", path,
		"window_start", shortfall.Start.UTC().Format(time.RFC3339),
		"window_end", shortfall.End.UTC().Format(time.RFC3339),
		"total", shortfall.Total,
		"collected", shortfall.Collected,
		"attempts", shortfall.Attempts)
	s.markDegraded(ctx, item.ID, shortfall.Error())
	return true
}
