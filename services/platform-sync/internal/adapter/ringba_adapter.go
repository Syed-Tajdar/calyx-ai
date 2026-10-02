package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state"
)

// Official Get Call Log (developers.ringba.com):
//
//	POST https://api.ringba.com/v2/{accountId}/calllogs
//
// Auth: Authorization: Token {apiToken}
// Body: reportStart / reportEnd (ISO8601 UTC) + size/offset (+ optional filters / valueColumns).
// Legacy CallLogs/Date + dateRange/callLog body returns HTTP 301 "endpoint moved".
const defaultRingbaAPIBase = "https://api.ringba.com/v2"

// Ringba has no last-mod cursor (Phase 2.A A3=NO): use call-time window + overlap + content-hash.
// Overlap default is RingbaIncrementalOverlap() (6h) — see ringba_window.go.
const ringbaDefaultLookbackDays = 30
const ringbaMaxPageSize = 1000 // official Get Call Log max; 1001 → 400

// Deprecated: use RingbaIncrementalOverlap(). Kept so older tests that referenced
// ringbaOverlap compile until updated — value matches the pre-S1A constant for docs only.
const ringbaOverlap = 2 * time.Hour

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
	now := ringbaNow()
	start, end, capped := buildRingbaIncrementalWindow(lastSyncAt, lookbackDays, now)
	if capped {
		windowCappedTotal.Add(1)
		slog.Warn("ringba incremental window_capped",
			"window_start", start.UTC().Format(time.RFC3339),
			"window_end", end.UTC().Format(time.RFC3339),
			"lookback_days", ringbaLookbackDays(lookbackDays),
			"window_capped_total", windowCappedTotal.Load())
	}
	// Stored sync_cursor window no longer overrides bounds (S0 F003b). Cursor may
	// still be present for state visibility but ReportStart/ReportEnd are ignored.
	_ = cursor

	calls, stats, err := a.fetchWindowed(ctx, creds, start, end)
	if err != nil {
		return PageResult{}, err
	}
	next := ringbaCursor{
		ReportStart: start.Format(time.RFC3339),
		ReportEnd:   end.Format(time.RFC3339),
		Offset:      0,
		Size:        ringbaMaxPageSize,
	}
	nextCursor, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{
		Calls:                 calls,
		NextCursor:            nextCursor,
		HasMore:               false,
		RequestCount:          stats.Requests,
		OffsetFallbackWindows: stats.OffsetFallbackWindows,
		OffsetFallbackRows:    stats.OffsetFallbackRows,
		WindowStart:           start,
		WindowEnd:             end,
		WindowCapped:          capped,
	}, nil
}

