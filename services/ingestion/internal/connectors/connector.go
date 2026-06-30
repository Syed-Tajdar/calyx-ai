package connectors

import (
	"net/http"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
)

// PlatformConnector parses and authenticates a single pay-per-call platform webhook.
type PlatformConnector interface {
	Platform() string
	Authenticate(r *http.Request, body []byte, webhookSecret string) bool
	BuildPing(
		tenantID string,
		r *http.Request,
		body []byte,
		encryptionKey []byte,
		receivedAt time.Time,
	) (models.Ping, error)
}
