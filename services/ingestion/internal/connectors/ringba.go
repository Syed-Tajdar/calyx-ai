package connectors

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// RingbaPayload holds parsed Ringba pixel/webhook fields.
// Ringba fires tracking pixels on Incoming/Completed events via GET query or POST JSON.
// See: https://support.ringba.com/hc/en-us/articles/17997458506135-Integromat
type RingbaPayload struct {
	InboundCallID string
	CallerID      string
	CampaignID    string
	PublisherID   string
	TargetID      string
	State         string
	EventType     string
	Revenue       float64
	Payout        float64
	IsDuplicate   bool
}

// ParseRingbaQuery parses Ringba pixel GET query parameters.
func ParseRingbaQuery(values url.Values) (RingbaPayload, error) {
	payload := RingbaPayload{
		InboundCallID: firstValue(values,
			"CallID", "call_id", "callId", "InboundCallId", "inbound_call_id"),
		CallerID: firstValue(values,
			"caller_id", "CallerID", "callerId", "ani", "InboundNumber"),
		CampaignID: firstValue(values,
			"campaign_id", "CampaignID", "campaignId"),
		PublisherID: firstValue(values,
			"publisher_id", "PublisherID", "publisherId"),
		TargetID: firstValue(values,
			"target_id", "TargetID", "targetId"),
		State: firstValue(values, "state", "State"),
		EventType: firstValue(values, "event", "Event", "event_type"),
	}

	if payload.EventType == "" {
		payload.EventType = "incoming"
	}

	revenue, err := parseFloat(values, "call_revenue", "revenue", "Revenue", "ConversionAmount")
	if err != nil {
		return RingbaPayload{}, fmt.Errorf("connectors.ParseRingbaQuery: revenue: %w", err)
	}
	payload.Revenue = revenue

	payout, err := parseFloat(values, "payout", "Payout", "call_payout")
	if err != nil {
		return RingbaPayload{}, fmt.Errorf("connectors.ParseRingbaQuery: payout: %w", err)
	}
	payload.Payout = payout

	dupStr := firstValue(values, "is_duplicate", "IsDuplicate", "duplicate")
	payload.IsDuplicate = strings.EqualFold(dupStr, "true") || dupStr == "1" || strings.EqualFold(dupStr, "yes")

	if payload.InboundCallID == "" {
		return RingbaPayload{}, fmt.Errorf("connectors.ParseRingbaQuery: missing inbound call id")
	}

	return payload, nil
}

// ParseRingbaJSON parses Ringba POST JSON pixel body.
func ParseRingbaJSON(body []byte) (RingbaPayload, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return RingbaPayload{}, fmt.Errorf("connectors.ParseRingbaJSON: %w", err)
	}
	values := url.Values{}
	for k, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			values.Set(k, s)
			continue
		}
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			values.Set(k, strconv.FormatFloat(f, 'f', -1, 64))
		}
	}
	return ParseRingbaQuery(values)
}
