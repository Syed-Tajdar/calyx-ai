package platforms

import (
	"context"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/models"
)

// Platform fires server-side conversion events to an ad network.
type Platform interface {
	Name() string
	Fire(ctx context.Context, req models.FireRequest) (models.FireResult, error)
}
