package consumer

import (
	"errors"
	"net"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

// MaxUnknownAttempts before treating a healthy-CH unknown insert error as poison.
// Justified: 5 covers brief CH blips where Ping still succeeds; avoids infinite stall (F013).
const MaxUnknownAttempts = 5

// UnknownAttemptMinInterval is the minimum gap between attempt increments for the same
// message. 5 attempts × 45s ≈ 3 minutes from first failure to poison — long enough that a
// brief ClickHouse blip does not dead-letter valid rows.
const UnknownAttemptMinInterval = 45 * time.Second

// ClickHouse server codes treated as transient even when Ping succeeds.
// See https://github.com/ClickHouse/ClickHouse/blob/master/src/Common/ErrorCodes.cpp
const (
	chCodeTimeoutExceeded     int32 = 159
	chCodeTooSlow             int32 = 160
	chCodeSocketTimeout       int32 = 209
	chCodeNetworkError        int32 = 210
	chCodeMemoryLimitExceeded int32 = 241
	chCodeTooManyParts        int32 = 252
)

type attemptState struct {
	count  int
	lastAt time.Time
	first  time.Time
}

// isTransientClickHouseError reports server-side / network failures that must never
// count toward the poison budget (even when Ping still succeeds).
func isTransientClickHouseError(err error) bool {
	if err == nil {
		return false
	}
	var ex *ch.Exception
	if errors.As(err, &ex) {
		switch ex.Code {
		case chCodeTooManyParts, chCodeMemoryLimitExceeded,
			chCodeTimeoutExceeded, chCodeTooSlow,
			chCodeSocketTimeout, chCodeNetworkError:
			return true
		}
		name := strings.ToUpper(ex.Name)
		switch {
		case strings.Contains(name, "TOO_MANY_PARTS"),
			strings.Contains(name, "MEMORY_LIMIT"),
			strings.Contains(name, "TIMEOUT"),
			strings.Contains(name, "NETWORK"),
			strings.Contains(name, "SOCKET"):
			return true
		}
	}
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return true
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"too_many_parts",
		"memory_limit_exceeded",
		"timeout",
		"i/o timeout",
		"connection refused",
		"connection reset",
		"broken pipe",
		"network is unreachable",
		"no such host",
		"eof",
		"read: connection",
		"write: connection",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

// UnknownAttemptMinSpan is the minimum wall time from first counted failure to poison.
func UnknownAttemptMinSpan() time.Duration {
	return UnknownAttemptMinInterval * time.Duration(MaxUnknownAttempts-1)
}