// FetchRange pulls an explicit [start, end) window (sweep / manual backfill).
func (a *RingbaAdapter) FetchRange(ctx context.Context, creds map[string]string, start, end time.Time) (PageResult, error) {
	start = start.UTC()
	end = end.UTC()
	if !start.Before(end) {
		return PageResult{}, fmt.Errorf("ringba FetchRange: start must be before end")
	}
	calls, stats, err := a.fetchWindowed(ctx, creds, start, end)
	if err != nil {
		return PageResult{}, err
	}
	next := ringbaCursor{
		ReportStart: start.Format(time.RFC3339),
		ReportEnd:   end.Format(time.RFC3339),
		Offset:      0,
		Size:        ringbaMaxPageSize,
	}
	nextCursor, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{
		Calls:                 calls,
		NextCursor:            nextCursor,
		HasMore:               false,
		RequestCount:          stats.Requests,
		OffsetFallbackWindows: stats.OffsetFallbackWindows,
		OffsetFallbackRows:    stats.OffsetFallbackRows,
		WindowStart:           start,
		WindowEnd:             end,
	}, nil
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

// ringbaDefaultCursor remains for TestAuth empty-cursor probes only.
func ringbaDefaultCursor(lastSyncAt string, lookbackDays int) ringbaCursor {
	start, end, _ := buildRingbaIncrementalWindow(lastSyncAt, lookbackDays, ringbaNow())
	return ringbaCursor{
		ReportStart: start.Format(time.RFC3339),
		ReportEnd:   end.Format(time.RFC3339),
		Offset:      0,
		Size:        ringbaMaxPageSize,
	}
}

func (a *RingbaAdapter) fetchPage(ctx context.Context, creds map[string]string, cs ringbaCursor) ([]models.RemoteCall, bool, error) {
	return a.fetchPageWith(ctx, creds, cs, nil)
}

func (a *RingbaAdapter) fetchPageWith(ctx context.Context, creds map[string]string, cs ringbaCursor, extraCols []string) ([]models.RemoteCall, bool, error) {
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
	vc := ringbaValueColumns()
	for _, col := range extraCols {
		vc = append(vc, map[string]string{"column": col})
	}
	body := map[string]interface{}{
		"reportStart":  cs.ReportStart,
		"reportEnd":    cs.ReportEnd,
		"filters":      []interface{}{},
		"size":         size,
		"offset":       cs.Offset,
		"valueColumns": vc,
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
	// Dual-window throttle: 5/sec AND 75/min, scoped per Ringba account_id (account-wide Get Logs bucket).
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
		call, err := mapRingbaCallLogEntry(raw)
		if err != nil {
			return nil, false, err
		}
		calls = append(calls, call)
	}
	return calls, len(records) >= size, nil
}

// orderByColumns was probed live: {"column":"callDt","direction":"asc"} returns
// HTTP 422 "Invalid sort column callDt". Empty orderByColumns is accepted in
// community samples, but a documented valid sort column for this account was
// not confirmed. Windowed single-page fetches do not rely on sort order.

func ringbaValueColumns() []map[string]string {
	names := []string{
		"inboundCallId",
		"callDt",
		"inboundPhoneNumber",
		"conversionAmount",
		"payoutAmount",
		"campaignId",
		"campaignName",
		"publisherId",
		"publisherName",
		"targetId",
		"targetName",
		"buyer",
		"buyerId",
		"connectedCallLengthInSeconds",
		"callLengthInSeconds",
		"hasRecording",
		"hasConverted",
		"hasConnected",
		"wasBlocked",
		"blockReason",
		"wasConversionAdjusted",
		"wasPayoutAdjusted",
		"ringTreeWinningBidDynamicDuration",
		"ringTreeWinningBidMaxDynamicDuration",
		"isDuplicate",
		"profitGross",
		"tag:InboundNumber:State",
		"tag:InboundNumber:Region",
	}
	vc := make([]map[string]string, 0, len(names))
	for _, name := range names {
		vc = append(vc, map[string]string{"column": name})
	}
	return vc
}

// parseRingbaCallLogResponse prefers official Get Call Log shape:
//
//	{ isSuccessful, transactionId, report: { records, partialResult } }
//
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

func mapRingbaCallLogEntry(raw json.RawMessage) (models.RemoteCall, error) {
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

	connectedLen := firstUint32(lookup, "connectedCallLengthInSeconds")
	callLen := firstUint32(lookup, "callLengthInSeconds")
	hasRec, recKnown := parseRingbaBool(lookup("hasRecording"))
	hasConv, hasConvPresent := parseRingbaBool(lookup("hasConverted"))
	// Requested columns: a missing key is Ringba omitting false, not "never asked".
	hasConn, _ := parseRingbaBool(lookup("hasConnected"))
	wasBlocked, _ := parseRingbaBool(lookup("wasBlocked"))
	wasConvAdj, _ := parseRingbaBool(lookup("wasConversionAdjusted"))
	wasPayAdj, _ := parseRingbaBool(lookup("wasPayoutAdjusted"))
	offline, offlinePresent := parseRingbaBool(lookup("offlineConversionUploaded"))
	platDup, _ := parseRingbaBool(lookup("isDuplicate"))
	required, err := ringbaOptionalSeconds(raw, "ringTreeWinningBidDynamicDuration")
	if err != nil {
		return models.RemoteCall{}, err
	}
	maxRequired, err := ringbaOptionalSeconds(raw, "ringTreeWinningBidMaxDynamicDuration")
	if err != nil {
		return models.RemoteCall{}, err
	}

	return models.RemoteCall{
		ExternalCallID:                   callID,
		CallerID:                         caller,
		State:                            state.NormalizeState(stateVal),
		Revenue:                          rev,
		Payout:                           pay,
		CampaignID:                       campaign,
		PublisherID:                      publisher,
		TargetID:                         target,
		EventType:                        "incoming",
		OccurredAt:                       occurred,
		PlatformDuplicate:                platDup,
		CampaignName:                     lookup("campaignName"),
		PublisherName:                    lookup("publisherName"),
		TargetName:                       lookup("targetName"),
		BuyerID:                          firstNonEmpty(lookup("buyerId"), lookup("buyerID")),
		BuyerName:                        lookup("buyer"),
		DurationSeconds:                  connectedLen,
		CallLengthSeconds:                callLen,
		HasRecording:                     hasRec,
		RecordingKnown:                   recKnown,
		ProfitGross:                      firstFloat(lookup, "profitGross"),
		HasConverted:                     hasConv,
		HasConvertedPresent:              hasConvPresent,
		HasConnected:                     hasConn,
		WasBlocked:                       wasBlocked,
		BlockReason:                      lookup("blockReason"),
		WasConversionAdjusted:            wasConvAdj,
		WasPayoutAdjusted:                wasPayAdj,
		OfflineConversionUploadedPresent: offlinePresent,
		OfflineConversionUploaded:        offline,
		RequiredDurationSeconds:          required,
		MaxRequiredDurationSeconds:       maxRequired,
	}, nil
}

// ringbaOptionalSeconds maps one timespan column.
// Absent key → nil. Present 0 → pointer to 0. Present N → pointer to N.
// A present value that is not a whole number of seconds is an error.
func ringbaOptionalSeconds(raw json.RawMessage, key string) (*uint32, error) {
	val, present, null, err := ringbaKeyState(raw, key)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	if null {
		return nil, fmt.Errorf("ringba: %s is present but null", key)
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return nil, fmt.Errorf("ringba: %s is present but empty", key)
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return nil, fmt.Errorf("ringba: %s=%q is not a number of seconds", key, val)
	}
	if f < 0 || f != math.Trunc(f) || f > math.MaxUint32 {
		return nil, fmt.Errorf("ringba: %s=%v is not a whole number of seconds", key, f)
	}
	n := uint32(f)
	return &n, nil
}

func ringbaKeyState(raw json.RawMessage, key string) (val string, present bool, null bool, err error) {
	var flat map[string]interface{}
	if err := json.Unmarshal(raw, &flat); err != nil {
		return "", false, false, fmt.Errorf("ringba: parse record: %w", err)
	}
	if cols, ok := flat["columns"].([]interface{}); ok && len(cols) > 0 {
		for _, item := range cols {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if strings.EqualFold(ringbaAnyString(m["name"]), key) {
				if m["value"] == nil {
					return "", true, true, nil
				}
				return ringbaAnyString(m["value"]), true, false, nil
			}
		}
		return "", false, false, nil
	}
	for k, v := range flat {
		if !strings.EqualFold(k, key) {
			continue
		}
		if v == nil {
			return "", true, true, nil
		}
		return ringbaAnyString(v), true, false, nil
	}
	return "", false, false, nil
}

func firstUint32(lookup func(string) string, keys ...string) uint32 {
	for _, k := range keys {
		if v := strings.TrimSpace(lookup(k)); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil && f >= 0 {
				return uint32(f)
			}
		}
	}
	return 0
}

