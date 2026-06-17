package platforms

import (
	"context"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
)

// Platform fires server-side conversion events to an ad network.
type Platform interface {
	Name() string
	Fire(ctx context.Context, req models.FireRequest) (models.FireResult, error)
}
