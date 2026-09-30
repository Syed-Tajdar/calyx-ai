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

// retreaverCursor tracks an in-progress frozen window (Stage 4 R1 / C1).
//
// Before R1 this held only {updated_at_start, page} and the request carried no end bound, so the
// result set shifted under the page walk (K1). It now carries both edges of the window, fixed
// when the window opened, so a crash or a slow walk resumes exactly the same set.
//
// Migration: an old cursor supplies UpdatedAtStart only; it becomes WindowStart and a fresh
// WindowEnd is computed on the next tick. CreatedAtStart is the pre-D9 legacy field.
type retreaverCursor struct {
	WindowStart string `json:"window_start,omitempty"`
	WindowEnd   string `json:"window_end,omitempty"`
	Page        int    `json:"page"`

	UpdatedAtStart string `json:"updated_at_start,omitempty"` // legacy migrate-only
	CreatedAtStart string `json:"created_at_start,omitempty"` // legacy migrate-only
}

func (cs *retreaverCursor) normalize() {
	// Oldest legacy form first, then the R0 form, then the R1 form.
	if cs.UpdatedAtStart == "" && cs.CreatedAtStart != "" {
		cs.UpdatedAtStart = cs.CreatedAtStart
	}
	if cs.WindowStart == "" && cs.UpdatedAtStart != "" {
		cs.WindowStart = cs.UpdatedAtStart
	}
	cs.CreatedAtStart = ""
	cs.UpdatedAtStart = ""
	if cs.Page < 1 {
		cs.Page = 1
	}
}

// inProgress reports whether the cursor describes a window that is still being walked.
func (cs *retreaverCursor) inProgress() bool {
	return cs.WindowStart != "" && cs.WindowEnd != ""
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
	now := time.Now().UTC()
	_, _, _, err := a.fetchWindowPage(ctx, creds, now.Add(-time.Hour), now, 1, 1)
	return err
}

// FetchPage walks one page of a frozen `[window_start, window_end)` window (Stage 4 R1).
//
// On the first page of a window the `Total` header decides whether the window is walkable within
// the page cap; if not the window END is shrunk (C1) and page 1 re-fetched, repeatedly, so no row
// is ever left beyond a cap. The window only advances once its last page has been read, which is
// also how a Total mismatch is made to block rather than warn (C2).
func (a *RetreaverAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string, lookbackDays int) (PageResult, error) {
	const perPage = 100 // R0.3 Q4: this is the API maximum; above it the API silently returns 25.

	cs := retreaverCursor{Page: 1}
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	}
	cs.normalize()

	now := time.Now().UTC()
	var start, end time.Time
	if cs.inProgress() {
		// Resume the same frozen window — a crash or restart must not move the goalposts.
		s, err1 := time.Parse(time.RFC3339, cs.WindowStart)
		e, err2 := time.Parse(time.RFC3339, cs.WindowEnd)
		if err1 == nil && err2 == nil && s.Before(e) {
			start, end = s.UTC(), e.UTC()
		}
	}
	if start.IsZero() {
		anchor := lastSyncAt
		if cs.WindowStart != "" {
			anchor = cs.WindowStart // finished window: continue from where it ended
		}
		start, end = BuildRetreaverWindow(now, anchor, lookbackDays)
		cs.Page = 1
	}

	calls, total, hasMore, err := a.fetchWindowPage(ctx, creds, start, end, cs.Page, perPage)
	if err != nil {
		return PageResult{}, err
	}

	// C1 — on page 1, split rather than truncate. Shrinking the end is safe: the part dropped is
	// simply not covered yet, and the next window starts exactly where this one ends.
	maxPages := RetreaverMaxPagesPerWindow()
	splits := 0
	for cs.Page == 1 && !retreaverWindowFits(total, perPage, maxPages) {
		newEnd, ok := shrinkRetreaverWindow(start, end)
		if !ok {
			break // at the floor; accept and walk it
		}
		end = newEnd
		splits++
		calls, total, hasMore, err = a.fetchWindowPage(ctx, creds, start, end, 1, perPage)
		if err != nil {
			return PageResult{}, err
		}
		if splits > 40 { // guard against pathological clocks
			break
		}
	}

	next := retreaverCursor{WindowStart: start.Format(time.RFC3339), Page: cs.Page}
	if hasMore {
		// Stay inside this window; keep the frozen end so later pages see the same set.
		next.WindowEnd = end.Format(time.RFC3339)
		next.Page = cs.Page + 1
	} else {
		// Window complete. Next tick starts at this window's end minus the overlap; leaving
		// WindowEnd empty marks it finished so a fresh end is computed then.
		next.WindowStart = end.Add(-RetreaverWindowOverlap()).UTC().Format(time.RFC3339)
		next.Page = 1
	}

	nextCursor, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{
		Calls:        calls,
		NextCursor:   nextCursor,
		HasMore:      hasMore,
		RequestCount: 1 + splits,
		WindowStart:  start,
		WindowEnd:    end,
		WindowCapped: splits > 0,
	}, nil
}