// parseRingbaBool returns (value, present). present=false when the key was absent/empty.
func parseRingbaBool(raw string) (value bool, present bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return false, false
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes", "y":
		return true, true
	case "false", "0", "no", "n":
		return false, true
	default:
		return false, true
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

// LookupCall finds one inboundCallId in a 7-day Get Call Log window.
func (a *RingbaAdapter) LookupCall(ctx context.Context, creds map[string]string, inboundCallID string) (models.RemoteCall, error) {
	inboundCallID = strings.TrimSpace(inboundCallID)
	if inboundCallID == "" {
		return models.RemoteCall{}, fmt.Errorf("ringba: inbound call id required")
	}
	end := time.Now().UTC().Add(2 * time.Hour)
	cs := ringbaCursor{
		ReportStart: end.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
		ReportEnd:   end.Format(time.RFC3339),
		Offset:      0,
		Size:        ringbaMaxPageSize,
	}
	for page := 0; page < 8; page++ {
		calls, hasMore, err := a.fetchPageWith(ctx, creds, cs, []string{
			"hasConnected", "wasBlocked", "blockReason",
			"offlineConversionUploaded", "wasConversionAdjusted", "wasPayoutAdjusted",
		})
		if err != nil {
			return models.RemoteCall{}, err
		}
		for _, c := range calls {
			if c.ExternalCallID == inboundCallID {
				return c, nil
			}
		}
		if !hasMore {
			break
		}
		cs.Offset += cs.Size
	}
	return models.RemoteCall{}, fmt.Errorf("ringba: call not found in 7d window")
}

type RingbaConversionWebhook struct {
	ID   string
	Name string
}

func (a *RingbaAdapter) ListConversionWebhooks(ctx context.Context, creds map[string]string) ([]RingbaConversionWebhook, error) {
	token, accountID, err := ringbaCreds(creds)
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("%s/%s/WebHooks", ringbaAPIBase(), accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+token)
	ringbaLimiters.ForAccount(accountID).Wait()
	resp, respBody, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := checkHTTPStatus(resp.StatusCode, respBody, resp.Header, "ringba"); err != nil {
		return nil, err
	}
	var parsed struct {
		WebHooks []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Enabled     bool   `json:"enabled"`
			WebHookType string `json:"webHookType"`
		} `json:"webHooks"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("ringba: parse webhooks: %w", err)
	}
	out := make([]RingbaConversionWebhook, 0, len(parsed.WebHooks))
	for _, wh := range parsed.WebHooks {
		if !wh.Enabled {
			continue
		}
		if wh.WebHookType != "" && !strings.EqualFold(wh.WebHookType, "conversion") {
			continue
		}
		out = append(out, RingbaConversionWebhook{ID: wh.ID, Name: wh.Name})
	}
	return out, nil
}

// FireInboundConversion GETs the Ringba inbound conversion hook (Phase G proven path).
func (a *RingbaAdapter) FireInboundConversion(ctx context.Context, hookID, inboundCallID string, amount float64) error {
	hookID = strings.TrimSpace(hookID)
	if hookID == "" {
		return fmt.Errorf("ringba: conversion webhook id required")
	}
	q := url.Values{}
	q.Set("call_id", inboundCallID)
	q.Set("call_revenue", strconv.FormatFloat(amount, 'f', 2, 64))
	q.Set("converted", "yes")
	fireURL := "https://webhook.ringba.com/hook/" + hookID + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fireURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ringba: conversion webhook HTTP %d", resp.StatusCode)
	}
	return nil
}

// VoidCall POSTs official …/calls/void (G-void proven body, including Ringba's voidConverionAmount spelling).
func (a *RingbaAdapter) VoidCall(ctx context.Context, creds map[string]string, inboundCallID string, convAmt, payAmt float64) (string, error) {
	token, accountID, err := ringbaCreds(creds)
	if err != nil {
		return "", err
	}
	body := map[string]interface{}{
		"inboundCallId":       inboundCallID,
		"voidReason":          "CALYX write-back void",
		"voidConversion":      true,
		"voidPayout":          true,
		"voidConverionAmount": convAmt,
		"voidPayoutAmount":    payAmt,
	}
	raw, _ := json.Marshal(body)
	reqURL := fmt.Sprintf("%s/%s/calls/void", ringbaAPIBase(), accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")
	ringbaLimiters.ForAccount(accountID).Wait()
	resp, respBody, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	if err := checkHTTPStatus(resp.StatusCode, respBody, resp.Header, "ringba"); err != nil {
		return "", err
	}
	var parsed struct {
		TransactionID string `json:"transactionId"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return parsed.TransactionID, nil
}

func ringbaCreds(creds map[string]string) (token, accountID string, err error) {
	token = creds["api_token"]
	if token == "" {
		token = creds["api_key"]
	}
	accountID = creds["account_id"]
	if token == "" || accountID == "" {
		return "", "", fmt.Errorf("ringba: api_token and account_id required")
	}
	return token, accountID, nil
}
