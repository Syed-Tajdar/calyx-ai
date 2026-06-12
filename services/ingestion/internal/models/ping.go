package models

import "time"

// Ping represents a normalized inbound call ping ready for Kafka.
type Ping struct {
	TenantID       string
	Platform       string
	ExternalCallID string
	ANIHash        string
	ANIEncrypted   []byte
	CampaignID     string
	PublisherID    string
	TargetID       string
	State          string
	EventType      string
	Revenue        float64
	Payout         float64
	IsDuplicate    bool
	ReceivedAt     time.Time
}
