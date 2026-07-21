package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
)

const defaultRetreaverBaseURL = "https://api.retreaver.com/api/v2/calls.json"

// retreaverCursorMode documents Retreaver incremental sync strategy (Decision 1).
// Full per-integration cursor_mode DB column is deferred; Retreaver is always updated_at.
const retreaverCursorMode = "updated_at"

// retreaverCursorSafetyDelta rewinds the updated_at cursor to avoid missing late updates.
const retreaverCursorSafetyDelta = 15 * time.Minute

// retreaverDefaultLookbackDays is used when empty-cursor lookback is not provided (B3 default).
const retreaverDefaultLookbackDays = 30

// retreaverInitialLookback is the default empty-cursor window duration.
const retreaverInitialLookback = time.Duration(retreaverDefaultLookbackDays) * 24 * time.Hour

func retreaverBaseURL() string {
	if v := os.Getenv("RETREAVER_BASE_URL"); v != "" {
		return v
	}
	return defaultRetreaverBaseURL
}

type RetreaverAdapter struct {
	http *HTTPClient
}

func NewRetreaverAdapter(http *HTTPClient) *RetreaverAdapter {
	return &RetreaverAdapter{http: http}
}

func (a *RetreaverAdapter) Platform() string { return "retreaver" }

// retreaverCursor tracks incremental pull by last-modified time (cursor_mode=updated_at).
// CreatedAtStart is legacy (pre-D9); migrated into UpdatedAtStart when present.
type retreaverCursor struct {
	UpdatedAtStart string `json:"updated_at_start"`
	CreatedAtStart string `json:"created_at_start,omitempty"` // legacy migrate-only
	Page           int    `json:"page"`
}

func (cs *retreaverCursor) normalize() {
	if cs.UpdatedAtStart == "" && cs.CreatedAtStart != "" {
		cs.UpdatedAtStart = cs.CreatedAtStart
	}
	cs.CreatedAtStart = "" // stop persisting legacy field
	if cs.Page < 1 {
		cs.Page = 1
	}
}

func retreaverLookbackDuration(lookbackDays int) time.Duration {
	days := lookbackDays
	if days < 1 {
		days = retreaverDefaultLookbackDays
	}
	if days > 365 {
		days = 365
	}
	return time.Duration(days) * 24 * time.Hour
}

// maxCallUpdatedAt returns the latest non-zero UpdatedAt among calls, or fallback.
func maxCallUpdatedAt(calls []models.RemoteCall, fallback time.Time) time.Time {
	max := time.Time{}
	for _, c := range calls {
		if !c.UpdatedAt.IsZero() && (max.IsZero() || c.UpdatedAt.After(max)) {
			max = c.UpdatedAt
		}
	}
	if max.IsZero() {
		return fallback
	}
	return max
}

func (a *RetreaverAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
	_, _, err := a.fetchCallsWithPerPage(ctx, creds, retreaverCursor{
		UpdatedAtStart: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		Page:           1,
	}, 1)
	return err
}

