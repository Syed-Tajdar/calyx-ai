package connectors

import (
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
	usstate "github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/state"
)

func buildPing(
	tenantID string,
	platform string,
	externalCallID string,
	callerID string,
	campaignID string,
	publisherID string,
	targetID string,
	state string,
	eventType string,
	revenue float64,
	payout float64,
	isDuplicate bool,
	encryptionKey []byte,
	receivedAt time.Time,
) (models.Ping, error) {
	aniHash := ""
	var aniEncrypted []byte
	if callerID != "" {
		aniHash = crypto.HashANI(callerID)
		enc, err := crypto.EncryptANI(callerID, encryptionKey)
		if err != nil {
			return models.Ping{}, err
		}
		aniEncrypted = enc
	}
	if eventType == "" {
		eventType = "incoming"
	}
	return models.Ping{
		TenantID:       tenantID,
		Platform:       platform,
		ExternalCallID: externalCallID,
		ANIHash:        aniHash,
		ANIEncrypted:   aniEncrypted,
		CampaignID:     campaignID,
		PublisherID:    publisherID,
		TargetID:       targetID,
		State:          usstate.NormalizeState(state),
		EventType:      eventType,
		Revenue:        revenue,
		Payout:         payout,
		IsDuplicate:    isDuplicate,
		ReceivedAt:     receivedAt,
	}, nil
}
