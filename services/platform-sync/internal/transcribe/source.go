// Package transcribe is v2 Phase 2B: transcription (Deepgram) and the recording archive.
//
// A job fetches the call's audio ONCE, hashes it, archives those bytes and sends the same bytes to
// Deepgram. Recording URLs (Ringba's open for days, Retreaver's public and permanent) live only in
// memory for that one fetch: they are never logged, stored, published or sent to Deepgram. The worker
// runs only when FEATURE_TRANSCRIPTION=true; it does not touch the Live sync or the write-back.
package transcribe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
)

// MaxAudioBytes caps one recording in memory (a pay-per-call recording is a few MB).
const MaxAudioBytes = 200 << 20

// Audio is one recording, fetched once.
type Audio struct {
	Bytes       []byte
	ContentType string
}

// SHA256 is the hex digest of the audio, the archive key and the integrity proof.
func (a Audio) SHA256() string {
	sum := sha256.Sum256(a.Bytes)
	return hex.EncodeToString(sum[:])
}

// Call is what a source needs to find the recording.
type Call struct {
	Platform        string
	ExternalCallID  string
	OccurredAt      time.Time
	SyncMode        string // "real" | "mock"
	Credentials     map[string]string
	RecordingAccess map[string]string // Ringba Recording Access {username, password}; may be empty
}

// No-recording reasons (transcription_jobs.no_recording_reason). A normal, final state.
const (
	NoRecNotAnswered = "not_answered"
	NoRecDisabled    = "recording_disabled"
	NoRecBlockedCut  = "blocked_or_cut"
	NoRecFetchFailed = "fetch_failed"
	NoRecNoAccess    = "no_access"
)

// NoRecordingError: the call has no recording we can get. Final, not retried.
type NoRecordingError struct {
	Reason string
	Detail string
}

func (e *NoRecordingError) Error() string { return "no recording: " + e.Reason + ": " + e.Detail }

// TransientError: worth retrying later (network, 429, 5xx).
type TransientError struct{ Err error }

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

// Source fetches one call's audio.
type Source interface {
	Fetch(ctx context.Context, c Call) (Audio, error)
}

// ---- shared download --------------------------------------------------------------------------

var urlRE = regexp.MustCompile(`https?://\S+`)

// safeErr turns any error into text with no URL in it. Go's HTTP errors quote the full request
// URL (`Get "https://…": …`), which for a recording IS the secret.
func safeErr(err error) string {
	if err == nil {
		return ""
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return urlRE.ReplaceAllString(err.Error(), "[url]")
}

// noRedirect returns the 3xx response itself, so a Ringba link's Location is read, not followed
// with the Basic Auth header still attached.
func noRedirectClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// download GETs a recording link; optional Basic Auth. It follows at most one redirect itself,
// WITHOUT credentials (Ringba redirects to a pre-signed S3 link that refuses a second auth scheme).
func download(ctx context.Context, hc *http.Client, link, user, pass string) (Audio, error) {
	for hop := 0; hop < 2; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "bad recording link"}
		}
		if hop == 0 && user != "" {
			req.SetBasicAuth(user, pass)
		}
		resp, err := hc.Do(req)
		if err != nil {
			return Audio{}, &TransientError{Err: fmt.Errorf("recording fetch: %s", safeErr(err))}
		}
		switch {
		case resp.StatusCode >= 300 && resp.StatusCode < 400:
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: fmt.Sprintf("HTTP %d without Location", resp.StatusCode)}
			}
			next, err := req.URL.Parse(loc)
			if err != nil {
				return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "bad redirect"}
			}
			link = next.String()
			continue
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			resp.Body.Close()
			return Audio{}, &NoRecordingError{Reason: NoRecNoAccess, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			resp.Body.Close()
			return Audio{}, &TransientError{Err: fmt.Errorf("recording fetch: HTTP %d", resp.StatusCode)}
		case resp.StatusCode != http.StatusOK:
			resp.Body.Close()
			return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, MaxAudioBytes+1))
		resp.Body.Close()
		if err != nil {
			return Audio{}, &TransientError{Err: fmt.Errorf("recording read: %s", safeErr(err))}
		}
		if len(data) == 0 {
			return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "empty recording"}
		}
		if len(data) > MaxAudioBytes {
			return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "recording larger than the limit"}
		}
		ct := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
		if ct == "" || ct == "application/octet-stream" || ct == "binary/octet-stream" {
			ct = "audio/mpeg"
		}
		return Audio{Bytes: data, ContentType: ct}, nil
	}
	return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "too many redirects"}
}

// ---- Ringba -------------------------------------------------------------------------------------

// RingbaLookup is adapter.RingbaAdapter.LookupRecording (an interface for tests).
type RingbaLookup interface {
	LookupRecording(ctx context.Context, creds map[string]string, inboundCallID string, around time.Time) (adapter.RingbaRecordingInfo, error)
}

type RingbaSource struct {
	Lookup RingbaLookup
	HTTP   *http.Client // must not follow redirects (noRedirectClient)
}

