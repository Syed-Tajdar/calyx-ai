package redact

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	urlInErrorRE = regexp.MustCompile(`https?://[^\s"']+`)
	paramRE      = regexp.MustCompile(`(?i)(api_key|company_id|token|secret|password|access_key)=([^&\s"']+)`)
	authHeaderRE = regexp.MustCompile(`(?i)Authorization:\s*(?:Bearer\s+\S+|Basic\s+\S+|Token\s+\S+|\S+)`)
	bearerTokRE  = regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9\-._~+/]+=*`)
	tokenTokRE   = regexp.MustCompile(`(?i)(Token\s+)[A-Za-z0-9\-._~+/]+=*`)
)

var sensitiveQueryKeys = map[string]struct{}{
	"api_key":    {},
	"company_id": {},
	"token":      {},
	"secret":     {},
	"password":   {},
	"access_key": {},
}

// RedactMessage strips or redacts credentials from free-form text (errors, URLs).
func RedactMessage(msg string) string {
	if msg == "" {
		return msg
	}
	out := urlInErrorRE.ReplaceAllStringFunc(msg, redactURLString)
	out = paramRE.ReplaceAllString(out, `${1}=[REDACTED]`)
	out = authHeaderRE.ReplaceAllStringFunc(out, redactAuthHeader)
	out = bearerTokRE.ReplaceAllString(out, `${1}[REDACTED]`)
	out = tokenTokRE.ReplaceAllString(out, `${1}[REDACTED]`)
	return out
}

func redactAuthHeader(m string) string {
	lower := strings.ToLower(m)
	switch {
	case strings.HasPrefix(lower, "authorization: bearer"):
		return "Authorization: Bearer [REDACTED]"
	case strings.HasPrefix(lower, "authorization: basic"):
		return "Authorization: Basic [REDACTED]"
	case strings.HasPrefix(lower, "authorization: token"):
		// Ringba API Access Tokens: Authorization: Token {apiToken}
		return "Authorization: Token [REDACTED]"
	default:
		// Bare Authorization value (legacy / misconfigured).
		return "Authorization: [REDACTED]"
	}
}

// SanitizeErr returns a redacted error string safe for logs and DB persistence.
func SanitizeErr(err error) string {
	if err == nil {
		return ""
	}
	return RedactMessage(err.Error())
}

// RedactURL redacts sensitive query parameters from a URL string.
func RedactURL(raw string) string {
	return redactURLString(raw)
}

func redactURLString(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return paramRE.ReplaceAllString(raw, `${1}=[REDACTED]`)
	}
	q := u.Query()
	for key := range q {
		if _, sensitive := sensitiveQueryKeys[strings.ToLower(key)]; sensitive {
			q.Set(key, "[REDACTED]")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ClassifySyncError returns a user-facing message with category prefix; credentials are redacted.
func ClassifySyncError(err error) string {
	if err == nil {
		return ""
	}
	msg := SanitizeErr(err)
	lower := strings.ToLower(msg)
	switch {
	case isNetworkError(lower):
		return "network error (transient, will retry): " + msg
	case isAuthError(lower):
		return "authentication error: " + msg
	case strings.Contains(lower, "skipped: missing external id") || strings.Contains(lower, "parse response"):
		return "mapping error: " + msg
	default:
		return msg
	}
}

func isNetworkError(lower string) bool {
	needles := []string{
		"dial tcp",
		"no such host",
		"server misbehaving",
		"i/o timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"tls handshake timeout",
		"context deadline exceeded",
		"eof",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

func isAuthError(lower string) bool {
	if strings.Contains(lower, "http 401") || strings.Contains(lower, "http 403") {
		return true
	}
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") {
		return true
	}
	return false
}

// IsRetryableNetwork reports whether an HTTP transport error should be retried.
func IsRetryableNetwork(err error) bool {
	if err == nil {
		return false
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isNetworkError(strings.ToLower(urlErr.Error()))
	}
	return isNetworkError(strings.ToLower(err.Error()))
}
