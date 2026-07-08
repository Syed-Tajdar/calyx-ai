package syncer



import (

	"context"

	"fmt"

	"log/slog"

	"time"



	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"

)



type Service struct {

	store     *store.IntegrationStore

	adapter   *adapter.Registry

	producer  *producer.Producer

	dedup     *dedup.Checker

	decryptor *crypto.CredentialDecryptor

	logger    *slog.Logger

}



func NewService(

	st *store.IntegrationStore,

	ad *adapter.Registry,

	prod *producer.Producer,

	d *dedup.Checker,

	dec *crypto.CredentialDecryptor,

	logger *slog.Logger,

) *Service {

	return &Service{

		store: st, adapter: ad, producer: prod, dedup: d,

		decryptor: dec, logger: logger,

	}

}



func (s *Service) RunAll(ctx context.Context) error {

	items, err := s.store.ListAPISync(ctx)

	if err != nil {

		return err

	}

	for _, item := range items {

		if err := s.syncOne(ctx, item); err != nil {

			s.logger.Error("sync failed",

				"integration_id", item.ID, "platform", item.Platform, "error", err)

		}

	}

	return nil

}



func (s *Service) RunOne(ctx context.Context, tenantID, integrationID string) error {

	item, err := s.store.GetAPISync(ctx, tenantID, integrationID)

	if err != nil {

		return fmt.Errorf("api_sync integration not found")

	}

	return s.syncOne(ctx, *item)

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



func (s *Service) syncOne(ctx context.Context, item models.SyncIntegration) error {

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



	result, err := s.adapter.FetchAllPages(ctx, item.Platform, item.SyncMode, creds, item.SyncCursor, lastSync)

	if err != nil {

		s.markError(ctx, item.ID, item.SyncCursor, err.Error())

		return err

	}



	now := time.Now().UTC()

	for _, call := range result.Calls {

		isDup, err := s.dedup.IsDuplicate(ctx, item.TenantID, item.Platform, call.ExternalCallID)

		if err != nil {

			s.markError(ctx, item.ID, item.SyncCursor, err.Error())

			return err

		}

		if err := s.producer.Publish(ctx, item.TenantID, item.Platform, call, isDup, now.UnixMilli()); err != nil {

			s.markError(ctx, item.ID, item.SyncCursor, err.Error())

			return err

		}

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

		"calls", len(result.Calls))

	return nil

}



func (s *Service) markError(ctx context.Context, id, cursor, msg string) {

	errMsg := msg

	_ = s.store.UpdateSyncState(ctx, id, cursor, time.Now().UTC(), "error", &errMsg)

}


