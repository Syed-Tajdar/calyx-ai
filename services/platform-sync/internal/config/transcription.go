package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// TranscriptionConfig is v2 Phase 2B. Everything is off unless FEATURE_TRANSCRIPTION=true: with it off
// the worker never starts and no Deepgram, archive or recording request is ever made.
type TranscriptionConfig struct {
	Enabled          bool    // FEATURE_TRANSCRIPTION
	DeepgramAPIKey   string  // DEEPGRAM_API_KEY (server env only)
	DeepgramBaseURL  string  // DEEPGRAM_BASE_URL (tests / laptop fake only)
	DailyCapMinutes  float64 // TRANSCRIBE_DAILY_MINUTES_CAP, hard stop per Eastern day (default 800)
	MaxAttempts      int     // TRANSCRIBE_MAX_ATTEMPTS (default 5)
	RetentionMonths  int     // RECORDING_RETENTION_MONTHS (default 24)
	SpanishPattern   string  // TRANSCRIBE_SPANISH_CAMPAIGN_PATTERN (regexp, lower case; default spanish|español|espanol)
	ArchiveEndpoint  string  // ARCHIVE_S3_ENDPOINT
	ArchiveRegion    string  // ARCHIVE_S3_REGION
	ArchiveBucket    string  // ARCHIVE_S3_BUCKET
	ArchiveKeyID     string  // ARCHIVE_S3_KEY_ID (application key limited to this bucket)
	ArchiveSecretKey string  // ARCHIVE_S3_SECRET_KEY
}

func (t TranscriptionConfig) ArchiveConfigured() bool {
	return t.ArchiveEndpoint != "" && t.ArchiveBucket != "" && t.ArchiveKeyID != "" && t.ArchiveSecretKey != ""
}

// LoadTranscription reads the 2B env. In production with the feature on, a missing or placeholder
// Deepgram key or archive credential refuses to start (like the other secrets).
func LoadTranscription() (TranscriptionConfig, error) {
	t := TranscriptionConfig{
		Enabled:          EnvFlag(os.Getenv("FEATURE_TRANSCRIPTION")),
		DeepgramAPIKey:   strings.TrimSpace(os.Getenv("DEEPGRAM_API_KEY")),
		DeepgramBaseURL:  strings.TrimSpace(os.Getenv("DEEPGRAM_BASE_URL")),
		DailyCapMinutes:  800,
		MaxAttempts:      5,
		RetentionMonths:  24,
		SpanishPattern:   envOrDefault("TRANSCRIBE_SPANISH_CAMPAIGN_PATTERN", "spanish|español|espanol"),
		ArchiveEndpoint:  strings.TrimSpace(os.Getenv("ARCHIVE_S3_ENDPOINT")),
		ArchiveRegion:    strings.TrimSpace(os.Getenv("ARCHIVE_S3_REGION")),
		ArchiveBucket:    strings.TrimSpace(os.Getenv("ARCHIVE_S3_BUCKET")),
		ArchiveKeyID:     strings.TrimSpace(os.Getenv("ARCHIVE_S3_KEY_ID")),
		ArchiveSecretKey: strings.TrimSpace(os.Getenv("ARCHIVE_S3_SECRET_KEY")),
	}
	if v := os.Getenv("TRANSCRIBE_DAILY_MINUTES_CAP"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 {
			return t, fmt.Errorf("TRANSCRIBE_DAILY_MINUTES_CAP: must be a number >= 0")
		}
		t.DailyCapMinutes = f
	}
	if v := os.Getenv("TRANSCRIBE_MAX_ATTEMPTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return t, fmt.Errorf("TRANSCRIBE_MAX_ATTEMPTS: must be an integer >= 1")
		}
		t.MaxAttempts = n
	}
	if v := os.Getenv("RECORDING_RETENTION_MONTHS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return t, fmt.Errorf("RECORDING_RETENTION_MONTHS: must be an integer >= 1")
		}
		t.RetentionMonths = n
	}
	if t.Enabled && os.Getenv("APP_ENV") == "production" {
		var bad []string
		if t.DeepgramAPIKey == "" || strings.Contains(strings.ToLower(t.DeepgramAPIKey), "change_me") || strings.HasPrefix(t.DeepgramAPIKey, "fake") {
			bad = append(bad, "DEEPGRAM_API_KEY")
		}
		if t.DeepgramBaseURL != "" {
			bad = append(bad, "DEEPGRAM_BASE_URL (must be unset in production)")
		}
		if !t.ArchiveConfigured() {
			bad = append(bad, "ARCHIVE_S3_ENDPOINT/BUCKET/KEY_ID/SECRET_KEY")
		}
		if len(bad) > 0 {
			return t, fmt.Errorf("FEATURE_TRANSCRIPTION=true in production refuses to start with missing/placeholder settings: %v", bad)
		}
	}
	return t, nil
}
