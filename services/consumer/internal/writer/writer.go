package writer

import (
	"context"
	"fmt"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/pkg/clickhouse"
)

// BatchWriter inserts ping batches into ClickHouse.
type BatchWriter struct {
	client *clickhouse.Client
}

// NewBatchWriter creates a ClickHouse batch writer.
func NewBatchWriter(client *clickhouse.Client) *BatchWriter {
	return &BatchWriter{client: client}
}

// InsertBatch writes a batch of pings to ClickHouse.
func (w *BatchWriter) InsertBatch(ctx context.Context, pings []models.PingMessage) error {
	if len(pings) == 0 {
		return nil
	}

	rows := make([]clickhouse.PingRow, 0, len(pings))
	for _, p := range pings {
		dup := uint8(0)
		if p.IsDuplicate {
			dup = 1
		}
		platDup := uint8(0)
		if p.PlatformDuplicate {
			platDup = 1
		}
		hasRec := uint8(0)
		if p.HasRecording {
			hasRec = 1
		}
		recKnown := uint8(0)
		if p.RecordingKnown {
			recKnown = 1
		}
		occurred := p.OccurredTime()
		if occurred.IsZero() {
			occurred = time.Unix(0, 0).UTC()
		}
		rows = append(rows, clickhouse.PingRow{
			TenantID:          p.TenantID,
			Platform:          p.Platform,
			ExternalCallID:    p.ExternalCallID,
			ANIHash:           p.ANIHash,
			CampaignID:        p.CampaignID,
			PublisherID:       p.PublisherID,
			TargetID:          p.TargetID,
			State:             p.State,
			EventType:         p.EventType,
			Revenue:           p.Revenue,
			Payout:            p.Payout,
			IsDuplicate:       dup,
			PlatformDuplicate: platDup,
			ReceivedAt:        p.ReceivedTime(),
			OccurredAt:        occurred,
			CallKey:           p.CallKey,
			CampaignName:      p.CampaignName,
			PublisherName:     p.PublisherName,
			TargetName:        p.TargetName,
			BuyerID:           p.BuyerID,
			BuyerName:         p.BuyerName,
			DurationSeconds:   p.DurationSeconds,
			CallLengthSeconds: p.CallLengthSeconds,
			HasRecording:      hasRec,
			RecordingKnown:    recKnown,
			ProfitGross:       p.ProfitGross,
			SyncPath:          p.SyncPath,
		})
	}

	if err := w.client.InsertPings(ctx, rows); err != nil {
		return fmt.Errorf("writer.InsertBatch: %w", err)
	}
	return nil
}

// Ping checks ClickHouse connectivity.
func (w *BatchWriter) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return w.client.Ping(ctx)
}
