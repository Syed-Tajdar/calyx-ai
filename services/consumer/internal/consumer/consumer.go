package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

// MaxUnknownAttempts before treating a healthy-CH unknown insert error as poison.
// Justified: 5 covers brief CH blips where Ping still succeeds; avoids infinite stall (F013).
const MaxUnknownAttempts = 5

type pendingEntry struct {
	ping models.PingMessage
	msg  kafkago.Message
}

type batchWriter interface {
	InsertBatch(ctx context.Context, pings []models.PingMessage) error
	Ping(ctx context.Context) error
}

type kafkaReader interface {
	Fetch(ctx context.Context) (kafkago.Message, error)
	Commit(ctx context.Context, msgs ...kafkago.Message) error
}

type dlqPublisher interface {
	Publish(ctx context.Context, key, value []byte, headers []kafkago.Header) error
}

// Service consumes ping events and batches them to ClickHouse.
type Service struct {
	cfg     config.Config
	reader  kafkaReader
	writer  batchWriter
	dlq     dlqPublisher
	logger  *slog.Logger
	batch   []pendingEntry
	batchMu sync.Mutex
	wg      sync.WaitGroup
	metrics *Metrics
	paused  atomic.Bool
	// attempt key = topic|partition|offset
	attempts   map[string]int
	attemptsMu sync.Mutex
}

// NewService creates a Kafka consumer service.
func NewService(cfg config.Config, reader kafkaReader, batchWriter batchWriter, logger *slog.Logger, dlq dlqPublisher) *Service {
	return &Service{
		cfg:      cfg,
		reader:   reader,
		writer:   batchWriter,
		dlq:      dlq,
		logger:   logger,
		batch:    make([]pendingEntry, 0, cfg.BatchSize),
		metrics:  &Metrics{},
		attempts: map[string]int{},
	}
}

// GetMetrics returns the service metrics.
func (s *Service) GetMetrics() *Metrics { return s.metrics }

// PoisonCount returns how many messages were skipped as poison (insert failures).
func (s *Service) PoisonCount() int64 {
	return s.metrics.poisonTotal.Load()
}

// Run starts the consumer loop and flush ticker. Blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) error {
	s.wg.Add(1)
	go s.flushLoop(ctx)

	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return s.finalFlush(context.Background())
		default:
		}

		if s.paused.Load() {
			s.batchMu.Lock()
			err := s.flushLocked(ctx, "resume")
			n := len(s.batch)
			s.batchMu.Unlock()
			s.metrics.SetBuffered(n)
			if err != nil {
				s.metrics.IncFlushFailure()
				s.logger.Error("paused flush failed; holding fetch",
					"service", "consumer", "error", err, "buffered", n)
				select {
				case <-ctx.Done():
					s.wg.Wait()
					return s.finalFlush(context.Background())
				case <-time.After(backoff):
					if backoff < 30*time.Second {
						backoff *= 2
					}
				}
				continue
			}
			s.paused.Store(false)
			backoff = time.Second
			continue
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
			s.logger.Warn("invalid kafka message, routing to DLQ",
				"service", "consumer", "error", err)
			if dlqErr := s.deadLetterThenCommit(ctx, pendingEntry{msg: msg}, "invalid_json: "+err.Error()); dlqErr != nil {
				s.logger.Error("dlq failed for invalid json; not committing",
					"service", "consumer", "error", dlqErr)
				// Do not commit — redeliver.
				time.Sleep(time.Second)
			}
			continue
		}

		if reason := ValidatePingForInsert(ping); reason != "" {
			if err := s.deadLetterThenCommit(ctx, pendingEntry{ping: ping, msg: msg}, reason); err != nil {
				s.logger.Error("dlq failed for poison validate; not committing",
					"service", "consumer", "error", err)
				time.Sleep(time.Second)
			}
			continue
		}

		if err := s.append(ctx, pendingEntry{ping: ping, msg: msg}); err != nil {
			// Size-path failures must not kill Run (F015) — append keeps the batch.
			s.logger.Error("append/flush soft failure", "service", "consumer", "error", err)
		}
	}
}

func (s *Service) append(ctx context.Context, entry pendingEntry) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	s.batch = append(s.batch, entry)
	s.metrics.SetBuffered(len(s.batch))
	if len(s.batch) >= s.cfg.BatchSize {
		s.logger.Info("size-triggered flush",
			"service", "consumer", "batch_size", len(s.batch))
		if err := s.flushLocked(ctx, "size"); err != nil {
			s.metrics.IncFlushFailure()
			s.paused.Store(true)
			s.logger.Error("size-triggered flush failed; pausing fetch",
				"service", "consumer", "error", err, "buffered", len(s.batch))
			return nil // keep Run alive
		}
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
				if err := s.flushLocked(ctx, "ticker"); err != nil {
					s.metrics.IncFlushFailure()
					s.logger.Error("batch flush failed", "service", "consumer", "error", err)
					if len(s.batch) >= s.cfg.BatchSize {
						s.paused.Store(true)
					}
				}
			}
			s.metrics.SetBuffered(len(s.batch))
			s.batchMu.Unlock()
		}
	}
}

func (s *Service) finalFlush(ctx context.Context) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()
	return s.flushLocked(ctx, "shutdown")
}