// fetchWindowPage requests one page of a bounded [start, end) window.
//
// Returns the mapped calls, the API's `Total` for the window (0 when absent) and whether more
// pages remain. R0.3 proved `updated_at_end` is honoured, which is what makes the window frozen.
func (a *RetreaverAdapter) fetchWindowPage(
	ctx context.Context,
	creds map[string]string,
	start, end time.Time,
	page, perPage int,
) ([]models.RemoteCall, int, bool, error) {
	apiKey := creds["api_key"]
	companyID := creds["company_id"]
	if apiKey == "" || companyID == "" {
		return nil, 0, false, fmt.Errorf("retreaver: api_key and company_id required")
	}
	if page < 1 {
		page = 1
	}

	q := url.Values{}
	// The key can only travel in the query string: header auth returns 401 (R1 B2(1),
	// https://retreaver.github.io/core-api-docs/#authentication). Every error path that can
	// carry this URL must therefore stay redacted — see retreaver_redaction_test.go.
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	q.Set("updated_at_start", start.UTC().Format(time.RFC3339))
	q.Set("updated_at_end", end.UTC().Format(time.RFC3339))
	q.Set("sort_by", "updated_at")
	// R0.3 Q3: order=asc is ignored for updated_at; the API always sorts desc. Stated
	// explicitly so nobody "fixes" this into asc and assumes stable paging from it.
	q.Set("order", "desc")
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, retreaverBaseURL()+"?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, false, err
	}
	// Per-account throttle: Retreaver publishes no numeric limit, only a documented 429 and
	// "highly concurrent requests are frowned upon".
	retreaverLimiters.ForAccount(companyID).Wait()

	resp, body, err := a.http.Do(req)
	if err != nil {
		return nil, 0, false, err
	}
	if err := checkHTTPStatus(resp.StatusCode, body, resp.Header, "retreaver"); err != nil {
		return nil, 0, false, err
	}

	// `Total` is the exact row count for this window (R0.3 Q6) — used to decide splitting and to
	// detect a short walk. `Link` is deliberately NOT read: it echoes the api_key.
	total := 0
	if v := strings.TrimSpace(resp.Header.Get("Total")); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			total = n
		}
	}

	var payload struct {
		Calls []json.RawMessage `json:"calls"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		var arr []json.RawMessage
		if err2 := json.Unmarshal(body, &arr); err2 != nil {
			return nil, 0, false, fmt.Errorf("retreaver: parse response: %w", err)
		}
		payload.Calls = arr
	}
	calls := make([]models.RemoteCall, 0, len(payload.Calls))
	for _, raw := range payload.Calls {
		calls = append(calls, mapRetreaverCall(raw))
	}

	hasMore := len(payload.Calls) >= perPage
	if total > 0 {
		// Prefer the authoritative count when the API supplies it.
		hasMore = page*perPage < total
	}
	return calls, total, hasMore, nil
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

		// Stage 4 R1 / D — mapped from R0.3's live field inventory (71 fields on a real call).
		// See the C4 field-meaning table in docs/retreaver-stage4-report.md: these must mean the
		// same thing as their Ringba counterparts, because the dashboard shows both side by side.
		Converted          *bool    `json:"converted"`
		TotalDuration      *uint32  `json:"total_duration"`
		DialedCallDuration *uint32  `json:"dialed_call_duration"`
		ProfitGross        *float64 `json:"profit_gross"`
		Duplicate          *bool    `json:"duplicate"`
		PayableDuplicate   *bool    `json:"payable_duplicate"`
		TargetName         string   `json:"target_name"`
		AffiliateName      string   `json:"affiliate_name"`
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

	// Conversion state. R0.3 sampled 100 real calls: `converted` is always present, and
	// `revenue`/`payout` are null on exactly the 60 unconverted calls and populated on exactly
	// the 40 converted ones — never zero. So a null amount is unambiguous once `converted` is
	// mapped alongside it, and no Nullable migration is required (C3 proposal).
	converted := row.Converted != nil && *row.Converted
	connected := row.Connected != nil && *row.Connected

	// Retreaver reports three duplicate flags. `duplicate` is the lead-level verdict that
	// matches Ringba's `isDuplicate`; `payable_duplicate` is the narrower "we don't pay the
	// publisher" case. Rule (b) excludes duplicates from revenue but keeps them in call counts,
	// so the lead-level flag is the correct one — fall back to payable_duplicate when absent.
	platDup := false
	if row.Duplicate != nil {
		platDup = *row.Duplicate
	} else if row.PayableDuplicate != nil {
		platDup = *row.PayableDuplicate
	}

	var profit float64
	if row.ProfitGross != nil {
		profit = *row.ProfitGross
	}
	// DurationSeconds = connected talk time (Ringba: connected duration).
	// CallLengthSeconds = whole-call length (Ringba: call duration).
	var talk, whole uint32
	if row.DialedCallDuration != nil {
		talk = *row.DialedCallDuration
	}
	if row.TotalDuration != nil {
		whole = *row.TotalDuration
	}

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

		HasConverted:        converted,
		HasConvertedPresent: row.Converted != nil,
		HasConnected:        connected,
		DurationSeconds:     talk,
		CallLengthSeconds:   whole,
		ProfitGross:         profit,
		PlatformDuplicate:   platDup,
		TargetName:          row.TargetName,
		PublisherName:       row.AffiliateName,
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
