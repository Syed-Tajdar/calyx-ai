package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
)

type TrackDriveAdapter struct {
	http *HTTPClient
}

func NewTrackDriveAdapter(http *HTTPClient) *TrackDriveAdapter {
	return &TrackDriveAdapter{http: http}
}

func (a *TrackDriveAdapter) Platform() string { return "trackdrive" }

type trackdriveCursor struct {
	CreatedAtFrom string `json:"created_at_from"`
	Cursor        string `json:"cursor"`
}

func (a *TrackDriveAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
	_, _, _, err := a.fetch(ctx, creds, trackdriveCursor{Cursor: "-1"})
	return err
}

func (a *TrackDriveAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string) (PageResult, error) {
	cs := trackdriveCursor{Cursor: "-1"}
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	} else {
		from := time.Now().UTC().Add(-24 * time.Hour)
		if lastSyncAt != "" {
			if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
				from = t.Add(-5 * time.Minute)
			}
		}
		cs.CreatedAtFrom = from.Format("2006-01-02 15:04:05 -0700")
	}
	calls, nextCursor, hasMore, err := a.fetch(ctx, creds, cs)
	if err != nil {
		return PageResult{}, err
	}
	next := trackdriveCursor{CreatedAtFrom: cs.CreatedAtFrom, Cursor: nextCursor}
	enc, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{Calls: calls, NextCursor: enc, HasMore: hasMore}, nil
}

func (a *TrackDriveAdapter) fetch(ctx context.Context, creds map[string]string, cs trackdriveCursor) ([]models.RemoteCall, string, bool, error) {
	subdomain := creds["subdomain"]
	publicKey := creds["public_key"]
	privateKey := creds["private_key"]
	authToken := creds["auth_token"]
	if subdomain == "" {
		return nil, "", false, fmt.Errorf("trackdrive: subdomain required")
	}
	base := fmt.Sprintf("https://%s.trackdrive.com/api/v1/calls", subdomain)
	q := url.Values{}
	q.Set("per_page", "25")
	q.Set("order", "created_at")
	q.Set("order_dir", "asc")
	if cs.Cursor != "" {
		q.Set("cursor", cs.Cursor)
	}
	if cs.CreatedAtFrom != "" {
		q.Set("created_at_from", cs.CreatedAtFrom)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q.Encode(), nil)
	if err != nil {
		return nil, "", false, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", authToken)
	} else if publicKey != "" && privateKey != "" {
		token := base64.StdEncoding.EncodeToString([]byte(publicKey + ":" + privateKey))
		req.Header.Set("Authorization", "Basic "+token)
	} else {
		return nil, "", false, fmt.Errorf("trackdrive: public_key+private_key or auth_token required")
	}
	resp, body, err := a.http.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, body, "trackdrive"); err != nil {
		return nil, "", false, err
	}
	var payload struct {
		Calls    []json.RawMessage `json:"calls"`
		Metadata struct {
			NextCursor string `json:"next_cursor"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		var arr []json.RawMessage
		if err2 := json.Unmarshal(body, &arr); err2 == nil {
			payload.Calls = arr
		} else {
			return nil, "", false, fmt.Errorf("trackdrive: parse: %w", err)
		}
	}
	calls := make([]models.RemoteCall, 0, len(payload.Calls))
	for _, raw := range payload.Calls {
		calls = append(calls, mapTrackDriveCall(raw))
	}
	next := payload.Metadata.NextCursor
	hasMore := next != "" && next != "0"
	return calls, next, hasMore, nil
}

func mapTrackDriveCall(raw json.RawMessage) models.RemoteCall {
	var row struct {
		UUID           string  `json:"uuid"`
		CallerNumber   string  `json:"caller_number"`
		CallerState    string  `json:"caller_state"`
		Revenue        float64 `json:"revenue"`
		BuyerRevenue   float64 `json:"buyer_revenue"`
		Payout         float64 `json:"payout"`
		TrafficPayout  float64 `json:"traffic_source_payout"`
		OfferID        int     `json:"offer_id"`
		TrafficID      int     `json:"traffic_source_id"`
		BuyerID        int     `json:"buyer_id"`
		Status         string  `json:"status"`
		BuyerConverted bool    `json:"buyer_converted"`
	}
	_ = json.Unmarshal(raw, &row)
	rev := row.Revenue
	if rev == 0 {
		rev = row.BuyerRevenue
	}
	pay := row.Payout
	if pay == 0 {
		pay = row.TrafficPayout
	}
	eventType := row.Status
	if row.BuyerConverted {
		eventType = "buyer_converted"
	}
	return models.RemoteCall{
		ExternalCallID: row.UUID,
		CallerID:       row.CallerNumber,
		State:          state.NormalizeState(row.CallerState),
		Revenue:        rev,
		Payout:         pay,
		CampaignID:     strconv.Itoa(row.OfferID),
		PublisherID:    strconv.Itoa(row.TrafficID),
		TargetID:       strconv.Itoa(row.BuyerID),
		EventType:      eventType,
	}
}
