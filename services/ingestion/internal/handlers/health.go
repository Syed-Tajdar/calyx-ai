package handlers

import (
	"context"
	"net/http"
	"time"
)

// ReadinessChecker verifies downstream dependencies.
type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	redis    ReadinessChecker
	kafka    ReadinessChecker
	postgres ReadinessChecker
}

// NewHealthHandler creates health probe handlers.
func NewHealthHandler(redis, kafka, postgres ReadinessChecker) *HealthHandler {
	return &HealthHandler{redis: redis, kafka: kafka, postgres: postgres}
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"redis", h.redis.Ping},
		{"kafka", h.kafka.Ping},
		{"postgres", h.postgres.Ping},
	}

	for _, check := range checks {
		if err := check.fn(ctx); err != nil {
			http.Error(w, check.name+" unavailable", http.StatusServiceUnavailable)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
