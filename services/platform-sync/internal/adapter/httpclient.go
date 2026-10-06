package adapter

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/sony/gobreaker"
)

const maxBodyBytes = 4 << 20 // 4MB

// HTTPClient wraps http.Client with per-host circuit breaker, 429 retry, and network retry.
type HTTPClient struct {
	client   *http.Client
	breakers map[string]*gobreaker.CircuitBreaker
}

func NewHTTPClient() *HTTPClient {
	// Do NOT auto-follow redirects. Go's default Client follows 301/302 (up to 10)
	// and may change POST→GET; cross-host redirects also drop Authorization.
	// For API adapters we must surface status + Location ourselves (Ringba 301 names
	// the new reporting endpoint in Location and/or the response body).
	return &HTTPClient{
		client: &http.Client{
			Timeout: 25 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		breakers: map[string]*gobreaker.CircuitBreaker{},
	}
}

func (c *HTTPClient) breaker(host string) *gobreaker.CircuitBreaker {
	if b, ok := c.breakers[host]; ok {
		return b
	}
	b := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:    host,
		Timeout: 60 * time.Second,
	})
	c.breakers[host] = b
	return b
}

type rateLimitedError struct {
	retryAfter time.Duration
}

func (e *rateLimitedError) Error() string {
	if e.retryAfter > 0 {
		return fmt.Sprintf("rate limited (429) retry_after=%s", e.retryAfter)
	}
	return "rate limited (429)"
}

func parseRetryAfter(h http.Header) time.Duration {
	ra := strings.TrimSpace(h.Get("Retry-After"))
	if ra == "" {
		return 0
	}
	if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(ra); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// Do executes an HTTP request with circuit breaker, 429 retry (honours Retry-After), and network retry.
func (c *HTTPClient) Do(req *http.Request) (*http.Response, []byte, error) {
	host := req.URL.Host
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			if rl, ok := lastErr.(*rateLimitedError); ok && rl.retryAfter > backoff {
				backoff = rl.retryAfter
			}
			time.Sleep(backoff)
		}
		result, err := c.breaker(host).Execute(func() (interface{}, error) {
			resp, err := c.client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
			if readErr != nil {
				return nil, readErr
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, &rateLimitedError{retryAfter: parseRetryAfter(resp.Header)}
			}
			if resp.StatusCode >= 500 {
				return nil, fmt.Errorf("upstream %d", resp.StatusCode)
			}
			return &httpResult{status: resp.StatusCode, header: resp.Header.Clone(), body: body}, nil
		})
		if err != nil {
			lastErr = err
			if _, ok := err.(*rateLimitedError); ok || strings.Contains(err.Error(), "429") || redact.IsRetryableNetwork(err) {
				continue
			}
			return nil, nil, sanitizeHTTPError(err)
		}
		hr := result.(*httpResult)
		return &http.Response{StatusCode: hr.status, Header: hr.header}, hr.body, nil
	}
	if lastErr != nil {
		return nil, nil, sanitizeHTTPError(lastErr)
	}
	return nil, nil, fmt.Errorf("request failed after retries")
}

func sanitizeHTTPError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", redact.SanitizeErr(err))
}

type httpResult struct {
	status int
	header http.Header
	body   []byte
}

// StatusError indicates a non-2xx response (full body; no 200-char truncation).
type StatusError struct {
	Status int
	Body   string
	Header http.Header
}

func (e *StatusError) Error() string {
	return formatHTTPStatusError(e.Status, []byte(e.Body), e.Header, "")
}

func checkStatus(status int, body []byte, hdr http.Header) error {
	if status >= 200 && status < 300 {
		return nil
	}
	return &StatusError{Status: status, Body: string(body), Header: hdr}
}

// safeHeaderKeys are response headers safe to log (values still URL-redacted).
var safeHeaderKeys = []string{
	"Location",
	"Content-Type",
	"Retry-After",
	"WWW-Authenticate",
}

func formatSafeResponseHeaders(hdr http.Header) string {
	if hdr == nil {
		return ""
	}
	parts := make([]string, 0, len(safeHeaderKeys))
	for _, key := range safeHeaderKeys {
		vals := hdr.Values(key)
		if len(vals) == 0 {
			continue
		}
		joined := strings.Join(vals, ", ")
		if strings.EqualFold(key, "Location") {
			joined = redact.RedactURL(joined)
		}
		if strings.EqualFold(key, "WWW-Authenticate") {
			joined = redact.RedactMessage(joined)
		}
		parts = append(parts, fmt.Sprintf("%s=%q", key, joined))
	}
	return strings.Join(parts, " ")
}

func formatHTTPStatusError(status int, body []byte, hdr http.Header, platform string) string {
	prefix := "HTTP"
	if platform != "" {
		prefix = platform + " HTTP"
	}
	headers := formatSafeResponseHeaders(hdr)
	// The body is attacker-/vendor-controlled text that ends up in the integration's
	// last_sync_error column. Retreaver authenticates with `?api_key=` in the query string
	// (header auth returns 401 — see retreaver_redaction_test.go), so any upstream error body
	// that echoes the request URL would otherwise persist the credential. Redact it.
	bodyStr := redact.RedactMessage(string(body))
	if headers != "" {
		return fmt.Sprintf("%s %d: headers={%s} body=%s", prefix, status, headers, bodyStr)
	}
	return fmt.Sprintf("%s %d: %s", prefix, status, bodyStr)
}
