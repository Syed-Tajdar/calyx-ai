package connectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
)

// RetreaverConnector handles Retreaver postback webhooks.
// Fields: call_uuid, call_key (private), tags as key:value pairs.
type RetreaverConnector struct{}

func (RetreaverConnector) Platform() string { return "retreaver" }

func (RetreaverConnector) Authenticate(r *http.Request, body []byte, webhookSecret string) bool {
	return security.VerifyRetreaverHMAC(r, body, webhookSecret)
}

func (c RetreaverConnector) BuildPing(
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
			return models.Ping{}, fmt.Errorf("retreaver parse json: %w", err)
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
		ping.CallKey = parseCallKeyFromJSON(body)
		if ping.CallKey == "" {
			ping.CallKey = firstValue(r.URL.Query(), "call_key", "callKey", "CallKey")
		}
		return ping, nil
	}

	callUUID := firstValue(values, "call_uuid", "callUuid", "CallUUID")
	if callUUID == "" {
		return models.Ping{}, fmt.Errorf("retreaver: missing call_uuid")
	}

	tags := parseTags(values)
	state := firstValue(values, "state", "State")
	if state == "" {
		state = tags["state"]
	}
	if state == "" {
		state = tags["geo_state"]
	}

	revenue, err := parseFloat(values, "revenue", "conversion_value", "payout")
	if err != nil {
		return models.Ping{}, fmt.Errorf("retreaver revenue: %w", err)
	}
	if revenue == 0 && tags["revenue"] != "" {
		revenue, _ = strconvParseFloat(tags["revenue"])
	}

	payout, err := parseFloat(values, "payout", "publisher_payout")
	if err != nil {
		return models.Ping{}, fmt.Errorf("retreaver payout: %w", err)
	}

	ping, err := buildPing(
		tenantID, c.Platform(),
		callUUID,
		firstValue(values, "caller", "caller_id", "caller_number", "ani"),
		firstValue(values, "campaign_id", "campaign"),
		firstValue(values, "publisher_id", "publisher"),
		firstValue(values, "target_id", "target"),
		state,
		firstValue(values, "event", "event_type", "status"),
		revenue, payout,
		parseBool(values, "is_duplicate", "duplicate"),
		encryptionKey, receivedAt,
	)
	if err != nil {
		return models.Ping{}, err
	}
	ping.CallKey = firstValue(values, "call_key", "callKey", "CallKey")
	return ping, nil
}

func parseCallKeyFromJSON(body []byte) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	for _, key := range []string{"call_key", "callKey", "CallKey"} {
		v, ok := raw[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}

// ParseRetreaverQuery is exported for tests.
func ParseRetreaverQuery(values url.Values) (string, error) {
	id := firstValue(values, "call_uuid", "callUuid")
	if id == "" {
		return "", fmt.Errorf("retreaver: missing call_uuid")
	}
	return id, nil
}

func strconvParseFloat(s string) (float64, error) {
	return parseFloat(url.Values{"v": {s}}, "v")
}
