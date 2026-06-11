package config

// Config holds consumer service configuration loaded from environment variables.
type Config struct {
	Port             string
	KafkaBroker      string
	KafkaTopic       string
	ClickHouseDSN    string
	BatchSize        int
	BatchFlushMillis int
}
