package types

import "time"

// Ping is the canonical ping shape shared across Go services.
type Ping struct {
	TenantID   string
	Platform   string
	ReceivedAt time.Time
}
