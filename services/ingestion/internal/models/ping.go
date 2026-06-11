package models

import "time"

// Ping represents a normalized inbound call ping.
type Ping struct {
	TenantID   string
	Platform   string
	ReceivedAt time.Time
}
