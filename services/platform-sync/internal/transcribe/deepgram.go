package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Deepgram pre-recorded API, bytes upload (POST /v1/listen with the audio as the body). No URL is ever
// sent to Deepgram. Parameters (docs/v2-intelligence-report.md §4.2-4.5, [PROVEN-docs]):
//   - model=nova-3, smart_format, punctuate, utterances, diarize_model=latest
//   - mip_opt_out=true on EVERY request (owner decision Q7): opt out of the Model Improvement Program
//   - English: detect_language=true, redact=pci, redact=ssn
//   - Spanish campaigns: language=multi (code-switching), redact=numbers — entity redaction is English-only
//
// The Calyx scrub (scrub.go) runs on every transcript afterwards, whatever Deepgram redacted.

type Utterance struct {
	Index   int    `json:"idx"`
	Speaker int    `json:"speaker"`
	StartMs int    `json:"start_ms"`
	EndMs   int    `json:"end_ms"`
	Text    string `json:"text"`
}

type TranscribeOptions struct {
	Spanish bool
}

type TranscriptResult struct {
	RequestID  string
	Model      string
	Language   string
	Duration   float64 // seconds, as billed
	Redaction  string  // what Deepgram was asked to redact
	Utterances []Utterance
}

type Transcriber interface {
	Transcribe(ctx context.Context, audio Audio, opts TranscribeOptions) (TranscriptResult, error)
}

// FinalError: Deepgram refused the request (4xx other than 429). Not retried.
type FinalError struct{ Err error }

func (e *FinalError) Error() string { return e.Err.Error() }
func (e *FinalError) Unwrap() error { return e.Err }

type Deepgram struct {
	BaseURL string // https://api.deepgram.com
	APIKey  string
	HTTP    *http.Client
}

func NewDeepgram(baseURL, apiKey string) *Deepgram {
	if baseURL == "" {
		baseURL = "https://api.deepgram.com"
	}
	return &Deepgram{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, HTTP: &http.Client{Timeout: 10 * time.Minute}}
}

// ListenQuery builds the query string. Exported for the tests that pin every parameter.
func ListenQuery(opts TranscribeOptions) (url.Values, string) {
	q := url.Values{}
	q.Set("model", "nova-3")
	q.Set("smart_format", "true")
	q.Set("punctuate", "true")
	q.Set("utterances", "true")
	q.Set("diarize_model", "latest")
	q.Set("mip_opt_out", "true")
	if opts.Spanish {
		q.Set("language", "multi")
		q.Add("redact", "numbers")
		return q, "deepgram:numbers"
	}
	q.Set("detect_language", "true")
	q.Add("redact", "pci")
	q.Add("redact", "ssn")
	return q, "deepgram:pci,ssn"
}

var keyRE = regexp.MustCompile(`(?i)(token|key|authorization)[^,;]{0,8}[:=]\s*\S+`)

func (d *Deepgram) Transcribe(ctx context.Context, audio Audio, opts TranscribeOptions) (TranscriptResult, error) {
	if d.APIKey == "" {
		return TranscriptResult{}, &FinalError{Err: fmt.Errorf("deepgram: DEEPGRAM_API_KEY is not set")}
	}
	q, redaction := ListenQuery(opts)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.BaseURL+"/v1/listen?"+q.Encode(), bytes.NewReader(audio.Bytes))
	if err != nil {
		return TranscriptResult{}, &FinalError{Err: fmt.Errorf("deepgram: %s", safeErr(err))}
	}
	req.Header.Set("Authorization", "Token "+d.APIKey)
	req.Header.Set("Content-Type", audio.ContentType)
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return TranscriptResult{}, &TransientError{Err: fmt.Errorf("deepgram: %s", safeErr(err))}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return TranscriptResult{}, &TransientError{Err: fmt.Errorf("deepgram: HTTP %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		msg := keyRE.ReplaceAllString(string(body), "$1=[REDACTED]")
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return TranscriptResult{}, &FinalError{Err: fmt.Errorf("deepgram: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(msg))}
	}
	r, err := parseListenResponse(body)
	if err != nil {
		return TranscriptResult{}, &FinalError{Err: err}
	}
	r.Redaction = redaction
	return r, nil
}

func parseListenResponse(body []byte) (TranscriptResult, error) {
	var p struct {
		Metadata struct {
			RequestID string   `json:"request_id"`
			Duration  float64  `json:"duration"`
			Models    []string `json:"models"`
			ModelInfo map[string]struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Arch    string `json:"arch"`
			} `json:"model_info"`
		} `json:"metadata"`
		Results struct {
			Channels []struct {
				DetectedLanguage string `json:"detected_language"`
				Alternatives     []struct {
					Languages []string `json:"languages"`
				} `json:"alternatives"`
			} `json:"channels"`
			Utterances []struct {
				Start      float64 `json:"start"`
				End        float64 `json:"end"`
				Speaker    *int    `json:"speaker"`
				Transcript string  `json:"transcript"`
			} `json:"utterances"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return TranscriptResult{}, fmt.Errorf("deepgram: parse response: %w", err)
	}
	r := TranscriptResult{RequestID: p.Metadata.RequestID, Duration: p.Metadata.Duration, Model: "nova-3"}
	for _, id := range p.Metadata.Models {
		if mi, ok := p.Metadata.ModelInfo[id]; ok {
			r.Model = strings.Trim(mi.Arch+" "+mi.Name+" "+mi.Version, " ")
			break
		}
	}
	if len(p.Results.Channels) > 0 {
		ch := p.Results.Channels[0]
		r.Language = ch.DetectedLanguage
		if r.Language == "" && len(ch.Alternatives) > 0 && len(ch.Alternatives[0].Languages) > 0 {
			r.Language = ch.Alternatives[0].Languages[0]
		}
	}
	for i, u := range p.Results.Utterances {
		sp := -1
		if u.Speaker != nil {
			sp = *u.Speaker
		}
		r.Utterances = append(r.Utterances, Utterance{
			Index: i, Speaker: sp, StartMs: int(u.Start * 1000), EndMs: int(u.End * 1000),
			Text: strings.TrimSpace(u.Transcript),
		})
	}
	return r, nil
}
