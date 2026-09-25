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

// RunAllSweeps re-fetches the hourly sweep window for every Ringba API-sync integration.
func (s *Service) RunAllSweeps(ctx context.Context) error {
	return s.runRingbaSweeps(ctx, adapter.RingbaSweepDays(), "sweep")
}

// RunAllDeepSweeps re-fetches the deep (dispute-window) sweep for every Ringba API-sync integration.
func (s *Service) RunAllDeepSweeps(ctx context.Context) error {
	return s.runRingbaSweeps(ctx, adapter.RingbaDeepSweepDays(), "deep_sweep")
}

func (s *Service) runRingbaSweeps(ctx context.Context, days int, reason string) error {
	items, err := s.store.ListAPISync(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	start, end := adapter.BuildRingbaSweepWindow(now, days)
	for _, item := range items {
		if !strings.EqualFold(item.Platform, "ringba") {
			continue
		}
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
	if !strings.EqualFold(item.Platform, "ringba") {
		return fmt.Errorf("RunWindow only supported for ringba")
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

// manual=true → ErrAlreadyRunning on lock miss (HTTP 409). manual=false → skip via same error for RunAll.
func (s *Service) syncOne(ctx context.Context, item models.SyncIntegration, manual bool) error {
	if s.lock != nil {
		token, ok, err := s.lock.TryAcquire(ctx, item.ID, "tick")
		if err != nil {
			return fmt.Errorf("sync lock: %w", err)
		}
		if !ok {
			return ErrAlreadyRunning
		}
		defer func() {
			if relErr := s.lock.Release(context.Background(), item.ID, token); relErr != nil {
				s.logger.Error("sync lock release failed",
					"integration_id", item.ID, "error", redact.SanitizeErr(relErr))
			}
		}()
	}

	// Status-only: do not advance last_sync_at on start (S1A A1).
	_ = s.store.UpdateSyncStatus(ctx, item.ID, "syncing", nil)

	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		s.markError(ctx, item.ID, "decrypt credentials failed")
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
	result, err := s.adapter.FetchAllPages(ctx, item.Platform, item.SyncMode, creds, item.SyncCursor, lastSync, lookbackDays)
	if err != nil {
		s.markError(ctx, item.ID, redact.ClassifySyncError(err))
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
	skippedEmptyID, published, unchanged, publishErr := publishSyncCalls(ctx, item.TenantID, item.Platform, result.Calls, s.dedup, s.producer, receivedAt)
	if publishErr != nil {
		s.markError(ctx, item.ID, redact.SanitizeErr(publishErr))
		return publishErr
	}

	if skippedEmptyID > 0 {
		errMsg := fmt.Sprintf("%d calls skipped: missing external id", skippedEmptyID)
		s.markError(ctx, item.ID, redact.ClassifySyncError(fmt.Errorf(errMsg)))
		return fmt.Errorf(errMsg)
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
	if err := s.store.UpdateSyncState(ctx, item.ID, nextCursor, syncAt, "ok", nil); err != nil {
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
	return nil
}

func (s *Service) runWindowLocked(ctx context.Context, item models.SyncIntegration, start, end time.Time, reason string) error {
	lockReason := reason
	if lockReason == "" {
		lockReason = "window"
	}
	if s.lock != nil {
		token, ok, err := s.lock.TryAcquire(ctx, item.ID, lockReason)
		if err != nil {
			return fmt.Errorf("sync lock: %w", err)
		}
		if !ok {
			return ErrAlreadyRunning
		}
		defer func() {
			if relErr := s.lock.Release(context.Background(), item.ID, token); relErr != nil {
				s.logger.Error("sync lock release failed",
					"integration_id", item.ID, "error", redact.SanitizeErr(relErr))
			}
		}()
	}

	began := time.Now().UTC()
	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		return fmt.Errorf("decrypt credentials: %w", err)
	}
	creds["platform"] = item.Platform

	result, err := s.adapter.FetchRange(ctx, item.Platform, item.SyncMode, creds, start, end)
	if err != nil {
		return err
	}

	receivedAt := time.Now().UTC().UnixMilli()
	skippedEmptyID, published, unchanged, publishErr := publishSyncCalls(ctx, item.TenantID, item.Platform, result.Calls, s.dedup, s.producer, receivedAt)
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
		"last_sync_at_unchanged", true)
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
	_ = s.store.UpdateSyncStatus(ctx, id, "error", &errMsg)
}
