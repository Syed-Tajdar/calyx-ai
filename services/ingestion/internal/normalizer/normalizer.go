package normalizer

import (
	"time"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ingestion/internal/connectors"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ingestion/internal/crypto"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ingestion/internal/models"
)

// NormalizeRingba maps a Ringba payload to the internal ping schema.
func NormalizeRingba(
	tenantID string,
	payload connectors.RingbaPayload,
	encryptionKey []byte,
	receivedAt time.Time,
) (models.Ping, error) {
	aniHash := ""
	var aniEncrypted []byte
	if payload.CallerID != "" {
		aniHash = crypto.HashANI(payload.CallerID)
		enc, err := crypto.EncryptANI(payload.CallerID, encryptionKey)
		if err != nil {
			return models.Ping{}, err
		}
		aniEncrypted = enc
	}

	return models.Ping{
		TenantID:       tenantID,
		Platform:       "ringba",
		ExternalCallID: payload.InboundCallID,
		ANIHash:        aniHash,
		ANIEncrypted:   aniEncrypted,
		CampaignID:     payload.CampaignID,
		PublisherID:    payload.PublisherID,
		TargetID:       payload.TargetID,
		State:          payload.State,
		EventType:      payload.EventType,
		Revenue:        payload.Revenue,
		Payout:         payload.Payout,
		IsDuplicate:    payload.IsDuplicate,
		ReceivedAt:     receivedAt,
	}, nil
}
