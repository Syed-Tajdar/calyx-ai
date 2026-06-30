package connectors

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
)

// CallGridConnector handles CallGrid postback webhooks.
type CallGridConnector struct{}

func (CallGridConnector) Platform() string { return "callgrid" }

func (CallGridConnector) Authenticate(r *http.Request, body []byte, webhookSecret string) bool {
	return security.VerifyCallGridHMAC(r, body, webhookSecret)
}

func (c CallGridConnector) BuildPing(
	tenantID string,
	r *http.Request,
	body []byte,
	encryptionKey []byte,
	receivedAt time.Time,
) (models.Ping, error) {
	values := r.URL.Query()
	if len(body) > 0 && r.Header.Get("Content-Type") == "application/json" {
		parsed, err := ParseRingbaJSON(body)
		if err != nil {
			return models.Ping{}, fmt.Errorf("callgrid parse json: %w", err)
		}
		ping, err := buildPing(
			tenantID, c.Platform(),
			parsed.InboundCallID, parsed.CallerID,
			parsed.CampaignID, parsed.PublisherID, parsed.TargetID,
			parsed.State, parsed.EventType,
			parsed.Revenue, parsed.Payout, parsed.IsDuplicate,
			encryptionKey, receivedAt,
		)
		if err != nil {
			return models.Ping{}, err
		}
		if ping.ExternalCallID == "" {
			return models.Ping{}, fmt.Errorf("callgrid: missing call id")
		}
		return ping, nil
	}

	callID := firstValue(values,
		"call_id", "callId", "CallId", "CallID", "external_call_id")
	if callID == "" {
		return models.Ping{}, fmt.Errorf("callgrid: missing call_id")
	}

	revenue, err := parseFloat(values, "revenue", "conversion_payout", "call_revenue")
	if err != nil {
		return models.Ping{}, fmt.Errorf("callgrid revenue: %w", err)
	}
	payout, err := parseFloat(values, "payout", "publisher_payout")
	if err != nil {
		return models.Ping{}, fmt.Errorf("callgrid payout: %w", err)
	}

	return buildPing(
		tenantID, c.Platform(),
		callID,
		firstValue(values, "caller", "caller_id", "ani", "phone"),
		firstValue(values, "campaign_id", "campaign", "utm_campaign"),
		firstValue(values, "publisher_id", "publisher", "affiliate_id"),
		firstValue(values, "target_id", "target", "buyer_id"),
		firstValue(values, "state", "geo_state", "region"),
		firstValue(values, "event", "event_type", "status"),
		revenue, payout,
		parseBool(values, "is_duplicate", "duplicate"),
		encryptionKey, receivedAt,
	)
}
