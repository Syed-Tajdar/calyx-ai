package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
)

// Retreaver write-back transport (Stage 4b).
//
// Docs (Phase 0, https://retreaver.github.io/core-api-docs/#update-a-call):
//
//	PUT https://api.retreaver.com/api/v5/calls/{uuid}.json?api_key=…&company_id=…
//	Content-Type: application/json
//	{"call":{"revenue":20.0, …}}
//
// "Setting a value of 0.01 or more marks the Call receivable. Passing 0 clears the revenue."
// "If an edit newly marks the Call as converted, its conversion pixels will fire."
//
// Calyx only ever sends {"call":{"revenue":<amount>}}. payout and session_notes are never
// sent: the request body type below has no field for them, so they cannot be.
//
// The key can only travel in the query string (header auth returns 401, R1 B2(1)), so every
// error this file returns is passed through the redaction helper first.

const defaultRetreaverAPIOrigin = "https://api.retreaver.com"

// retreaverAPIOrigin is the scheme+host for single-call requests. RETREAVER_API_ORIGIN
// overrides it for tests; production leaves it unset.
func retreaverAPIOrigin() string {
	if v := strings.TrimSpace(os.Getenv("RETREAVER_API_ORIGIN")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultRetreaverAPIOrigin
}

var retreaverUUIDRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsRetreaverCallUUID reports whether id is a Retreaver call UUID. Anything else is refused
// before a request is built, so a call id can never reach the URL path unvalidated.
func IsRetreaverCallUUID(id string) bool {
	return retreaverUUIDRE.MatchString(strings.TrimSpace(id))
}

// retreaverRevenueUpdate is the only PUT body Calyx sends. No omitempty: revenue 0 must be
// sent as 0, because that is the documented void.
type retreaverRevenueUpdate struct {
	Call retreaverRevenueFields `json:"call"`
}

type retreaverRevenueFields struct {
	Revenue float64 `json:"revenue"`
}

// RetreaverRevenueBody returns the exact JSON body for a revenue write. Exported for tests.
func RetreaverRevenueBody(revenue float64) ([]byte, error) {
	return json.Marshal(retreaverRevenueUpdate{Call: retreaverRevenueFields{Revenue: revenue}})
}

// RetreaverCallReadBack is one call as GET /api/v2/calls/{uuid}.json returns it: the mapped
// call plus the raw read-back fields. Pointers are nil when the API returned null or no key.
type RetreaverCallReadBack struct {
	Call                    models.RemoteCall
	CompanyID               string
	Converted               *bool
	Revenue                 *float64
	Payout                  *float64
	Receivable              *bool
	Payable                 *bool
	ConversionsDeterminedAt *string
	FiredPixelsCount        *int
	PostbackValue           *string
	UpdatedAt               *string
}

// RetreaverCallClientOptions configures a RetreaverCallClient. Zero values take defaults.
type RetreaverCallClientOptions struct {
	Origin      string
	HTTPClient  *http.Client
	MaxAttempts int
	MaxBackoff  time.Duration
	// Sleep replaces the backoff wait (tests).
	Sleep func(ctx context.Context, d time.Duration) error
	// Limiters replaces the shared per-company limiters (tests only; production must share
	// the sync limiters, which is the default).
	Limiters *RetreaverAccountLimiters
}

// RetreaverCallClient does single-call reads and revenue writes. It shares the per-company
// limiter with Retreaver sync (2/s and 60/min by default), so write-back and sync together
// cannot exceed the R1 budget.
type RetreaverCallClient struct {
	client      *http.Client
	origin      string
	limiters    *RetreaverAccountLimiters
	maxAttempts int
	maxBackoff  time.Duration
	sleep       func(ctx context.Context, d time.Duration) error
}

func NewRetreaverCallClient() *RetreaverCallClient {
	return NewRetreaverCallClientWith(RetreaverCallClientOptions{})
}

func NewRetreaverCallClientWith(o RetreaverCallClientOptions) *RetreaverCallClient {
	c := &RetreaverCallClient{
		client:      o.HTTPClient,
		origin:      strings.TrimRight(o.Origin, "/"),
		limiters:    retreaverLimiters,
		maxAttempts: o.MaxAttempts,
		maxBackoff:  o.MaxBackoff,
		sleep:       o.Sleep,
	}
	if o.Limiters != nil {
		c.limiters = o.Limiters
	}
	if c.client == nil {
		c.client = &http.Client{
			Timeout: 25 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if c.origin == "" {
		c.origin = retreaverAPIOrigin()
	}
	if c.maxAttempts < 1 {
		c.maxAttempts = 3
	}
	if c.maxBackoff <= 0 {
		c.maxBackoff = 10 * time.Second
	}
	if c.sleep == nil {
		c.sleep = sleepCtx
	}
	return c
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func retreaverCreds(creds map[string]string) (apiKey, companyID string, err error) {
	apiKey = strings.TrimSpace(creds["api_key"])
	companyID = strings.TrimSpace(creds["company_id"])
	if apiKey == "" || companyID == "" {
		return "", "", fmt.Errorf("retreaver: api_key and company_id required")
	}
	return apiKey, companyID, nil
}

func (c *RetreaverCallClient) callURL(version, uuid, apiKey, companyID string) string {
	q := url.Values{}
	q.Set("api_key", apiKey)
	q.Set("company_id", companyID)
	return c.origin + "/api/" + version + "/calls/" + url.PathEscape(uuid) + ".json?" + q.Encode()
}

// SetRevenue writes revenue on one call: PUT /api/v5/calls/{uuid}.json {"call":{"revenue":x}}.
// The value is absolute (set, not added). 0 is the documented way to clear revenue.
func (c *RetreaverCallClient) SetRevenue(ctx context.Context, creds map[string]string, uuid string, revenue float64) error {
	apiKey, companyID, err := retreaverCreds(creds)
	if err != nil {
		return err
	}
	uuid = strings.TrimSpace(uuid)
	if !IsRetreaverCallUUID(uuid) {
		return fmt.Errorf("retreaver: invalid call uuid")
	}
	body, err := RetreaverRevenueBody(revenue)
	if err != nil {
		return err
	}
	status, respBody, hdr, err := c.do(ctx, http.MethodPut, c.callURL("v5", uuid, apiKey, companyID), companyID, body)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("retreaver: call not found (HTTP 404)")
	}
	return checkHTTPStatus(status, respBody, hdr, "retreaver")
}

// GetCall reads one call: GET /api/v2/calls/{uuid}.json.
func (c *RetreaverCallClient) GetCall(ctx context.Context, creds map[string]string, uuid string) (RetreaverCallReadBack, error) {
	apiKey, companyID, err := retreaverCreds(creds)
	if err != nil {
		return RetreaverCallReadBack{}, err
	}
	uuid = strings.TrimSpace(uuid)
	if !IsRetreaverCallUUID(uuid) {
		return RetreaverCallReadBack{}, fmt.Errorf("retreaver: invalid call uuid")
	}
	status, body, hdr, err := c.do(ctx, http.MethodGet, c.callURL("v2", uuid, apiKey, companyID), companyID, nil)
	if err != nil {
		return RetreaverCallReadBack{}, err
	}
	if status == http.StatusNotFound {
		return RetreaverCallReadBack{}, fmt.Errorf("retreaver: call not found (HTTP 404)")
	}
	if err := checkHTTPStatus(status, body, hdr, "retreaver"); err != nil {
		return RetreaverCallReadBack{}, err
	}
	return ParseRetreaverCallReadBack(body)
}

// do sends one request with the shared per-company limiter. It retries only 429 and 5xx,
// with backoff (Retry-After honoured, capped). A 4xx is returned at once. A transport error
// is returned at once too: the write may have landed, and the read-back decides. Every error
// it returns is redacted.
func (c *RetreaverCallClient) do(ctx context.Context, method, rawURL, companyID string, body []byte) (int, []byte, http.Header, error) {
	var lastStatus int
	var lastBody []byte
	var lastHdr http.Header
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("retreaver: %s call: %s", method, redact.SanitizeErr(err))
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		c.limiters.ForAccount(companyID).Wait()
		resp, err := c.client.Do(req)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("retreaver: %s call: %s", method, redact.SanitizeErr(err))
		}
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		resp.Body.Close()
		if readErr != nil {
			return 0, nil, nil, fmt.Errorf("retreaver: %s call: read body: %s", method, redact.SanitizeErr(readErr))
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastStatus, lastBody, lastHdr = resp.StatusCode, respBody, resp.Header.Clone()
			if attempt == c.maxAttempts {
				break
			}
			wait := time.Duration(attempt*attempt) * 500 * time.Millisecond
			if ra := parseRetryAfter(resp.Header); ra > wait {
				wait = ra
			}
			if wait > c.maxBackoff {
				wait = c.maxBackoff
			}
			if err := c.sleep(ctx, wait); err != nil {
				return 0, nil, nil, fmt.Errorf("retreaver: %s call: %s", method, redact.SanitizeErr(err))
			}
			continue
		}
		return resp.StatusCode, respBody, resp.Header.Clone(), nil
	}
	return 0, nil, nil, fmt.Errorf("retreaver: %s call: gave up after %d attempts: %s",
		method, c.maxAttempts, formatHTTPStatusError(lastStatus, lastBody, lastHdr, "retreaver"))
}

// flexFloat accepts a JSON number or a numeric string.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

// ParseRetreaverCallReadBack maps a single-call response ({"call":{...}} or a bare object).
func ParseRetreaverCallReadBack(body []byte) (RetreaverCallReadBack, error) {
	raw := unwrapRetreaverCallPayload(json.RawMessage(body))
	var row struct {
		CompanyID               json.RawMessage `json:"company_id"`
		Converted               *bool           `json:"converted"`
		Revenue                 *flexFloat      `json:"revenue"`
		Payout                  *flexFloat      `json:"payout"`
		Receivable              *bool           `json:"receivable"`
		Payable                 *bool           `json:"payable"`
		ConversionsDeterminedAt *string         `json:"conversions_determined_at"`
		FiredPixelsCount        *flexFloat      `json:"fired_pixels_count"`
		PostbackValue           json.RawMessage `json:"postback_value"`
		UpdatedAt               *string         `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return RetreaverCallReadBack{}, fmt.Errorf("retreaver: parse response: %w", err)
	}
	out := RetreaverCallReadBack{
		Call:                    mapRetreaverCallOpts(raw, false),
		CompanyID:               rawScalar(row.CompanyID),
		Converted:               row.Converted,
		Receivable:              row.Receivable,
		Payable:                 row.Payable,
		ConversionsDeterminedAt: row.ConversionsDeterminedAt,
		UpdatedAt:               row.UpdatedAt,
	}
	// The sync mapper reads revenue/payout as JSON numbers only. A read-back must not misread a
	// numeric string as 0, so the mapped call takes the tolerant values parsed here.
	out.Call.Revenue, out.Call.Payout = 0, 0
	if row.Revenue != nil {
		v := float64(*row.Revenue)
		out.Revenue = &v
		out.Call.Revenue = v
	}
	if row.Payout != nil {
		v := float64(*row.Payout)
		out.Payout = &v
		out.Call.Payout = v
	}
	if row.FiredPixelsCount != nil {
		v := int(*row.FiredPixelsCount)
		out.FiredPixelsCount = &v
	}
	if s := rawScalar(row.PostbackValue); s != "" {
		red := redact.RedactMessage(s)
		out.PostbackValue = &red
	}
	return out, nil
}

// rawScalar renders a JSON string or number as text; null and absent are "".
func rawScalar(b json.RawMessage) string {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		return ""
	}
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		return str
	}
	return s
}
