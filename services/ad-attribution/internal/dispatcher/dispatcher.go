package dispatcher

import (
	"context"
	"time"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/models"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/platforms"
)

// Service fires conversions with retry backoff.
type Service struct {
	maxRetries int
}

// NewService creates a fire dispatcher.
func NewService(maxRetries int) *Service {
	return &Service{maxRetries: maxRetries}
}

// Fire invokes the platform with exponential backoff retries.
func (s *Service) Fire(ctx context.Context, platform platforms.Platform, req models.FireRequest) (models.FireResult, error) {
	var last models.FireResult
	backoff := time.Second
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		res, err := platform.Fire(ctx, req)
		last = res
		if err == nil && res.Success {
			return res, nil
		}
		if attempt == s.maxRetries {
			if err != nil {
				return last, err
			}
			return last, nil
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}
	return last, nil
}