func (a *RetreaverAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string, lookbackDays int) (PageResult, error) {
	cs := retreaverCursor{Page: 1}
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	} else if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			cs.UpdatedAtStart = t.Add(-retreaverCursorSafetyDelta).UTC().Format(time.RFC3339)
		}
	}
	cs.normalize()
	if cs.UpdatedAtStart == "" {
		cs.UpdatedAtStart = time.Now().UTC().Add(-retreaverLookbackDuration(lookbackDays)).Format(time.RFC3339)
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
		anchor := maxCallUpdatedAt(calls, time.Now().UTC())
		next.UpdatedAtStart = anchor.Add(-retreaverCursorSafetyDelta).UTC().Format(time.RFC3339)
	}
	next.CreatedAtStart = ""
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
	cs.normalize()
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("updated_at_start", cs.UpdatedAtStart)
	q.Set("sort_by", "updated_at")
	// Docs: sort_by=updated_at is forced to desc even if order=asc.
	q.Set("order", "desc")
	q.Set("page", strconv.Itoa(cs.Page))
	q.Set("per_page", strconv.Itoa(perPage))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, retreaverBaseURL()+"?"+q.Encode(), nil)
	if err != nil {
		return nil, false, err
	}
	resp, body, err := a.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, body, resp.Header, "retreaver"); err != nil {
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
	raw = unwrapRetreaverCallPayload(raw)
	var row struct {
		UUID               string   `json:"uuid"`
		CallUUID           string   `json:"call_uuid"`
		ID                 string   `json:"id"`
		Caller             string   `json:"caller"`
		CallerState        string   `json:"caller_state"`
		Revenue            *float64 `json:"revenue"`
		Payout             *float64 `json:"payout"`
		CampaignID         int      `json:"campaign_id"`
		SystemCampaignID   int      `json:"system_campaign_id"`
		ClientAFID         string   `json:"client_afid"`
		AFID               string   `json:"afid"`
		TargetID           int      `json:"target_id"`
		SystemTargetID     int      `json:"system_target_id"`
		CallpixelsTargetID int      `json:"callpixels_target_id"`
		Status             string   `json:"status"`
		Connected          *bool    `json:"connected"`
		CreatedAt          string   `json:"created_at"`
		UpdatedAt          string   `json:"updated_at"`
	}
	_ = json.Unmarshal(raw, &row)
	rev, pay := 0.0, 0.0
	if row.Revenue != nil {
		rev = *row.Revenue
	}
	if row.Payout != nil {
		pay = *row.Payout
	}
	publisher := row.AFID
	if publisher == "" {
		publisher = row.ClientAFID
	}
	campaignID := row.SystemCampaignID
	if campaignID == 0 {
		campaignID = row.CampaignID
	}
	targetID := row.SystemTargetID
	if targetID == 0 {
		targetID = row.TargetID
	}
	if targetID == 0 {
		targetID = row.CallpixelsTargetID
	}
	externalID := row.UUID
	if externalID == "" {
		externalID = row.CallUUID
	}
	if externalID == "" {
		externalID = row.ID
	}
	// Phase 1.6 D8b Case A: Call Logs UI shows UUID (= external_call_id).
	// Fixture keys: uuid (primary), call_uuid, id. No cid field. Case B platform_display_id skipped.
	eventType := "incoming"
	if row.Status != "" {
		eventType = row.Status
	} else if row.Connected != nil && *row.Connected {
		eventType = "connected"
	}
	occurredAt := parseRetreaverTime(row.CreatedAt)
	updatedAt := parseRetreaverTime(row.UpdatedAt)
	return models.RemoteCall{
		ExternalCallID: externalID,
		CallerID:       row.Caller,
		State:          state.NormalizeState(row.CallerState),
		Revenue:        rev,
		Payout:         pay,
		CampaignID:     strconv.Itoa(campaignID),
		PublisherID:    publisher,
		TargetID:       strconv.Itoa(targetID),
		EventType:      eventType,
		OccurredAt:     occurredAt,
		UpdatedAt:      updatedAt,
	}
}

func parseRetreaverTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999Z",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// unwrapRetreaverCallPayload handles V2 list items: [{"call":{...}}] or {"calls":[...]}.
func unwrapRetreaverCallPayload(raw json.RawMessage) json.RawMessage {
	var wrapped struct {
		Call json.RawMessage `json:"call"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Call) > 0 {
		return wrapped.Call
	}
	return raw
}

func checkHTTPStatus(status int, body []byte, hdr http.Header, platform string) error {
	if status >= 200 && status < 300 {
		return nil
	}
	// Full body + safe headers (Location, etc.) — never truncate; credentials redacted via SanitizeErr on persist.
	return fmt.Errorf("%s", formatHTTPStatusError(status, body, hdr, platform))
}
