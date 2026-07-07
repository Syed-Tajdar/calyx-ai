package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
)

const ringbaAPIBase = "https://api.ringba.com"

type RingbaAdapter struct {
	http *HTTPClient
}

func NewRingbaAdapter(http *HTTPClient) *RingbaAdapter {
	return &RingbaAdapter{http: http}
}

func (a *RingbaAdapter) Platform() string { return "ringba" }

type ringbaCursor struct {
	ReportStart string `json:"report_start"`
	ReportEnd   string `json:"report_end"`
	Offset      int    `json:"offset"`
	Size        int    `json:"size"`
}

func (a *RingbaAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
	_, _, err := a.fetchPage(ctx, creds, ringbaDefaultCursor(""))
	return err
}

func (a *RingbaAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string) (PageResult, error) {
	cs := ringbaDefaultCursor(lastSyncAt)
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	}
	calls, hasMore, err := a.fetchPage(ctx, creds, cs)
	if err != nil {
		return PageResult{}, err
	}
	next := cs
	if hasMore {
		next.Offset += next.Size
	} else {
		next = ringbaDefaultCursor(time.Now().UTC().Format(time.RFC3339))
	}
	nextCursor, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{Calls: calls, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func ringbaDefaultCursor(lastSyncAt string) ringbaCursor {
	end := time.Now().UTC()
	start := end.Add(-24 * time.Hour)
	if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			start = t.Add(-5 * time.Minute)
		}
	}
	return ringbaCursor{
		ReportStart: start.Format(time.RFC3339),
		ReportEnd:   end.Format(time.RFC3339),
		Offset:      0,
		Size:        50,
	}
}

func (a *RingbaAdapter) fetchPage(ctx context.Context, creds map[string]string, cs ringbaCursor) ([]models.RemoteCall, bool, error) {
	token := creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID := creds["account_id"]
	if token == "" || accountID == "" {
		return nil, false, fmt.Errorf("ringba: api_token and account_id required")
	}
	body := map[string]interface{}{
		"reportStart": cs.ReportStart,
		"reportEnd":   cs.ReportEnd,
		"size":        cs.Size,
		"offset":      cs.Offset,
	}
	bodyJSON, _ := json.Marshal(body)
	reqURL := fmt.Sprintf("%s/%s/CallLogs/Date", ringbaAPIBase, accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	time.Sleep(220 * time.Millisecond)

	resp, respBody, err := a.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, respBody, "ringba"); err != nil {
		return nil, false, err
	}
	var payload struct {
		CallLog struct {
			Data []json.RawMessage `json:"data"`
		} `json:"callLog"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, false, fmt.Errorf("ringba: parse response: %w", err)
	}
	calls := make([]models.RemoteCall, 0, len(payload.CallLog.Data))
	for _, raw := range payload.CallLog.Data {
		calls = append(calls, mapRingbaCallLogEntry(raw))
	}
	return calls, len(payload.CallLog.Data) >= cs.Size, nil
}

func mapRingbaCallLogEntry(raw json.RawMessage) models.RemoteCall {
	var entry struct {
		Columns []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"columns"`
		Tags []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"tags"`
	}
	_ = json.Unmarshal(raw, &entry)
	lookup := func(name string) string {
		for _, c := range entry.Columns {
			if c.Name == name {
				return c.Value
			}
		}
		for _, t := range entry.Tags {
			if t.Name == name {
				return t.Value
			}
		}
		return ""
	}
	rev, _ := strconv.ParseFloat(lookup("revenue"), 64)
	pay, _ := strconv.ParseFloat(lookup("payout"), 64)
	callID := lookup("inboundCallId")
	if callID == "" {
		callID = lookup("callId")
	}
	stateVal := lookup("state")
	if stateVal == "" {
		stateVal = lookup("geo:state")
	}
	caller := lookup("caller")
	if caller == "" {
		caller = lookup("caller:number")
	}
	return models.RemoteCall{
		ExternalCallID: callID,
		CallerID:       caller,
		State:          state.NormalizeState(stateVal),
		Revenue:        rev,
		Payout:         pay,
		CampaignID:     lookup("campaignId"),
		PublisherID:    lookup("publisherId"),
		TargetID:       lookup("targetId"),
		EventType:      "incoming",
	}
}
