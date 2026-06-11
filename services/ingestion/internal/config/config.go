package config

// Config holds ingestion service configuration loaded from environment variables.
type Config struct {
	Port        string
	RedisURL    string
	KafkaBroker string
}
