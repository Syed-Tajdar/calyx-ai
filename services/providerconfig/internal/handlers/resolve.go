package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/providerconfig/internal/loader"
)

// ResolveHandler handles POST /v1/resolve.
type ResolveHandler struct {
	resolver *loader.Resolver
	logger   *slog.Logger
}

// NewResolveHandler creates a resolve handler.
func NewResolveHandler(resolver *loader.Resolver, logger *slog.Logger) *ResolveHandler {
	return &ResolveHandler{resolver: resolver, logger: logger}
}

type resolveRequest struct {
	TenantID string `json:"tenant_id"`
	Domain   string `json:"domain"`
	TraceID  string `json:"trace_id"`
}

// ServeHTTP resolves provider config for a tenant+domain.
func (h *ResolveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.Domain == "" {
		http.Error(w, "tenant_id and domain required", http.StatusBadRequest)
		return
	}
	if req.TraceID == "" {
		req.TraceID = "resolve"
	}

	resolved, err := h.resolver.Resolve(req.TenantID, req.Domain, req.TraceID)
	if err != nil {
		h.logger.Error("resolve failed",
			"service", "providerconfig",
			"tenant_id", req.TenantID,
			"domain", req.Domain,
			"trace_id", req.TraceID,
			"error", err,
		)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	h.logger.Info("provider resolved",
		"service", "providerconfig",
		"tenant_id", req.TenantID,
		"domain", req.Domain,
		"trace_id", req.TraceID,
		"provider_id", resolved.ProviderID,
		"cache_hit_binding", resolved.CacheHitBinding,
		"cache_hit_secret", resolved.CacheHitSecret,
	)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resolved)
}
