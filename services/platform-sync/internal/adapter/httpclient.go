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
	return &HTTPClient{
		client:   &http.Client{Timeout: 25 * time.Second},
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
			return &httpResult{status: resp.StatusCode, body: body}, nil
		})
		if err != nil {
			lastErr = err
			if _, ok := err.(*rateLimitedError); ok || strings.Contains(err.Error(), "429") || redact.IsRetryableNetwork(err) {
				continue
			}
			return nil, nil, sanitizeHTTPError(err)
		}
		hr := result.(*httpResult)
		return &http.Response{StatusCode: hr.status}, hr.body, nil
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
	body   []byte
}

// StatusError indicates a non-2xx response.
type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	snippet := e.Body
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, snippet)
}

func checkStatus(status int, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	return &StatusError{Status: status, Body: string(body)}
}
