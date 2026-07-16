package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
)

const defaultRingbaAPIBase = "https://api.ringba.com"

// Ringba has no last-mod cursor (Phase 2.A A3=NO): use call-time window + overlap + content-hash.
const ringbaOverlap = 2 * time.Hour
const ringbaDefaultLookbackDays = 30

func ringbaAPIBase() string {
	if v := os.Getenv("RINGBA_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultRingbaAPIBase
}

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
	_, _, err := a.fetchPage(ctx, creds, ringbaDefaultCursor("", ringbaDefaultLookbackDays))
	return err
}

func (a *RingbaAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string, lookbackDays int) (PageResult, error) {
	cs := ringbaDefaultCursor(lastSyncAt, lookbackDays)
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	}
	if cs.Size < 1 {
		cs.Size = 50
	}
	calls, hasMore, err := a.fetchPage(ctx, creds, cs)
	if err != nil {
		return PageResult{}, err
	}
	next := cs
	if hasMore {
		next.Offset += next.Size
	} else {
		// Slide window forward with overlap so late revenue updates re-enter the fetch set.
		next = ringbaDefaultCursor(time.Now().UTC().Format(time.RFC3339), lookbackDays)
	}
	nextCursor, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{Calls: calls, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func ringbaLookbackDays(lookbackDays int) int {
	days := lookbackDays
	if days < 1 {
		days = ringbaDefaultLookbackDays
	}
	if days > 365 {
		days = 365
	}
	return days
}

func ringbaDefaultCursor(lastSyncAt string, lookbackDays int) ringbaCursor {
	end := time.Now().UTC()
	start := end.Add(-time.Duration(ringbaLookbackDays(lookbackDays)) * 24 * time.Hour)
	if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			start = t.Add(-ringbaOverlap)
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
	reqURL := fmt.Sprintf("%s/%s/CallLogs/Date", ringbaAPIBase(), accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, false, err
	}
	// Post–Jun 2025: API access token in Authorization (Bearer deprecated).
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	// Dual-window throttle: 5/sec AND 80/min, scoped per Ringba account_id.
	ringbaLimiters.ForAccount(accountID).Wait()

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
			if strings.EqualFold(c.Name, name) {
				return c.Value
			}
		}
		for _, t := range entry.Tags {
			if strings.EqualFold(t.Name, name) {
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
	if caller == "" {
		caller = lookup("inboundPhoneNumber")
	}

	occurred := parseRingbaOccurredAt(lookup)
	campaign := firstNonEmpty(lookup("campaignId"), lookup("campaignID"))
	publisher := firstNonEmpty(lookup("publisherId"), lookup("publisherID"))
	target := firstNonEmpty(lookup("targetId"), lookup("targetID"))

	return models.RemoteCall{
		ExternalCallID: callID,
		CallerID:       caller,
		State:          state.NormalizeState(stateVal),
		Revenue:        rev,
		Payout:         pay,
		CampaignID:     campaign,
		PublisherID:    publisher,
		TargetID:       target,
		EventType:      "incoming",
		OccurredAt:     occurred,
	}
}

func parseRingbaOccurredAt(lookup func(string) string) time.Time {
	for _, key := range []string{"callDt", "callDateTime", "Call Date", "callDate"} {
		if t := parseFlexibleTime(lookup(key)); !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

func parseFlexibleTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"01/02/2006 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