func NewRingbaSource(lookup RingbaLookup) *RingbaSource {
	return &RingbaSource{Lookup: lookup, HTTP: noRedirectClient(2 * time.Minute)}
}

func (s *RingbaSource) Fetch(ctx context.Context, c Call) (Audio, error) {
	info, err := s.Lookup.LookupRecording(ctx, c.Credentials, c.ExternalCallID, c.OccurredAt)
	if errors.Is(err, adapter.ErrRecordingCallNotFound) {
		return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "call not found on Ringba"}
	}
	if err != nil {
		return Audio{}, &TransientError{Err: fmt.Errorf("ringba lookup: %s", safeErr(err))}
	}
	switch {
	case info.WasBlocked:
		return Audio{}, &NoRecordingError{Reason: NoRecBlockedCut, Detail: "call was blocked"}
	case !info.HasConnected && info.URL == "":
		return Audio{}, &NoRecordingError{Reason: NoRecNotAnswered, Detail: "call was not answered"}
	case info.URL == "":
		return Audio{}, &NoRecordingError{Reason: NoRecDisabled, Detail: "no recording on this call"}
	}
	user, pass := strings.TrimSpace(c.RecordingAccess["username"]), c.RecordingAccess["password"]
	if user == "" {
		// Ringba opens recordings without auth only for the first days; after that the Recording Access
		// credentials are required. Try without, and report no_access clearly if refused.
		a, err := download(ctx, s.HTTP, info.URL, "", "")
		var nr *NoRecordingError
		if errors.As(err, &nr) && nr.Reason == NoRecNoAccess {
			nr.Detail = "Recording Access credentials are not set for this Ringba integration"
		}
		return a, err
	}
	return download(ctx, s.HTTP, info.URL, user, pass)
}

// ---- Retreaver ----------------------------------------------------------------------------------

// RetreaverLookup is adapter.RetreaverCallClient.RecordingLink.
type RetreaverLookup interface {
	RecordingLink(ctx context.Context, creds map[string]string, uuid string) (adapter.RetreaverRecordingInfo, error)
}

type RetreaverSource struct {
	Lookup RetreaverLookup
	HTTP   *http.Client
}

func NewRetreaverSource(lookup RetreaverLookup) *RetreaverSource {
	return &RetreaverSource{Lookup: lookup, HTTP: noRedirectClient(2 * time.Minute)}
}

func (s *RetreaverSource) Fetch(ctx context.Context, c Call) (Audio, error) {
	info, err := s.Lookup.RecordingLink(ctx, c.Credentials, c.ExternalCallID)
	if errors.Is(err, adapter.ErrRecordingCallNotFound) {
		return Audio{}, &NoRecordingError{Reason: NoRecFetchFailed, Detail: "call not found on Retreaver"}
	}
	if err != nil {
		return Audio{}, &TransientError{Err: fmt.Errorf("retreaver lookup: %s", safeErr(err))}
	}
	if info.URL == "" {
		switch {
		case info.Blocked:
			return Audio{}, &NoRecordingError{Reason: NoRecBlockedCut, Detail: "call was blocked or cut"}
		case !info.Connected:
			return Audio{}, &NoRecordingError{Reason: NoRecNotAnswered, Detail: "call was not answered"}
		default:
			return Audio{}, &NoRecordingError{Reason: NoRecDisabled, Detail: "no recording on this call"}
		}
	}
	return download(ctx, s.HTTP, info.URL, "", "")
}

// ---- Test (mock) integrations ---------------------------------------------------------------------

// MockSource serves Test-mode integrations (laptop / demo only; Test mode is off in production): a
// short, deterministic silent WAV, so the whole pipeline can run against the fake Deepgram.
type MockSource struct{}

func (MockSource) Fetch(_ context.Context, c Call) (Audio, error) {
	return Audio{Bytes: silentWAV(c.ExternalCallID), ContentType: "audio/wav"}, nil
}

// silentWAV: 1 s of 8 kHz 8-bit mono silence, with the call id in a trailing chunk so each call has
// its own hash.
func silentWAV(seed string) []byte {
	const rate, n = 8000, 8000
	extra := []byte(seed)
	if len(extra)%2 == 1 {
		extra = append(extra, 0)
	}
	le32 := func(v int) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
	le16 := func(v int) []byte { return []byte{byte(v), byte(v >> 8)} }
	var b []byte
	b = append(b, "RIFF"...)
	b = append(b, le32(4+(8+16)+(8+n)+(8+len(extra)))...)
	b = append(b, "WAVEfmt "...)
	b = append(b, le32(16)...)
	b = append(b, le16(1)...)    // PCM
	b = append(b, le16(1)...)    // mono
	b = append(b, le32(rate)...) // sample rate
	b = append(b, le32(rate)...) // byte rate
	b = append(b, le16(1)...)    // block align
	b = append(b, le16(8)...)    // bits
	b = append(b, "data"...)
	b = append(b, le32(n)...)
	for i := 0; i < n; i++ {
		b = append(b, 128)
	}
	b = append(b, "cyid"...)
	b = append(b, le32(len(extra))...)
	return append(b, extra...)
}
