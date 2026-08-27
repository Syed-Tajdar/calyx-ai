package producer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/pkg/kafka"
)

type Producer struct {
	client        *kafka.Client
	topic         string
	encryptionKey []byte
}

func NewProducer(client *kafka.Client, topic string, encryptionKey []byte) *Producer {
	return &Producer{client: client, topic: topic, encryptionKey: encryptionKey}
}

func (p *Producer) Publish(ctx context.Context, tenantID, platform string, call models.RemoteCall, isDuplicate bool, receivedAt int64) error {
	aniHash := ""
	if call.CallerID != "" {
		aniHash = crypto.HashANI(call.CallerID)
	}
	callKeyStored := ""
	if call.CallKey != "" {
		enc, err := crypto.EncryptSecret(call.CallKey, p.encryptionKey)
		if err != nil {
			return fmt.Errorf("encrypt call_key: %w", err)
		}
		callKeyStored = hex.EncodeToString(enc)
	}
	eventType := call.EventType
	if eventType == "" {
		eventType = "incoming"
	}
	msg := models.PingMessage{
		TenantID:          tenantID,
		Platform:          platform,
		ExternalCallID:    call.ExternalCallID,
		ANIHash:           aniHash,
		CampaignID:        call.CampaignID,
		PublisherID:       call.PublisherID,
		TargetID:          call.TargetID,
		State:             state.NormalizeState(call.State),
		EventType:         eventType,
		Revenue:           call.Revenue,
		Payout:            call.Payout,
		IsDuplicate:       isDuplicate,
		PlatformDuplicate: call.PlatformDuplicate,
		ReceivedAt:        receivedAt,
		CallKey:           callKeyStored,
		CampaignName:      call.CampaignName,
		PublisherName:     call.PublisherName,
		TargetName:        call.TargetName,
		BuyerID:           call.BuyerID,
		BuyerName:         call.BuyerName,
		DurationSeconds:   call.DurationSeconds,
		CallLengthSeconds: call.CallLengthSeconds,
		HasRecording:      call.HasRecording,
		RecordingKnown:    call.RecordingKnown,
		ProfitGross:       call.ProfitGross,
		SyncPath:          "api_sync",
	}
	if !call.OccurredAt.IsZero() {
		msg.OccurredAt = call.OccurredAt.UTC().UnixMilli()
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return p.client.Write(ctx, p.topic, tenantID, body)
}

func (p *Producer) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}
