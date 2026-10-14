package writeback

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Writer is one platform's write-back path (Stage 4b).
//
// It replaces the Ringba-only guard (`ErrNotRingba`) the same way R1 replaced the sync guard
// with RangeFetcher: the Service keeps every shared rule — kill switch, per-tenant flag,
// Live-only, credentials, idempotency lock, audit on every attempt — and hands only the
// platform-specific steps to the Writer registered for the integration's platform.
type Writer interface {
	// Platform is the integrations.platform value this writer serves, lower case.
	Platform() string
	// CheckCredentials runs before the lock. A non-nil error is audited as a rejection.
	CheckCredentials(creds map[string]string) error
	// LookupCall reads the call before anything is written. A missing call returns an
	// error whose text contains "not found".
	LookupCall(ctx context.Context, creds map[string]string, callID string) (CallState, error)
	// Convert writes the conversion amount. A *preWriteError means nothing was sent.
	Convert(ctx context.Context, creds map[string]string, callID string, amount float64) (WriteInfo, error)
	// Void clears the conversion. before is the state LookupCall returned.
	Void(ctx context.Context, creds map[string]string, callID string, before CallState) (WriteInfo, error)
	// ReadBack re-reads the call after a write.
	ReadBack(ctx context.Context, creds map[string]string, callID string) (CallState, error)
	// IsConverted is the precondition check: convert refuses a converted call, void refuses
	// a call that is not converted.
	IsConverted(c CallState) bool
	// ConvertSettled / VoidSettled decide when the read-back confirms the write.
	ConvertSettled(after CallState, amount float64) bool
	VoidSettled(after CallState) bool
	// SettleWindow is this platform's read-back window. Zero uses the Service default.
	SettleWindow() time.Duration
	// Revenue is the call's revenue as this platform's read-back reports it (null → 0). The
	// Service uses it for the shared unsettled rule: a read-back that never settles is
	// pending, unless it shows a contradicting value — a revenue that is neither what was
	// written nor what the call had before — which is a failure. Same rule for every platform.
	Revenue(c CallState) float64
}

// RequestValidator is an optional Writer hook. It runs after CheckCredentials and before the
// lock; an error is audited as a rejection and nothing is sent to the platform.
type RequestValidator interface {
	ValidateRequest(action, callID string, amount float64) error
}

// CallState is what LookupCall / ReadBack return: the mapped call plus, for platforms that
// expose them, the raw read-back fields stored in the audit row.
type CallState struct {
	Call     models.RemoteCall
	ReadBack *ReadBackFields
}

// ReadBackFields are the platform read-back values recorded in the audit row. Never secrets.
// Pointer fields are nil when the platform returned null or omitted the key.
type ReadBackFields struct {
	Converted               *bool    `json:"converted"`
	Revenue                 *float64 `json:"revenue"`
	Payout                  *float64 `json:"payout"`
	Receivable              *bool    `json:"receivable"`
	Payable                 *bool    `json:"payable"`
	ConversionsDeterminedAt *string  `json:"conversions_determined_at"`
	FiredPixelsCount        *int     `json:"fired_pixels_count"`
	PostbackValue           *string  `json:"postback_value"`
	UpdatedAt               *string  `json:"updated_at"`
}

// WriteInfo is what a write returned that the audit row and Result keep.
type WriteInfo struct {
	Ref         string
	WebhookName string
}

// preWriteError marks a failure before anything was sent to the platform. The Service
// returns it bare, without an audit row and without ErrFireRejected — exactly what the
// Ringba webhook-listing path did before the registry.
type preWriteError struct{ err error }

func (e *preWriteError) Error() string { return e.err.Error() }
func (e *preWriteError) Unwrap() error { return e.err }

// PreWrite wraps err as a failure that happened before any write.
func PreWrite(err error) error {
	if err == nil {
		return nil
	}
	return &preWriteError{err: err}
}

func asPreWrite(err error) (error, bool) {
	var pw *preWriteError
	if errors.As(err, &pw) {
		return pw.err, true
	}
	return nil, false
}

// Register adds or replaces the writer for w.Platform().
func (s *Service) Register(w Writer) {
	if w == nil {
		return
	}
	if s.writers == nil {
		s.writers = map[string]Writer{}
	}
	s.writers[strings.ToLower(strings.TrimSpace(w.Platform()))] = w
}

func (s *Service) writerFor(platform string) (Writer, bool) {
	w, ok := s.writers[strings.ToLower(strings.TrimSpace(platform))]
	return w, ok
}

// Supports reports whether write-back is implemented for platform.
func (s *Service) Supports(platform string) bool {
	_, ok := s.writerFor(platform)
	return ok
}

// SupportedPlatforms lists the registered platforms, sorted.
func (s *Service) SupportedPlatforms() []string {
	out := make([]string, 0, len(s.writers))
	for p := range s.writers {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// platformError keeps errors.Is(err, base) while naming the right platform in the message.
type platformError struct {
	base error
	msg  string
}

func (e *platformError) Error() string { return e.msg }
func (e *platformError) Unwrap() error { return e.base }

// forPlatform returns base unchanged for Ringba (its messages already say Ringba) and a
// copy that names the platform otherwise.
func forPlatform(base error, platform string) error {
	p := strings.ToLower(strings.TrimSpace(platform))
	if p == "" || p == "ringba" {
		return base
	}
	name := platformLabel(p)
	msg := strings.ReplaceAll(base.Error(), "Ringba rejected the fire", name+" rejected the write")
	msg = strings.ReplaceAll(msg, "Ringba", name)
	return &platformError{base: base, msg: msg}
}

func platformLabel(p string) string {
	switch p {
	case "retreaver":
		return "Retreaver"
	case "ringba":
		return "Ringba"
	default:
		if p == "" {
			return p
		}
		return strings.ToUpper(p[:1]) + p[1:]
	}
}
