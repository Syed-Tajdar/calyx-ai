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
				s.logger.Info("api sync skipped — already running",
					"integration_id", item.ID, "platform", item.Platform)
				continue
			}
			s.logger.Error("sync failed",
				"integration_id", item.ID, "platform", item.Platform, "error", redact.SanitizeErr(err))
		}
	}
	return nil
}

func (s *Service) RunOne(ctx context.Context, tenantID, integrationID string) error {
	item, err := s.store.GetAPISync(ctx, tenantID, integrationID)
	if err != nil {
		return fmt.Errorf("api_sync integration not found")
	}
	return s.syncOne(ctx, *item, true)
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
		ok, err := s.lock.TryAcquire(ctx, item.ID)
		if err != nil {
			return fmt.Errorf("sync lock: %w", err)
		}
		if !ok {
			return ErrAlreadyRunning
		}
		defer func() {
			if relErr := s.lock.Release(context.Background(), item.ID); relErr != nil {
				s.logger.Error("sync lock release failed",
					"integration_id", item.ID, "error", redact.SanitizeErr(relErr))
			}
		}()
	}

	_ = s.store.UpdateSyncState(ctx, item.ID, item.SyncCursor, time.Now().UTC(), "syncing", nil)

	creds, err := s.decryptor.DecryptMap(item.APICredentialsBlob)
	if err != nil {
		s.markError(ctx, item.ID, item.SyncCursor, "decrypt credentials failed")
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
		s.markError(ctx, item.ID, item.SyncCursor, redact.ClassifySyncError(err))
		return err
	}

	now := time.Now().UTC()
	skippedEmptyID, published, unchanged, publishErr := publishSyncCalls(ctx, item.TenantID, item.Platform, result.Calls, s.dedup, s.producer, now.UnixMilli())
	if publishErr != nil {
		s.markError(ctx, item.ID, item.SyncCursor, redact.SanitizeErr(publishErr))
		return publishErr
	}

	if skippedEmptyID > 0 {
		errMsg := fmt.Sprintf("%d calls skipped: missing external id", skippedEmptyID)
		s.markError(ctx, item.ID, item.SyncCursor, redact.ClassifySyncError(fmt.Errorf(errMsg)))
		return fmt.Errorf(errMsg)
	}

	nextCursor := result.NextCursor
	if nextCursor == "" {
		nextCursor = item.SyncCursor
	}
	if err := s.store.UpdateSyncState(ctx, item.ID, nextCursor, now, "ok", nil); err != nil {
		return err
	}
	s.logger.Info("api sync completed",
		"integration_id", item.ID, "platform", item.Platform,
		"manual", manual,
		"calls", len(result.Calls),
		"published", published,
		"unchanged", unchanged,
		"page1_refresh", result.Page1Refresh,
		"requests", result.RequestCount,
		"offset_fallback_windows", result.OffsetFallbackWindows,
		"offset_fallback_rows", result.OffsetFallbackRows)
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
		decision, err := dedupChecker.Evaluate(ctx, tenantID, platform, remoteCallContent(call))
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
		published++
	}
	return skippedEmptyID, published, unchanged, nil
}

func (s *Service) markError(ctx context.Context, id, cursor, msg string) {
	errMsg := redact.RedactMessage(msg)
	_ = s.store.UpdateSyncState(ctx, id, cursor, time.Now().UTC(), "error", &errMsg)
}
