package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
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
	cfg     config.Config
	reader  *kafka.Reader
	writer  *writer.BatchWriter
	logger  *slog.Logger
	batch   []pendingEntry
	batchMu sync.Mutex
	wg      sync.WaitGroup
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
		return fmt.Errorf("consumer.flushLocked: %w", err)
	}

	if err := s.reader.Commit(ctx, msgs...); err != nil {
		return fmt.Errorf("consumer.flushLocked: commit: %w", err)
	}

	s.batch = s.batch[:0]
	s.logger.Info("batch flushed", "service", "consumer", "count", len(pings))
	return nil
}
