package config

import (
	"strings"
	"testing"
)

func TestTranscriptionOffByDefault(t *testing.T) {
	t.Setenv("FEATURE_TRANSCRIPTION", "")
	c, err := LoadTranscription()
	if err != nil || c.Enabled || c.DailyCapMinutes != 800 || c.RetentionMonths != 24 || c.MaxAttempts != 5 {
		t.Fatalf("defaults: %+v %v", c, err)
	}
}

func TestTranscriptionProductionRefusesPlaceholders(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("FEATURE_TRANSCRIPTION", "true")
	t.Setenv("DEEPGRAM_API_KEY", "")
	_, err := LoadTranscription()
	if err == nil || !strings.Contains(err.Error(), "DEEPGRAM_API_KEY") || !strings.Contains(err.Error(), "ARCHIVE_S3") {
		t.Fatalf("want a refusal naming the key and the archive, got %v", err)
	}
	t.Setenv("DEEPGRAM_API_KEY", "real-looking-key-1234567890")
	t.Setenv("DEEPGRAM_BASE_URL", "http://fake-providers:8099")
	for k, v := range map[string]string{"ARCHIVE_S3_ENDPOINT": "https://s3.example", "ARCHIVE_S3_BUCKET": "b", "ARCHIVE_S3_KEY_ID": "id", "ARCHIVE_S3_SECRET_KEY": "s"} {
		t.Setenv(k, v)
	}
	if _, err := LoadTranscription(); err == nil || !strings.Contains(err.Error(), "DEEPGRAM_BASE_URL") {
		t.Fatalf("a fake Deepgram URL must be refused in production, got %v", err)
	}
	t.Setenv("DEEPGRAM_BASE_URL", "")
	if _, err := LoadTranscription(); err != nil {
		t.Fatalf("complete production config must load: %v", err)
	}
}

func TestTranscriptionOffInProductionNeedsNothing(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("FEATURE_TRANSCRIPTION", "false")
	t.Setenv("DEEPGRAM_API_KEY", "")
	if _, err := LoadTranscription(); err != nil {
		t.Fatalf("with the feature off nothing is required: %v", err)
	}
}
