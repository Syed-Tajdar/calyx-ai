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

// Official Get Call Log (developers.ringba.com):
//   POST https://api.ringba.com/v2/{accountId}/calllogs
// Auth: Authorization: Token {apiToken}
// Body: reportStart / reportEnd (ISO8601 UTC) + size/offset (+ optional filters / valueColumns).
// Legacy CallLogs/Date + dateRange/callLog body returns HTTP 301 "endpoint moved".
const defaultRingbaAPIBase = "https://api.ringba.com/v2"

// Ringba has no last-mod cursor (Phase 2.A A3=NO): use call-time window + overlap + content-hash.
const ringbaOverlap = 2 * time.Hour
const ringbaDefaultLookbackDays = 30
const ringbaMaxPageSize = 1000 // official Get Call Log max; 1001 → 400

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
	if cs.Size < 1 || cs.Size > ringbaMaxPageSize {
		cs.Size = ringbaMaxPageSize
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
		Size:        ringbaMaxPageSize,
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
	size := cs.Size
	if size < 1 || size > ringbaMaxPageSize {
		size = ringbaMaxPageSize
	}
	// Official body: reportStart/reportEnd (+ size/offset). Empty filters = no attribute filter.
	// valueColumns: default pages omit money + geo tags; request State/Region for coverage.
	body := map[string]interface{}{
		"reportStart": cs.ReportStart,
		"reportEnd":   cs.ReportEnd,
		"filters":     []interface{}{},
		"size":        size,
		"offset":      cs.Offset,
		"valueColumns": []map[string]string{
			{"column": "inboundCallId"},
			{"column": "callDt"},
			{"column": "inboundPhoneNumber"},
			{"column": "conversionAmount"},
			{"column": "payoutAmount"},
			{"column": "campaignId"},
			{"column": "publisherId"},
			{"column": "targetId"},
			{"column": "tag:InboundNumber:State"},
			{"column": "tag:InboundNumber:Region"},
		},
	}
	bodyJSON, _ := json.Marshal(body)
	// Official: POST https://api.ringba.com/v2/{accountId}/calllogs
	reqURL := fmt.Sprintf("%s/%s/calllogs", ringbaAPIBase(), accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, false, err
	}
	// https://support.ringba.com/hc/en-us/articles/31661837696023-API-Access-Tokens
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	// Dual-window throttle: 5/sec AND 80/min, scoped per Ringba account_id (account-wide Get Logs bucket).
	ringbaLimiters.ForAccount(accountID).Wait()

	resp, respBody, err := a.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, respBody, resp.Header, "ringba"); err != nil {
		return nil, false, err
	}
	records, err := parseRingbaCallLogResponse(respBody)
	if err != nil {
		return nil, false, err
	}
	calls := make([]models.RemoteCall, 0, len(records))
	for _, raw := range records {
		calls = append(calls, mapRingbaCallLogEntry(raw))
	}
	return calls, len(records) >= size, nil
}

// parseRingbaCallLogResponse prefers official Get Call Log shape:
//   { isSuccessful, transactionId, report: { records, partialResult } }
// Legacy callLog.data / result.callLog.data kept for fixtures only.
func parseRingbaCallLogResponse(respBody []byte) ([]json.RawMessage, error) {
	var modern struct {
		Report struct {
			Records []json.RawMessage `json:"records"`
		} `json:"report"`
	}
	if err := json.Unmarshal(respBody, &modern); err == nil && modern.Report.Records != nil {
		return modern.Report.Records, nil
	}
	var wrapped struct {
		Result struct {
			CallLog struct {
				Data []json.RawMessage `json:"data"`
			} `json:"callLog"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &wrapped); err == nil && len(wrapped.Result.CallLog.Data) > 0 {
		return wrapped.Result.CallLog.Data, nil
	}
	var legacy struct {
		CallLog struct {
			Data []json.RawMessage `json:"data"`
		} `json:"callLog"`
	}
	if err := json.Unmarshal(respBody, &legacy); err != nil {
		return nil, fmt.Errorf("ringba: parse response: %w", err)
	}
	return legacy.CallLog.Data, nil
}

func mapRingbaCallLogEntry(raw json.RawMessage) models.RemoteCall {
	lookup := ringbaFieldLookup(raw)
	rev := firstFloat(lookup, "conversionAmount", "revenue")
	pay := firstFloat(lookup, "payoutAmount", "payout")
	callID := firstNonEmpty(lookup("inboundCallId"), lookup("callId"))
	stateVal := firstNonEmpty(
		lookup("tag:InboundNumber:State"),
		lookup("InboundNumber:State"),
		lookup("tag:InboundNumber:Region"), // queued sample key; Prefer State when both present
		lookup("InboundNumber:Region"),
		lookup("state"),
		lookup("geo:state"),
		lookup("user:state"),
		lookup("caller:state"),
	)
	caller := firstNonEmpty(lookup("inboundPhoneNumber"), lookup("caller"), lookup("caller:number"))
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

func ringbaFieldLookup(raw json.RawMessage) func(string) string {
	var legacy struct {
		Columns []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"columns"`
		Tags []struct {
			Name     string `json:"name"`
			Value    string `json:"value"`
			TagName  string `json:"tagName"`
			TagValue string `json:"tagValue"`
			TagType  string `json:"tagType"`
		} `json:"tags"`
	}
	_ = json.Unmarshal(raw, &legacy)
	if len(legacy.Columns) > 0 || len(legacy.Tags) > 0 {
		return func(name string) string {
			for _, c := range legacy.Columns {
				if strings.EqualFold(c.Name, name) {
					return c.Value
				}
			}
			for _, t := range legacy.Tags {
				tagName := firstNonEmpty(t.Name, t.TagName)
				tagVal := firstNonEmpty(t.Value, t.TagValue)
				if strings.EqualFold(tagName, name) {
					return tagVal
				}
				if t.TagType != "" && strings.EqualFold(t.TagType+":"+tagName, name) {
					return tagVal
				}
			}
			return ""
		}
	}

	var flat map[string]interface{}
	_ = json.Unmarshal(raw, &flat)
	return func(name string) string {
		for k, v := range flat {
			if strings.EqualFold(k, name) {
				return ringbaAnyString(v)
			}
		}
		// Nested tags array on flat records (when present).
		if tags, ok := flat["tags"].([]interface{}); ok {
			for _, item := range tags {
				tm, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				tagName := firstNonEmpty(ringbaAnyString(tm["name"]), ringbaAnyString(tm["tagName"]))
				tagVal := firstNonEmpty(ringbaAnyString(tm["value"]), ringbaAnyString(tm["tagValue"]))
				if strings.EqualFold(tagName, name) {
					return tagVal
				}
				tagType := ringbaAnyString(tm["tagType"])
				if tagType != "" && strings.EqualFold(tagType+":"+tagName, name) {
					return tagVal
				}
			}
		}
		return ""
	}
}

func ringbaAnyString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func firstFloat(lookup func(string) string, keys ...string) float64 {
	for _, k := range keys {
		if v := strings.TrimSpace(lookup(k)); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return f
			}
		}
	}
	return 0
}

func parseRingbaOccurredAt(lookup func(string) string) time.Time {
	// Official Get Call Log sorts/exposes callDt; dtStamp only on legacy sample shapes.
	for _, key := range []string{"callDt", "dtStamp", "callDateTime", "Call Date", "callDate"} {
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
	// New calllogs API returns callDt as epoch milliseconds.
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1_000_000_000_000 { // ms
			return time.UnixMilli(n).UTC()
		}
		if n > 1_000_000_000 { // sec
			return time.Unix(n, 0).UTC()
		}
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
