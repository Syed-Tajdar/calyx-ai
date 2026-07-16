package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/connectors"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/producer"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/store"
)

// WebhookHandler serves platform webhook ingestion at /webhooks/{platform}/{token}.
type WebhookHandler struct {
	cfg      config.Config
	store    *store.IntegrationStore
	dedup    *dedup.Checker
	producer *producer.Producer
	logger   *slog.Logger
}

// NewWebhookHandler creates a multi-platform webhook handler.
func NewWebhookHandler(
	cfg config.Config,
	integrationStore *store.IntegrationStore,
	dedupChecker *dedup.Checker,
	pingProducer *producer.Producer,
	logger *slog.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		cfg:      cfg,
		store:    integrationStore,
		dedup:    dedupChecker,
		producer: pingProducer,
		logger:   logger,
	}
}

// ServeHTTP handles GET and POST webhook requests for all registered platforms.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
	defer cancel()

	platform, token, ok := parseWebhookPath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid webhook path", http.StatusBadRequest)
		return
	}

	connector, err := connectors.Get(platform)
	if err != nil {
		http.Error(w, "unknown platform", http.StatusNotFound)
		return
	}

	integration, err := h.store.LookupByToken(ctx, token)
	if err != nil {
		if errors.Is(err, store.ErrIntegrationNotFound) {
			h.logger.Warn("webhook token not found",
				"service", "ingestion", "platform", platform, "client_ip", clientIP(r))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.logger.Error("webhook token lookup failed",
			"service", "ingestion", "platform", platform, "client_ip", clientIP(r), "error", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	if integration.Platform != platform {
		h.logger.Warn("webhook platform mismatch",
			"service", "ingestion",
			"path_platform", platform,
			"integration_platform", integration.Platform,
			"tenant_id", integration.TenantID,
		)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := readBody(r, h.cfg.MaxBodyBytes)
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	if integration.WebhookSecret == "" {
		h.logger.Error("integration missing webhook secret",
			"service", "ingestion", "tenant_id", integration.TenantID, "platform", platform)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !connector.Authenticate(r, body, integration.WebhookSecret) {
		h.logger.Warn("webhook auth failed",
			"service", "ingestion",
			"platform", platform,
			"tenant_id", integration.TenantID,
			"client_ip", clientIP(r),
			"payload_hash", security.PayloadHash(body, r.URL.RawQuery),
		)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	receivedAt := time.Now().UTC()
	ping, err := connector.BuildPing(integration.TenantID, r, body, h.cfg.EncryptionKey, receivedAt)
	if err != nil {
		if errors.Is(err, connectors.ErrUnsupportedMethod) {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.logger.Warn("webhook parse failed",
			"service", "ingestion", "platform", platform, "tenant_id", integration.TenantID, "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	decision, err := h.dedup.Evaluate(ctx, ping.TenantID, ping.Platform, dedup.Content{
		ExternalCallID:    ping.ExternalCallID,
		Revenue:           ping.Revenue,
		Payout:            ping.Payout,
		State:             ping.State,
		EventType:         ping.EventType,
		CampaignID:        ping.CampaignID,
		PublisherID:       ping.PublisherID,
		TargetID:          ping.TargetID,
		PlatformDuplicate: ping.PlatformDuplicate,
	})
	if err != nil {
		h.logger.Error("dedup check failed",
			"service", "ingestion", "tenant_id", ping.TenantID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if decision == dedup.Unchanged {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","dedup":"unchanged"}`))
		return
	}
	ping.IsDuplicate = false

	if err := h.producer.Publish(ctx, ping); err != nil {
		h.logger.Error("kafka publish failed",
			"service", "ingestion", "tenant_id", ping.TenantID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("ping ingested",
		"service", "ingestion",
		"tenant_id", ping.TenantID,
		"platform", ping.Platform,
		"external_call_id", ping.ExternalCallID,
		"event_type", ping.EventType,
		"is_duplicate", ping.IsDuplicate,
		"platform_duplicate", ping.PlatformDuplicate,
		"call_key_present", ping.CallKey != "",
		"dedup_decision", decision.String(),
	)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func parseWebhookPath(path string) (platform, token string, ok bool) {
	rest := strings.TrimPrefix(path, "/webhooks/")
	if rest == path || rest == "" {
		return "", "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[1], "/") {
		return "", "", false
	}
	return parts[0], parts[1], true
}
