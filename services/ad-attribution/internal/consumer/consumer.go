package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/dispatcher"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/matcher"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/platforms"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/providerconfig"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/publisher"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/pkg/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

// Service consumes conversion events and fires ad platform conversions.
type Service struct {
	cfg        config.Config
	reader     *kafka.Reader
	ch         *clickhouse.Client
	dedup      *dedup.Checker
	dispatcher *dispatcher.Service
	publisher  *publisher.Service
	resolver   *providerconfig.Client
	logger     *slog.Logger
}

// NewService creates the attribution consumer.
func NewService(
	cfg config.Config,
	reader *kafka.Reader,
	ch *clickhouse.Client,
	dedupChecker *dedup.Checker,
	dispatch *dispatcher.Service,
	pub *publisher.Service,
	resolver *providerconfig.Client,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg:        cfg,
		reader:     reader,
		ch:         ch,
		dedup:      dedupChecker,
		dispatcher: dispatch,
		publisher:  pub,
		resolver:   resolver,
		logger:     logger,
	}
}

// Run blocks until context cancelled.
func (s *Service) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		msg, err := s.reader.Fetch(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		if err := s.handleMessage(ctx, msg); err != nil {
			s.logger.Error("attribution handle failed", "service", "ad-attribution", "error", err)
		}
		if err := s.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("consumer.Run commit: %w", err)
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var evt models.ConversionEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return nil
	}
	if evt.TenantID == "" || evt.CallID == "" || evt.EventID == "" {
		return nil
	}

	processed, err := s.dedup.AlreadyProcessed(ctx, evt.TenantID, evt.EventID)
	if err != nil {
		return err
	}
	if processed {
		return nil
	}

	platformName := matcher.ResolvePlatform(evt)
	if s.resolver != nil {
		resolveCtx, resolveCancel := context.WithTimeout(ctx, 3*time.Second)
		resolved, resolveErr := s.resolver.Resolve(resolveCtx, evt.TenantID, evt.CallID)
		resolveCancel()
		if resolveErr == nil && resolved.MockMode {
			platformName = "mock"
		} else if resolveErr == nil && resolved.ProviderID != "" && resolved.ProviderID != "mock" {
			platformName = matcher.ResolvePlatform(models.ConversionEvent{
				PlatformHint: resolved.ProviderID,
				ClickID:      evt.ClickID,
				CallID:       evt.CallID,
			})
		}
	}

	platform := platforms.Registry(platformName, s.logger)
	phoneHash := matcher.HashPhoneSHA256(evt.ANIHash)
	fireReq := models.FireRequest{
		TenantID:  evt.TenantID,
		CallID:    evt.CallID,
		EventID:   evt.EventID,
		Platform:  platformName,
		ClickID:   evt.ClickID,
		Revenue:   evt.Revenue,
		PhoneHash: phoneHash,
	}

	fireCtx, fireCancel := context.WithTimeout(ctx, 15*time.Second)
	defer fireCancel()
	res, err := s.dispatcher.Fire(fireCtx, platform, fireReq)
	if err != nil {
		res.Error = err.Error()
		res.Success = false
	}

	if writeErr := s.ch.InsertFiredEvent(ctx, evt, platformName, res); writeErr != nil {
		_ = s.ch.InsertDLQ(ctx, evt, "clickhouse_write", writeErr.Error(), payloadHash(msg.Value))
		_ = s.publisher.PublishDLQ(ctx, evt, "clickhouse_write", writeErr.Error())
		return writeErr
	}

	_ = s.publisher.PublishAttributed(ctx, evt, platformName, res)
	if !res.Success {
		_ = s.ch.InsertDLQ(ctx, evt, "platform_fire", res.Error, payloadHash(msg.Value))
		_ = s.publisher.PublishDLQ(ctx, evt, "platform_fire", res.Error)
	}

	if err := s.dedup.MarkProcessed(ctx, evt.TenantID, evt.EventID); err != nil {
		return err
	}

	s.logger.Info("attribution fired",
		"service", "ad-attribution",
		"tenant_id", evt.TenantID,
		"call_id", evt.CallID,
		"platform", platformName,
		"success", res.Success,
		"http_status", res.HTTPStatus,
	)
	return nil
}

func payloadHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
