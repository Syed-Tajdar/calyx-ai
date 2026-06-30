package connectors

import (
	"net/http"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
)

// RingbaConnector handles Ringba tracking pixel webhooks.
type RingbaConnector struct{}

func (RingbaConnector) Platform() string { return "ringba" }

func (RingbaConnector) Authenticate(r *http.Request, body []byte, webhookSecret string) bool {
	return security.VerifyRingbaHMAC(r, body, webhookSecret)
}

func (c RingbaConnector) BuildPing(
	tenantID string,
	r *http.Request,
	body []byte,
	encryptionKey []byte,
	receivedAt time.Time,
) (models.Ping, error) {
	var payload RingbaPayload
	var err error
	switch r.Method {
	case http.MethodGet:
		payload, err = ParseRingbaQuery(r.URL.Query())
	case http.MethodPost:
		if len(body) > 0 && r.Header.Get("Content-Type") == "application/json" {
			payload, err = ParseRingbaJSON(body)
		} else {
			payload, err = ParseRingbaQuery(r.URL.Query())
		}
	default:
		return models.Ping{}, ErrUnsupportedMethod
	}
	if err != nil {
		return models.Ping{}, err
	}
	return buildPing(
		tenantID, c.Platform(),
		payload.InboundCallID, payload.CallerID,
		payload.CampaignID, payload.PublisherID, payload.TargetID,
		payload.State, payload.EventType,
		payload.Revenue, payload.Payout, payload.IsDuplicate,
		encryptionKey, receivedAt,
	)
}
