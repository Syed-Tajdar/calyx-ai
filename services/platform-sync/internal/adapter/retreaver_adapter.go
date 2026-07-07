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

const retreaverBaseURL = "https://api.retreaver.com/api/v2/calls.json"

type RetreaverAdapter struct {
	http *HTTPClient
}

func NewRetreaverAdapter(http *HTTPClient) *RetreaverAdapter {
	return &RetreaverAdapter{http: http}
}

func (a *RetreaverAdapter) Platform() string { return "retreaver" }

type retreaverCursor struct {
	CreatedAtStart string `json:"created_at_start"`
	Page           int    `json:"page"`
}

func (a *RetreaverAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
	_, _, err := a.fetchCallsWithPerPage(ctx, creds, retreaverCursor{Page: 1}, 1)
	return err
}

func (a *RetreaverAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string) (PageResult, error) {
	cs := retreaverCursor{Page: 1}
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	} else if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			cs.CreatedAtStart = t.Add(-5 * time.Minute).UTC().Format(time.RFC3339)
		}
	}
	if cs.CreatedAtStart == "" {
		cs.CreatedAtStart = time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	}
	if cs.Page < 1 {
		cs.Page = 1
	}
	calls, hasMore, err := a.fetchCallsWithPerPage(ctx, creds, cs, 100)
	if err != nil {
		return PageResult{}, err
	}
	next := cs
	if hasMore {
		next.Page++
	} else {
		next.Page = 1
		next.CreatedAtStart = time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	}
	nextCursor, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{Calls: calls, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (a *RetreaverAdapter) fetchCallsWithPerPage(ctx context.Context, creds map[string]string, cs retreaverCursor, perPage int) ([]models.RemoteCall, bool, error) {
	apiKey := creds["api_key"]
	companyID := creds["company_id"]
	if apiKey == "" || companyID == "" {
		return nil, false, fmt.Errorf("retreaver: api_key and company_id required")
	}
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("created_at_start", cs.CreatedAtStart)
	q.Set("sort_by", "created_at")
	q.Set("order", "asc")
	q.Set("page", strconv.Itoa(cs.Page))
	q.Set("per_page", strconv.Itoa(perPage))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, retreaverBaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, false, err
	}
	resp, body, err := a.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, body, "retreaver"); err != nil {
		return nil, false, err
	}
	var payload struct {
		Calls []json.RawMessage `json:"calls"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		var arr []json.RawMessage
		if err2 := json.Unmarshal(body, &arr); err2 != nil {
			return nil, false, fmt.Errorf("retreaver: parse response: %w", err)
		}
		payload.Calls = arr
	}
	calls := make([]models.RemoteCall, 0, len(payload.Calls))
	for _, raw := range payload.Calls {
		calls = append(calls, mapRetreaverCall(raw))
	}
	return calls, len(payload.Calls) >= perPage, nil
}

func mapRetreaverCall(raw json.RawMessage) models.RemoteCall {
	var row struct {
		UUID        string   `json:"uuid"`
		Caller      string   `json:"caller"`
		CallerState string   `json:"caller_state"`
		Revenue     *float64 `json:"revenue"`
		Payout      *float64 `json:"payout"`
		CampaignID  int      `json:"campaign_id"`
		ClientAFID  string   `json:"client_afid"`
		TargetID    int      `json:"target_id"`
		Status      string   `json:"status"`
		Connected   *bool    `json:"connected"`
	}
	_ = json.Unmarshal(raw, &row)
	rev, pay := 0.0, 0.0
	if row.Revenue != nil {
		rev = *row.Revenue
	}
	if row.Payout != nil {
		pay = *row.Payout
	}
	eventType := "incoming"
	if row.Status != "" {
		eventType = row.Status
	} else if row.Connected != nil && *row.Connected {
		eventType = "connected"
	}
	return models.RemoteCall{
		ExternalCallID: row.UUID,
		CallerID:       row.Caller,
		State:          state.NormalizeState(row.CallerState),
		Revenue:        rev,
		Payout:         pay,
		CampaignID:     strconv.Itoa(row.CampaignID),
		PublisherID:    row.ClientAFID,
		TargetID:       strconv.Itoa(row.TargetID),
		EventType:      eventType,
	}
}

func checkHTTPStatus(status int, body []byte, platform string) error {
	if status >= 200 && status < 300 {
		return nil
	}
	snippet := string(body)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	return fmt.Errorf("%s HTTP %d: %s", platform, status, snippet)
}
