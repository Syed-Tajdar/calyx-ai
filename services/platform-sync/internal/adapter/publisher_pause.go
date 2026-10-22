package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
)

// Publisher pause transport (v2 Phase 2D, docs/v2-2d-contract.md "Platform calls").
//
// Ringba (developers.ringba.com, Publishers › Edit a Publisher):
//
//	GET   https://api.ringba.com/v2/{accountId}/Publishers/{publisherId}   → "enabled"
//	PATCH https://api.ringba.com/v2/{accountId}/Publishers/{publisherId}   {"enabled":<bool>}
//	Authorization: Token {apiToken}
//
// Retreaver (learn.retreaver.com/api/affiliates):
//
//	GET /api/v1/affiliates/afid/{afid}.json   (404 on some accounts → list /api/v1/affiliates.json)
//	GET /api/v1/affiliates/{id}.json
//	PUT /api/v1/affiliates/{id}.json          {"affiliate":{"paused":<bool>}}
//	?api_key=…&company_id=…
//
// The bodies below are the only bodies these clients can send: the request types have exactly
// one field each, so no other publisher/affiliate setting (payout, campaigns, routing) can be
// changed through them. Nothing here deletes anything.
//
// Retry: only 429 and 5xx, at most 3 attempts, backoff honouring Retry-After (capped). A 4xx is
// returned at once; a transport error is returned at once too (the write may have landed — the
// read-back decides). Every error is redacted: Retreaver's key travels in the query string.

// ErrPublisherNotFound is returned when the platform answers 404 for the publisher/affiliate,
// or the afid is not in the affiliate list.
var ErrPublisherNotFound = errors.New("publisher not found on the platform")

// ErrInvalidPlatformID is returned before any request when an id is not a short safe token.
var ErrInvalidPlatformID = errors.New("invalid platform id")

