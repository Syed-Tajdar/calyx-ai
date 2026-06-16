package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds scoring service configuration.
type Config struct {
	Port                      string
	MockMode                  bool
	RedisURL                  string
	KafkaBrokers              []string
	TopicTranscriptReady      string
	TopicCallScored           string
	TopicPings                string
	TopicScoringDLQ           string
	ConsumerGroup             string
	MockEmitterGroup          string
	ClickHouseAddr            string
	ClickHouseDB              string
	ClickHouseUser            string
	ClickHousePass            string
	TranscriptionProvider     string
	AIProvider                string
	RecordingSource           string
	DeepgramAPIKey            string
	DeepgramCallbackSecret    string
	OpenAIAPIKey              string
	MockEmitterEnabled        bool
}

// Load reads and validates configuration from environment.
func Load() (Config, error) {
	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)

	cfg := Config{
		Port:                  envOrDefault("SCORING_PORT", "8082"),
		MockMode:              mockMode,
		RedisURL:              envOrDefault("REDIS_URL", "redis://localhost:6379"),
		KafkaBrokers:          []string{envOrDefault("KAFKA_BROKER", "localhost:9092")},
		TopicTranscriptReady:  envOrDefault("KAFKA_TOPIC_CALL_TRANSCRIPT_READY", "callloom.calls.call_transcript_ready.v1"),
		TopicCallScored:       envOrDefault("KAFKA_TOPIC_CALL_SCORED", "callloom.calls.call_scored.v1"),
		TopicPings:            envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1"),
		TopicScoringDLQ:       envOrDefault("KAFKA_TOPIC_SCORING_DLQ", "callloom.scoring.dlq.v1"),
		ConsumerGroup:         envOrDefault("KAFKA_SCORING_GROUP", "callloom-scoring"),
		MockEmitterGroup:      envOrDefault("KAFKA_MOCK_EMITTER_GROUP", "callloom-scoring-mock-emitter"),
		ClickHouseAddr:        envOrDefault("CLICKHOUSE_ADDR", "clickhouse:9000"),
		ClickHouseDB:          envOrDefault("CLICKHOUSE_DB", "default"),
		ClickHouseUser:        envOrDefault("CLICKHOUSE_USER", "default"),
		ClickHousePass:        envOrDefault("CLICKHOUSE_PASSWORD", ""),
		TranscriptionProvider: resolveProvider("TRANSCRIPTION_PROVIDER", mockMode, "deepgram"),
		AIProvider:            resolveProvider("AI_PROVIDER", mockMode, "openai"),
		RecordingSource:       resolveProvider("RECORDING_SOURCE", mockMode, "ringba"),
		DeepgramAPIKey:        os.Getenv("DEEPGRAM_API_KEY"),
		DeepgramCallbackSecret: os.Getenv("DEEPGRAM_CALLBACK_SECRET"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		MockEmitterEnabled:    parseBoolDefault(os.Getenv("MOCK_EMITTER_ENABLED"), mockMode),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.MockMode {
		return nil
	}
	if c.TranscriptionProvider == "deepgram" {
		if c.DeepgramAPIKey == "" || c.DeepgramCallbackSecret == "" {
			return fmt.Errorf("config.validate: DEEPGRAM_API_KEY and DEEPGRAM_CALLBACK_SECRET required when MOCK_MODE=false and TRANSCRIPTION_PROVIDER=deepgram")
		}
	}
	if c.AIProvider == "openai" && c.OpenAIAPIKey == "" {
		return fmt.Errorf("config.validate: OPENAI_API_KEY required when MOCK_MODE=false and AI_PROVIDER=openai")
	}
	return nil
}

func resolveProvider(envKey string, mockMode bool, realDefault string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if mockMode {
		return "mock"
	}
	return realDefault
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBoolDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
