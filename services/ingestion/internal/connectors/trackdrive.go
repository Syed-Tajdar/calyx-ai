package connectors

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
)

// TrackDriveConnector handles TrackDrive postbacks.
// Auth is token-based (lead_token / auth_token), NOT HMAC.
type TrackDriveConnector struct{}

func (TrackDriveConnector) Platform() string { return "trackdrive" }

func (TrackDriveConnector) Authenticate(r *http.Request, _ []byte, webhookSecret string) bool {
	if webhookSecret == "" {
		return false
	}
	token := firstValue(r.URL.Query(), "lead_token", "auth_token", "token")
	return token != "" && token == webhookSecret
}

func (c TrackDriveConnector) BuildPing(
	tenantID string,
	r *http.Request,
	_ []byte,
	encryptionKey []byte,
	receivedAt time.Time,
) (models.Ping, error) {
	values := r.URL.Query()
	callUUID := firstValue(values, "call_uuid", "callUuid", "uuid")
	if callUUID == "" {
		return models.Ping{}, fmt.Errorf("trackdrive: missing call_uuid")
	}

	revenue, err := parseFloat(values, "revenue", "buyer_revenue", "conversion_amount")
	if err != nil {
		return models.Ping{}, fmt.Errorf("trackdrive revenue: %w", err)
	}
	payout, err := parseFloat(values, "payout", "traffic_source_payout", "publisher_payout")
	if err != nil {
		return models.Ping{}, fmt.Errorf("trackdrive payout: %w", err)
	}

	eventType := firstValue(values, "event", "event_type", "conversion_type")
	if eventType == "" {
		eventType = "buyer_converted"
	}

	return buildPing(
		tenantID, c.Platform(),
		callUUID,
		firstValue(values, "caller_number", "caller_id", "caller", "ani"),
		firstValue(values, "campaign_id", "campaign"),
		firstValue(values, "traffic_source_id", "publisher_id", "publisher"),
		firstValue(values, "buyer_id", "target_id", "target"),
		firstValue(values, "state", "geo_state"),
		eventType,
		revenue, payout,
		parseBool(values, "is_duplicate", "duplicate"),
		encryptionKey, receivedAt,
	)
}