func (s *Service) flushLocked(ctx context.Context, path string) error {
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
		s.logger.Warn("batch insert failed; falling back to per-message insert",
			"service", "consumer", "count", len(pings), "path", path, "error", err)
		return s.flushOneByOne(ctx, s.batch)
	}

	if err := s.reader.Commit(ctx, msgs...); err != nil {
		return fmt.Errorf("consumer.flushLocked: commit: %w", err)
	}

	for _, e := range s.batch {
		s.clearAttempts(e.msg)
	}
	s.batch = s.batch[:0]
	s.metrics.SetBuffered(0)
	s.metrics.MarkFlushOK()
	s.logger.Info("batch flushed", "service", "consumer", "count", len(pings), "path", path)
	return nil
}

func (s *Service) flushOneByOne(ctx context.Context, entries []pendingEntry) error {
	var toCommit []kafkago.Message
	var retry []pendingEntry
	ok := 0
	poisoned := 0
	chHealthy := s.writer.Ping(ctx) == nil

	for _, entry := range entries {
		if reason := ValidatePingForInsert(entry.ping); reason != "" {
			if err := s.deadLetter(ctx, entry, reason); err != nil {
				retry = append(retry, entry)
				s.logger.Error("dlq failed; keeping message", "service", "consumer", "error", err)
				continue
			}
			toCommit = append(toCommit, entry.msg)
			poisoned++
			continue
		}
		if err := s.writer.InsertBatch(ctx, []models.PingMessage{entry.ping}); err != nil {
			if isPoisonInsertError(err) {
				if dlqErr := s.deadLetter(ctx, entry, err.Error()); dlqErr != nil {
					retry = append(retry, entry)
					s.logger.Error("dlq failed for poison insert; keeping", "service", "consumer", "error", dlqErr)
					continue
				}
				toCommit = append(toCommit, entry.msg)
				poisoned++
				continue
			}
			// Unknown error: outage vs poison (F013 / fix2e).
			if !chHealthy {
				retry = append(retry, entry)
				s.metrics.IncRetries(1)
				s.logger.Error("per-message insert during CH outage; retry forever",
					"service", "consumer", "external_call_id", entry.ping.ExternalCallID, "error", err)
				continue
			}
			n := s.bumpAttempts(entry.msg)
			s.metrics.IncRetries(1)
			if n >= MaxUnknownAttempts {
				reason := fmt.Sprintf("unknown_insert_after_%d_attempts: %v", n, err)
				if dlqErr := s.deadLetter(ctx, entry, reason); dlqErr != nil {
					retry = append(retry, entry)
					s.logger.Error("dlq failed after bounded retries; keeping", "service", "consumer", "error", dlqErr)
					continue
				}
				toCommit = append(toCommit, entry.msg)
				poisoned++
				s.clearAttempts(entry.msg)
				continue
			}
			retry = append(retry, entry)
			s.logger.Error("per-message insert transient failure",
				"service", "consumer", "external_call_id", entry.ping.ExternalCallID,
				"attempt", n, "error", err)
			continue
		}
		toCommit = append(toCommit, entry.msg)
		s.clearAttempts(entry.msg)
		ok++
	}
	if err := s.reader.Commit(ctx, toCommit...); err != nil {
		return fmt.Errorf("consumer.flushOneByOne: commit: %w", err)
	}
	s.batch = retry
	s.metrics.SetBuffered(len(retry))
	if ok > 0 || poisoned > 0 {
		s.metrics.MarkFlushOK()
	}
	s.logger.Info("per-message flush done",
		"service", "consumer", "written", ok, "poison_skipped", poisoned,
		"retry", len(retry), "poison_total", s.PoisonCount(), "ch_healthy", chHealthy)
	if len(retry) > 0 {
		return fmt.Errorf("consumer.flushOneByOne: %d messages pending retry after transient errors", len(retry))
	}
	return nil
}

func msgKey(msg kafkago.Message) string {
	return fmt.Sprintf("%s|%d|%d", msg.Topic, msg.Partition, msg.Offset)
}

func (s *Service) bumpAttempts(msg kafkago.Message) int {
	s.attemptsMu.Lock()
	defer s.attemptsMu.Unlock()
	k := msgKey(msg)
	s.attempts[k]++
	return s.attempts[k]
}

func (s *Service) clearAttempts(msg kafkago.Message) {
	s.attemptsMu.Lock()
	defer s.attemptsMu.Unlock()
	delete(s.attempts, msgKey(msg))
}

func (s *Service) deadLetter(ctx context.Context, entry pendingEntry, reason string) error {
	s.recordPoison(entry, reason)
	if s.dlq == nil {
		return fmt.Errorf("dlq publisher not configured")
	}
	headers := []kafkago.Header{
		{Key: "x-source-topic", Value: []byte(entry.msg.Topic)},
		{Key: "x-source-partition", Value: []byte(strconv.Itoa(entry.msg.Partition))},
		{Key: "x-source-offset", Value: []byte(strconv.FormatInt(entry.msg.Offset, 10))},
		{Key: "x-poison-reason", Value: []byte(reason)},
		{Key: "x-poisoned-at", Value: []byte(time.Now().UTC().Format(time.RFC3339Nano))},
	}
	for _, h := range entry.msg.Headers {
		headers = append(headers, h)
	}
	if err := s.dlq.Publish(ctx, entry.msg.Key, entry.msg.Value, headers); err != nil {
		return err
	}
	s.metrics.IncDLQ()
	return nil
}

func (s *Service) deadLetterThenCommit(ctx context.Context, entry pendingEntry, reason string) error {
	if err := s.deadLetter(ctx, entry, reason); err != nil {
		return err
	}
	return s.reader.Commit(ctx, entry.msg)
}

func (s *Service) recordPoison(entry pendingEntry, reason string) {
	s.metrics.IncPoison()
	n := s.PoisonCount()
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
