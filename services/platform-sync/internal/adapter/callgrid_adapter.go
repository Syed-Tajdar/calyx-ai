package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
)

const callgridBaseURL = "https://api.callgrid.com/api/call"

type CallGridAdapter struct {
	http *HTTPClient
}

func NewCallGridAdapter(http *HTTPClient) *CallGridAdapter {
	return &CallGridAdapter{http: http}
}

func (a *CallGridAdapter) Platform() string { return "callgrid" }

type callgridCursor struct {
	StartDate   string          `json:"start_date"`
	EndDate     string          `json:"end_date"`
	SearchAfter json.RawMessage `json:"search_after,omitempty"`
}

func (a *CallGridAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
	_, _, _, err := a.fetch(ctx, creds, callgridDefaultCursor(""))
	return err
}

func (a *CallGridAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string, _ int) (PageResult, error) {
	cs := callgridDefaultCursor(lastSyncAt)
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	}
	calls, nextCursor, hasMore, err := a.fetch(ctx, creds, cs)
	if err != nil {
		return PageResult{}, err
	}
	next := callgridCursor{StartDate: cs.StartDate, EndDate: cs.EndDate, SearchAfter: nextCursor}
	enc, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{Calls: calls, NextCursor: enc, HasMore: hasMore}, nil
}

func callgridDefaultCursor(lastSyncAt string) callgridCursor {
	end := time.Now().UTC()
	start := end.Add(-24 * time.Hour)
	if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			start = t.Add(-5 * time.Minute)
		}
	}
	return callgridCursor{
		StartDate: start.Format(time.RFC3339),
		EndDate:   end.Format(time.RFC3339),
	}
}

func (a *CallGridAdapter) fetch(ctx context.Context, creds map[string]string, cs callgridCursor) ([]models.RemoteCall, json.RawMessage, bool, error) {
	apiKey := creds["api_key"]
	if apiKey == "" {
		return nil, nil, false, fmt.Errorf("callgrid: api_key required")
	}
	q := url.Values{}
	q.Set("startDate", cs.StartDate)
	q.Set("endDate", cs.EndDate)
	q.Set("useCursor", "true")
	q.Set("maxItems", "100")
	if len(cs.SearchAfter) > 0 {
		q.Set("searchAfter", string(cs.SearchAfter))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, callgridBaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	time.Sleep(500 * time.Millisecond)

	resp, body, err := a.http.Do(req)
	if err != nil {
		return nil, nil, false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, body, "callgrid"); err != nil {
		return nil, nil, false, err
	}
	var payload struct {
		Data       json.RawMessage `json:"data"`
		HasMore    bool            `json:"hasMore"`
		NextCursor json.RawMessage `json:"nextCursor"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, false, fmt.Errorf("callgrid: parse: %w", err)
	}
	var rows []json.RawMessage
	var wrapped struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(payload.Data, &wrapped); err == nil && len(wrapped.Items) > 0 {
		rows = wrapped.Items
	} else {
		_ = json.Unmarshal(payload.Data, &rows)
	}
	calls := make([]models.RemoteCall, 0, len(rows))
	for _, raw := range rows {
		calls = append(calls, mapCallGridCall(raw))
	}
	return calls, payload.NextCursor, payload.HasMore, nil
}

func mapCallGridCall(raw json.RawMessage) models.RemoteCall {
	var row map[string]interface{}
	_ = json.Unmarshal(raw, &row)
	str := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := row[k]; ok && v != nil {
				return fmt.Sprint(v)
			}
		}
		return ""
	}
	float := func(keys ...string) float64 {
		for _, k := range keys {
			if v, ok := row[k]; ok {
				switch n := v.(type) {
				case float64:
					return n
				case string:
					f, _ := strconv.ParseFloat(n, 64)
					return f
				}
			}
		}
		return 0
	}
	return models.RemoteCall{
		ExternalCallID: str("id", "callId", "call_id", "externalCallId"),
		CallerID:       str("caller", "callerId", "inboundNumber", "phone"),
		State:          state.NormalizeState(str("state", "inboundStateCode", "geo_state", "region")),
		Revenue:        float("revenue", "callRevenue", "conversionPayout"),
		Payout:         float("payout", "publisherPayout"),
		CampaignID:     str("campaignId", "campaign_id", "utm_campaign"),
		PublisherID:    str("publisherId", "affiliateId", "publisher_id"),
		TargetID:       str("targetId", "buyerId", "target_id"),
		EventType:      str("status", "event", "eventType"),
	}
}