// ValidPlatformID reports whether id is safe to place in a URL path segment: 1–64 characters of
// letters, digits, '-', '_' or '.', and never "..". Ids with '/', '?', '#', whitespace or any
// other character are refused before a request is built.
func ValidPlatformID(id string) bool {
	if id == "" || len(id) > 64 || strings.Contains(id, "..") {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// pauseHTTP is the shared retrying sender for both pause clients.
type pauseHTTP struct {
	client      *http.Client
	maxAttempts int
	maxBackoff  time.Duration
	sleep       func(ctx context.Context, d time.Duration) error
}

func newPauseHTTP(c *http.Client, maxAttempts int, maxBackoff time.Duration, sleep func(context.Context, time.Duration) error) pauseHTTP {
	if c == nil {
		c = &http.Client{
			Timeout: 25 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	if maxBackoff <= 0 {
		maxBackoff = 10 * time.Second
	}
	if sleep == nil {
		sleep = sleepCtx
	}
	return pauseHTTP{client: c, maxAttempts: maxAttempts, maxBackoff: maxBackoff, sleep: sleep}
}

func (p pauseHTTP) do(ctx context.Context, platform, method, rawURL string, headers map[string]string, body []byte, wait func()) (int, []byte, error) {
	var lastStatus int
	var lastBody []byte
	var lastHdr http.Header
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
		if err != nil {
			return 0, nil, fmt.Errorf("%s: %s call: %s", platform, method, redact.SanitizeErr(err))
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if wait != nil {
			wait()
		}
		resp, err := p.client.Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("%s: %s call: %s", platform, method, redact.SanitizeErr(err))
		}
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		resp.Body.Close()
		if readErr != nil {
			return 0, nil, fmt.Errorf("%s: %s call: read body: %s", platform, method, redact.SanitizeErr(readErr))
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastStatus, lastBody, lastHdr = resp.StatusCode, respBody, resp.Header.Clone()
			if attempt == p.maxAttempts {
				break
			}
			wait := time.Duration(attempt*attempt) * 500 * time.Millisecond
			if ra := parseRetryAfter(resp.Header); ra > wait {
				wait = ra
			}
			if wait > p.maxBackoff {
				wait = p.maxBackoff
			}
			if err := p.sleep(ctx, wait); err != nil {
				return 0, nil, fmt.Errorf("%s: %s call: %s", platform, method, redact.SanitizeErr(err))
			}
			continue
		}
		return resp.StatusCode, respBody, nil
	}
	return 0, nil, fmt.Errorf("%s: %s call: gave up after %d attempts: %s",
		platform, method, p.maxAttempts, formatHTTPStatusError(lastStatus, lastBody, lastHdr, platform))
}

// ---------------------------------------------------------------------------------------------
// Ringba

// RingbaPublisher is one publisher as GET /v2/{accountId}/Publishers/{id} returns it. Enabled is
// nil when the response has no "enabled" key.
type RingbaPublisher struct {
	ID      string
	Name    string
	Enabled *bool
}

// ringbaEnabledUpdate is the only PATCH body Calyx sends. No omitempty: false must be sent.
type ringbaEnabledUpdate struct {
	Enabled bool `json:"enabled"`
}

// RingbaPublisherEnabledBody returns the exact PATCH body. Exported for tests.
func RingbaPublisherEnabledBody(enabled bool) ([]byte, error) {
	return json.Marshal(ringbaEnabledUpdate{Enabled: enabled})
}

// RingbaPublisherClientOptions configures a RingbaPublisherClient. Zero values take defaults.
type RingbaPublisherClientOptions struct {
	BaseURL     string // default RINGBA_BASE_URL or https://api.ringba.com/v2
	HTTPClient  *http.Client
	MaxAttempts int
	MaxBackoff  time.Duration
	Sleep       func(ctx context.Context, d time.Duration) error
	// Limiters replaces the shared per-account limiters (tests only).
	Limiters *RingbaAccountLimiters
}

// RingbaPublisherClient reads and toggles a Ringba publisher's "enabled". It shares the
// per-account limiter with the Ringba sync.
type RingbaPublisherClient struct {
	http     pauseHTTP
	base     string
	limiters *RingbaAccountLimiters
}

func NewRingbaPublisherClient() *RingbaPublisherClient {
	return NewRingbaPublisherClientWith(RingbaPublisherClientOptions{})
}

func NewRingbaPublisherClientWith(o RingbaPublisherClientOptions) *RingbaPublisherClient {
	c := &RingbaPublisherClient{
		http:     newPauseHTTP(o.HTTPClient, o.MaxAttempts, o.MaxBackoff, o.Sleep),
		base:     strings.TrimRight(o.BaseURL, "/"),
		limiters: ringbaLimiters,
	}
	if o.Limiters != nil {
		c.limiters = o.Limiters
	}
	if c.base == "" {
		c.base = ringbaAPIBase()
	}
	return c
}

// RingbaPublisherPath is the path used in audit rows: the account id is never stored.
func RingbaPublisherPath(publisherID string) string {
	return "/v2/{accountId}/Publishers/" + publisherID
}

func (c *RingbaPublisherClient) prepare(creds map[string]string, publisherID string) (token, accountID, rawURL string, err error) {
	token, accountID, err = ringbaCreds(creds)
	if err != nil {
		return "", "", "", err
	}
	accountID = strings.TrimSpace(accountID)
	if !ValidPlatformID(accountID) {
		return "", "", "", fmt.Errorf("ringba: invalid account id")
	}
	publisherID = strings.TrimSpace(publisherID)
	if !ValidPlatformID(publisherID) {
		return "", "", "", fmt.Errorf("ringba: %w", ErrInvalidPlatformID)
	}
	rawURL = c.base + "/" + url.PathEscape(accountID) + "/Publishers/" + url.PathEscape(publisherID)
	return strings.TrimSpace(token), accountID, rawURL, nil
}

// GetPublisher reads one publisher. 404 → ErrPublisherNotFound.
func (c *RingbaPublisherClient) GetPublisher(ctx context.Context, creds map[string]string, publisherID string) (RingbaPublisher, error) {
	token, accountID, rawURL, err := c.prepare(creds, publisherID)
	if err != nil {
		return RingbaPublisher{}, err
	}
	status, body, err := c.http.do(ctx, "ringba", http.MethodGet, rawURL,
		map[string]string{"Authorization": "Token " + token}, nil,
		func() { c.limiters.ForAccount(accountID).Wait() })
	if err != nil {
		return RingbaPublisher{}, err
	}
	if status == http.StatusNotFound {
		return RingbaPublisher{}, fmt.Errorf("ringba: %w (HTTP 404)", ErrPublisherNotFound)
	}
	if err := checkHTTPStatus(status, body, nil, "ringba"); err != nil {
		return RingbaPublisher{}, err
	}
	return ParseRingbaPublisher(body)
}

// SetPublisherEnabled sends PATCH …/Publishers/{id} {"enabled":<bool>}. 404 → ErrPublisherNotFound.
func (c *RingbaPublisherClient) SetPublisherEnabled(ctx context.Context, creds map[string]string, publisherID string, enabled bool) error {
	token, accountID, rawURL, err := c.prepare(creds, publisherID)
	if err != nil {
		return err
	}
	body, err := RingbaPublisherEnabledBody(enabled)
	if err != nil {
		return err
	}
	status, respBody, err := c.http.do(ctx, "ringba", http.MethodPatch, rawURL,
		map[string]string{"Authorization": "Token " + token}, body,
		func() { c.limiters.ForAccount(accountID).Wait() })
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("ringba: %w (HTTP 404)", ErrPublisherNotFound)
	}
	return checkHTTPStatus(status, respBody, nil, "ringba")
}

// ParseRingbaPublisher maps a single-publisher response: a bare object or one wrapped in
// "publisher" / "affiliate" (the docs' example is not explicit about the wrapper).
func ParseRingbaPublisher(body []byte) (RingbaPublisher, error) {
	raw := unwrapObject(body, "publisher", "affiliate")
	var row struct {
		ID      json.RawMessage `json:"id"`
		Name    *string         `json:"name"`
		Enabled *bool           `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return RingbaPublisher{}, fmt.Errorf("ringba: parse publisher: %s", redact.SanitizeErr(err))
	}
	out := RingbaPublisher{ID: rawScalar(row.ID), Enabled: row.Enabled}
	if row.Name != nil {
		out.Name = *row.Name
	}
	return out, nil
}

// unwrapObject returns the first wrapper key that holds a JSON object, else the body itself.
func unwrapObject(body []byte, keys ...string) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err == nil {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				t := bytes.TrimSpace(v)
				if len(t) > 0 && t[0] == '{' {
					return v
				}
			}
		}
	}
	return json.RawMessage(body)
}

// ---------------------------------------------------------------------------------------------
// Retreaver

// RetreaverAffiliate is one affiliate. Paused is nil when the response has no "paused" key
// (the documented response has none — read-back then falls back to the PUT's own response).
type RetreaverAffiliate struct {
	ID     string
	AFID   string
	Name   string
	Paused *bool
}

type retreaverPausedUpdate struct {
	Affiliate retreaverPausedFields `json:"affiliate"`
}

type retreaverPausedFields struct {
	Paused bool `json:"paused"`
}

// RetreaverAffiliatePausedBody returns the exact PUT body. Exported for tests.
func RetreaverAffiliatePausedBody(paused bool) ([]byte, error) {
	return json.Marshal(retreaverPausedUpdate{Affiliate: retreaverPausedFields{Paused: paused}})
}

// RetreaverAffiliateClientOptions configures a RetreaverAffiliateClient. Zero values take defaults.
type RetreaverAffiliateClientOptions struct {
	Origin      string // default RETREAVER_API_ORIGIN or https://api.retreaver.com
	HTTPClient  *http.Client
	MaxAttempts int
	MaxBackoff  time.Duration
	Sleep       func(ctx context.Context, d time.Duration) error
	// Limiters replaces the shared per-company limiters (tests only).
	Limiters *RetreaverAccountLimiters
	// MaxListPages bounds the afid fallback walk (default 50 pages of 100).
	MaxListPages int
}

// RetreaverAffiliateClient reads and pauses/unpauses a Retreaver affiliate. It shares the
// per-company limiter (2/s, 60/min) with the Retreaver sync and write-back.
type RetreaverAffiliateClient struct {
	http         pauseHTTP
	origin       string
	limiters     *RetreaverAccountLimiters
	maxListPages int
}

func NewRetreaverAffiliateClient() *RetreaverAffiliateClient {
	return NewRetreaverAffiliateClientWith(RetreaverAffiliateClientOptions{})
}

func NewRetreaverAffiliateClientWith(o RetreaverAffiliateClientOptions) *RetreaverAffiliateClient {
	c := &RetreaverAffiliateClient{
		http:         newPauseHTTP(o.HTTPClient, o.MaxAttempts, o.MaxBackoff, o.Sleep),
		origin:       strings.TrimRight(o.Origin, "/"),
		limiters:     retreaverLimiters,
		maxListPages: o.MaxListPages,
	}
	if o.Limiters != nil {
		c.limiters = o.Limiters
	}
	if c.origin == "" {
		c.origin = retreaverAPIOrigin()
	}
	if c.maxListPages < 1 {
		c.maxListPages = 50
	}
	return c
}

// RetreaverAffiliatePath is the path used in audit rows (never the query string).
func RetreaverAffiliatePath(id string) string { return "/api/v1/affiliates/" + id + ".json" }

// RetreaverAffiliateAFIDPath is the afid lookup path used in audit rows.
func RetreaverAffiliateAFIDPath(afid string) string {
	return "/api/v1/affiliates/afid/" + afid + ".json"
}

func (c *RetreaverAffiliateClient) url(path string, extra url.Values, apiKey, companyID string) string {
	q := url.Values{}
	for k, v := range extra {
		q[k] = v
	}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	return c.origin + path + "?" + q.Encode()
}

func (c *RetreaverAffiliateClient) get(ctx context.Context, creds map[string]string, path string, extra url.Values) (int, []byte, error) {
	apiKey, companyID, err := retreaverCreds(creds)
	if err != nil {
		return 0, nil, err
	}
	return c.http.do(ctx, "retreaver", http.MethodGet, c.url(path, extra, apiKey, companyID), nil, nil,
		func() { c.limiters.ForAccount(companyID).Wait() })
}

// GetAffiliate reads one affiliate by system id. 404 → ErrPublisherNotFound.
func (c *RetreaverAffiliateClient) GetAffiliate(ctx context.Context, creds map[string]string, id string) (RetreaverAffiliate, error) {
	id = strings.TrimSpace(id)
	if !ValidPlatformID(id) {
		return RetreaverAffiliate{}, fmt.Errorf("retreaver: %w", ErrInvalidPlatformID)
	}
	status, body, err := c.get(ctx, creds, "/api/v1/affiliates/"+url.PathEscape(id)+".json", nil)
	if err != nil {
		return RetreaverAffiliate{}, err
	}
	if status == http.StatusNotFound {
		return RetreaverAffiliate{}, fmt.Errorf("retreaver: %w (HTTP 404)", ErrPublisherNotFound)
	}
	if err := checkHTTPStatus(status, body, nil, "retreaver"); err != nil {
		return RetreaverAffiliate{}, err
	}
	return ParseRetreaverAffiliate(body)
}

// ResolveAffiliate finds the affiliate for an afid: GET /api/v1/affiliates/afid/{afid}.json;
// on 404, walk GET /api/v1/affiliates.json pages for an exact afid match, then read it by its
// system id. The returned affiliate always carries the system id used for the PUT. The second
// return value lists the paths requested (for the audit row; no query strings).
func (c *RetreaverAffiliateClient) ResolveAffiliate(ctx context.Context, creds map[string]string, afid string) (RetreaverAffiliate, []string, error) {
	afid = strings.TrimSpace(afid)
	if !ValidPlatformID(afid) {
		return RetreaverAffiliate{}, nil, fmt.Errorf("retreaver: %w", ErrInvalidPlatformID)
	}
	steps := []string{"GET " + RetreaverAffiliateAFIDPath(afid)}
	status, body, err := c.get(ctx, creds, "/api/v1/affiliates/afid/"+url.PathEscape(afid)+".json", nil)
	if err != nil {
		return RetreaverAffiliate{}, steps, err
	}
	if status != http.StatusNotFound {
		if err := checkHTTPStatus(status, body, nil, "retreaver"); err != nil {
			return RetreaverAffiliate{}, steps, err
		}
		aff, err := ParseRetreaverAffiliate(body)
		if err != nil {
			return RetreaverAffiliate{}, steps, err
		}
		if !ValidPlatformID(aff.ID) {
			return RetreaverAffiliate{}, steps, fmt.Errorf("retreaver: affiliate response has no usable id")
		}
		return aff, steps, nil
	}

	// Fallback: list pages, exact afid match.
	const perPage = 100
	for page := 1; page <= c.maxListPages; page++ {
		steps = append(steps, fmt.Sprintf("GET /api/v1/affiliates.json page=%d", page))
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", strconv.Itoa(perPage))
		status, body, err := c.get(ctx, creds, "/api/v1/affiliates.json", q)
		if err != nil {
			return RetreaverAffiliate{}, steps, err
		}
		if err := checkHTTPStatus(status, body, nil, "retreaver"); err != nil {
			return RetreaverAffiliate{}, steps, err
		}
		list, err := ParseRetreaverAffiliateList(body)
		if err != nil {
			return RetreaverAffiliate{}, steps, err
		}
		for _, a := range list {
			if a.AFID == afid {
				if !ValidPlatformID(a.ID) {
					return RetreaverAffiliate{}, steps, fmt.Errorf("retreaver: affiliate list entry has no usable id")
				}
				steps = append(steps, "GET "+RetreaverAffiliatePath(a.ID))
				full, err := c.GetAffiliate(ctx, creds, a.ID)
				return full, steps, err
			}
		}
		if len(list) < perPage {
			break
		}
	}
	return RetreaverAffiliate{}, steps, fmt.Errorf("retreaver: %w (afid not in affiliate list)", ErrPublisherNotFound)
}

// SetPaused sends PUT /api/v1/affiliates/{id}.json {"affiliate":{"paused":<bool>}}. The parsed
// response is returned (its Paused is nil when the response has no "paused" key).
func (c *RetreaverAffiliateClient) SetPaused(ctx context.Context, creds map[string]string, id string, paused bool) (RetreaverAffiliate, error) {
	apiKey, companyID, err := retreaverCreds(creds)
	if err != nil {
		return RetreaverAffiliate{}, err
	}
	id = strings.TrimSpace(id)
	if !ValidPlatformID(id) {
		return RetreaverAffiliate{}, fmt.Errorf("retreaver: %w", ErrInvalidPlatformID)
	}
	body, err := RetreaverAffiliatePausedBody(paused)
	if err != nil {
		return RetreaverAffiliate{}, err
	}
	status, respBody, err := c.http.do(ctx, "retreaver", http.MethodPut,
		c.url("/api/v1/affiliates/"+url.PathEscape(id)+".json", nil, apiKey, companyID), nil, body,
		func() { c.limiters.ForAccount(companyID).Wait() })
	if err != nil {
		return RetreaverAffiliate{}, err
	}
	if status == http.StatusNotFound {
		return RetreaverAffiliate{}, fmt.Errorf("retreaver: %w (HTTP 404)", ErrPublisherNotFound)
	}
	if err := checkHTTPStatus(status, respBody, nil, "retreaver"); err != nil {
		return RetreaverAffiliate{}, err
	}
	if len(bytes.TrimSpace(respBody)) == 0 {
		return RetreaverAffiliate{ID: id}, nil
	}
	aff, perr := ParseRetreaverAffiliate(respBody)
	if perr != nil {
		// The write was accepted (2xx); an unparsable body does not undo that.
		return RetreaverAffiliate{ID: id}, nil
	}
	return aff, nil
}

type retreaverAffiliateRow struct {
	ID          json.RawMessage `json:"id"`
	AFID        json.RawMessage `json:"afid"`
	ClientAFID  json.RawMessage `json:"client_afid"`
	FirstName   *string         `json:"first_name"`
	LastName    *string         `json:"last_name"`
	CompanyName *string         `json:"company_name"`
	Paused      *bool           `json:"paused"`
}

func (r retreaverAffiliateRow) toAffiliate() RetreaverAffiliate {
	out := RetreaverAffiliate{ID: rawScalar(r.ID), AFID: rawScalar(r.AFID), Paused: r.Paused}
	if out.AFID == "" {
		out.AFID = rawScalar(r.ClientAFID)
	}
	if r.CompanyName != nil && strings.TrimSpace(*r.CompanyName) != "" {
		out.Name = strings.TrimSpace(*r.CompanyName)
	} else {
		var parts []string
		if r.FirstName != nil && strings.TrimSpace(*r.FirstName) != "" {
			parts = append(parts, strings.TrimSpace(*r.FirstName))
		}
		if r.LastName != nil && strings.TrimSpace(*r.LastName) != "" {
			parts = append(parts, strings.TrimSpace(*r.LastName))
		}
		out.Name = strings.Join(parts, " ")
	}
	return out
}

// ParseRetreaverAffiliate maps {"affiliate":{...}} or a bare object.
func ParseRetreaverAffiliate(body []byte) (RetreaverAffiliate, error) {
	raw := unwrapObject(body, "affiliate")
	var row retreaverAffiliateRow
	if err := json.Unmarshal(raw, &row); err != nil {
		return RetreaverAffiliate{}, fmt.Errorf("retreaver: parse affiliate: %s", redact.SanitizeErr(err))
	}
	return row.toAffiliate(), nil
}

// ParseRetreaverAffiliateList maps a list response: a JSON array of {"affiliate":{...}} or bare
// objects, or an object holding such an array under "affiliates".
func ParseRetreaverAffiliateList(body []byte) ([]RetreaverAffiliate, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		var wrapped struct {
			Affiliates []json.RawMessage `json:"affiliates"`
		}
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			return nil, fmt.Errorf("retreaver: parse affiliate list: %s", redact.SanitizeErr(err))
		}
		items = wrapped.Affiliates
	}
	out := make([]RetreaverAffiliate, 0, len(items))
	for _, it := range items {
		a, err := ParseRetreaverAffiliate(it)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
