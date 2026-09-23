package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/cache"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/providers"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/pkg/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

// Service consumes pings and enriches before bidding.
type Service struct {
	cfg      config.Config
	reader   *kafka.Reader
	ch       *clickhouse.Client
	cache    *cache.Store
	provider providers.Provider
	logger   *slog.Logger
}

// NewService creates enrichment consumer.
func NewService(cfg config.Config, reader *kafka.Reader, ch *clickhouse.Client, cacheStore *cache.Store, provider providers.Provider, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, reader: reader, ch: ch, cache: cacheStore, provider: provider, logger: logger}
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
			s.logger.Error("enrichment handle failed", "service", "enrichment", "error", err)
		}
		if err := s.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("consumer.Run commit: %w", err)
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var ping models.PingMessage
	if err := json.Unmarshal(msg.Value, &ping); err != nil {
		return nil
	}
	if ping.IsDuplicate || ping.ANIHash == "" || ping.TenantID == "" {
		return nil
	}

	cached, err := s.cache.IsCached(ctx, ping.TenantID, ping.ANIHash)
	if err != nil {
		return err
	}
	if cached {
		aniLog := ping.ANIHash
		if len(aniLog) > 8 {
			aniLog = aniLog[:8]
		}
		s.logger.Info("enrichment cache hit", "service", "enrichment", "tenant_id", ping.TenantID, "ani_hash", aniLog)
		return nil
	}

	// Intentionally no HasAttributes skip: enrichment_attrs is ReplacingMergeTree(created_at).
	// Re-enrichment must be allowed so real providers (Chapter 2) can update values.
	// Duplicates collapse on merge; reads use GROUP BY + argMax(created_at).

	attrs, err := s.provider.Enrich(ping.ANIHash)
	if err != nil {
		_ = s.ch.InsertDLQ(ctx, ping.TenantID, ping.ANIHash, "ping", err.Error(), payloadHash(msg.Value))
		return err
	}

	if err := s.ch.InsertAttributes(ctx, ping.TenantID, ping.ANIHash, s.cfg.EnrichmentProvider, attrs); err != nil {
		_ = s.ch.InsertDLQ(ctx, ping.TenantID, ping.ANIHash, "ping", err.Error(), payloadHash(msg.Value))
		return err
	}

	if err := s.cache.MarkCached(ctx, ping.TenantID, ping.ANIHash); err != nil {
		return err
	}

	aniLog := ping.ANIHash
	if len(aniLog) > 8 {
		aniLog = aniLog[:8]
	}
	s.logger.Info("enrichment complete",
		"service", "enrichment",
		"tenant_id", ping.TenantID,
		"ani_hash", aniLog,
		"attribute_count", len(attrs),
	)
	return nil
}

func payloadHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
