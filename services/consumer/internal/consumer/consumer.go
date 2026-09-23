package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/writer"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/pkg/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

type pendingEntry struct {
	ping models.PingMessage
	msg  kafkago.Message
}

// Service consumes ping events and batches them to ClickHouse.
type Service struct {
	cfg         config.Config
	reader      *kafka.Reader
	writer      *writer.BatchWriter
	logger      *slog.Logger
	batch       []pendingEntry
	batchMu     sync.Mutex
	wg          sync.WaitGroup
	poisonCount atomic.Int64
}

// NewService creates a Kafka consumer service.
func NewService(cfg config.Config, reader *kafka.Reader, batchWriter *writer.BatchWriter, logger *slog.Logger) *Service {
	return &Service{
		cfg:    cfg,
		reader: reader,
		writer: batchWriter,
		logger: logger,
		batch:  make([]pendingEntry, 0, cfg.BatchSize),
	}
}

// PoisonCount returns how many messages were skipped as poison (insert failures).
func (s *Service) PoisonCount() int64 {
	return s.poisonCount.Load()
}

// Run starts the consumer loop and flush ticker. Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	s.wg.Add(1)
	go s.flushLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return s.finalFlush(context.Background())
		default:
		}

		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		msg, err := s.reader.Fetch(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				s.wg.Wait()
				return s.finalFlush(context.Background())
			}
			continue
		}

		var ping models.PingMessage
		if err := json.Unmarshal(msg.Value, &ping); err != nil {
			s.logger.Warn("invalid kafka message, committing offset",
				"service", "consumer", "error", err)
			if commitErr := s.reader.Commit(ctx, msg); commitErr != nil {
				return fmt.Errorf("consumer.Run: commit invalid msg: %w", commitErr)
			}
			continue
		}

		if reason := ValidatePingForInsert(ping); reason != "" {
			s.skipPoison(ctx, pendingEntry{ping: ping, msg: msg}, reason)
			continue
		}

		if err := s.append(ctx, pendingEntry{ping: ping, msg: msg}); err != nil {
			return fmt.Errorf("consumer.Run: %w", err)
		}
	}
}

func (s *Service) append(ctx context.Context, entry pendingEntry) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	s.batch = append(s.batch, entry)
	if len(s.batch) >= s.cfg.BatchSize {
		return s.flushLocked(ctx)
	}
	return nil
}

func (s *Service) flushLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cfg.FlushInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.batchMu.Lock()
			if len(s.batch) > 0 {
				if err := s.flushLocked(ctx); err != nil {
					s.logger.Error("batch flush failed", "service", "consumer", "error", err)
				}
			}
			s.batchMu.Unlock()
		}
	}
}

func (s *Service) finalFlush(ctx context.Context) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	return s.flushLocked(ctx)
}

func (s *Service) flushLocked(ctx context.Context) error {
	if len(s.batch) == 0 {
		return nil
	}

	pings := make([]models.PingMessage, len(s.batch))
	msgs := make([]kafkago.Message, len(s.batch))
	for i, entry := range s.batch {
		pings[i] = entry.ping
		msgs[i] = entry.msg
	}

	if err := s.writer.InsertBatch(ctx, pings); err != nil {
		// Whole-batch failure: isolate so one bad row cannot block or drop valid ones.
		s.logger.Warn("batch insert failed; falling back to per-message insert",
			"service", "consumer", "count", len(pings), "error", err)
		return s.flushOneByOne(ctx, s.batch)
	}

	if err := s.reader.Commit(ctx, msgs...); err != nil {
		return fmt.Errorf("consumer.flushLocked: commit: %w", err)
	}

	s.batch = s.batch[:0]
	s.logger.Info("batch flushed", "service", "consumer", "count", len(pings))
	return nil
}

func (s *Service) flushOneByOne(ctx context.Context, entries []pendingEntry) error {
	var toCommit []kafkago.Message
	var retry []pendingEntry
	ok := 0
	poisoned := 0
	for _, entry := range entries {
		if reason := ValidatePingForInsert(entry.ping); reason != "" {
			s.recordPoison(entry, reason)
			toCommit = append(toCommit, entry.msg)
			poisoned++
			continue
		}
		if err := s.writer.InsertBatch(ctx, []models.PingMessage{entry.ping}); err != nil {
			if isPoisonInsertError(err) {
				s.recordPoison(entry, err.Error())
				toCommit = append(toCommit, entry.msg)
				poisoned++
				continue
			}
			retry = append(retry, entry)
			s.logger.Error("per-message insert transient failure",
				"service", "consumer", "external_call_id", entry.ping.ExternalCallID, "error", err)
			continue
		}
		toCommit = append(toCommit, entry.msg)
		ok++
	}
	if err := s.reader.Commit(ctx, toCommit...); err != nil {
		return fmt.Errorf("consumer.flushOneByOne: commit: %w", err)
	}
	s.batch = retry
	s.logger.Info("per-message flush done",
		"service", "consumer", "written", ok, "poison_skipped", poisoned,
		"retry", len(retry), "poison_total", s.poisonCount.Load())
	if len(retry) > 0 {
		return fmt.Errorf("consumer.flushOneByOne: %d messages pending retry after transient errors", len(retry))
	}
	return nil
}

func (s *Service) recordPoison(entry pendingEntry, reason string) {
	n := s.poisonCount.Add(1)
	s.logger.Warn("poison ping skipped",
		"service", "consumer",
		"external_call_id", entry.ping.ExternalCallID,
		"tenant_id", entry.ping.TenantID,
		"platform", entry.ping.Platform,
		"kafka_topic", entry.msg.Topic,
		"kafka_partition", entry.msg.Partition,
		"kafka_offset", entry.msg.Offset,
		"reason", reason,
		"poison_total", n,
	)
}

// isPoisonInsertError reports whether an insert error is a deterministic row/schema
// problem (safe to skip + commit) vs a transient infrastructure failure (must retry).
// Only explicit data-shape needles match; connection/timeout/DNS errors fall through
// as transient (proven under a real ClickHouse outage in Phase 2A-fix2e).
func isPoisonInsertError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	needles := []string{"AppendRow", "invalid UUID", "FixedString", "batch is invalid", "invalid size"}
	for _, n := range needles {
		if containsFold(msg, n) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (s *Service) skipPoison(ctx context.Context, entry pendingEntry, reason string) {
	s.recordPoison(entry, reason)
	if err := s.reader.Commit(ctx, entry.msg); err != nil {
		s.logger.Error("poison commit failed", "service", "consumer", "error", err,
			"external_call_id", entry.ping.ExternalCallID)
	}
}

// ValidatePingForInsert returns a non-empty reason when the row would poison a
// ClickHouse batch (invalid UUID or FixedString(64) size panic).
func ValidatePingForInsert(p models.PingMessage) string {
	if !looksLikeUUID(p.TenantID) {
		return "invalid tenant_id UUID"
	}
	if n := len(p.ANIHash); n > 64 {
		return fmt.Sprintf("ani_hash length %d exceeds FixedString(64)", n)
	}
	return ""
}

func looksLikeUUID(s string) bool {
	// 8-4-4-4-12 hex with dashes
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
