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
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/normalizer"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/producer"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/store"
)

// RingbaHandler handles Ringba tracking pixel webhooks.
type RingbaHandler struct {
	cfg      config.Config
	store    *store.IntegrationStore
	dedup    *dedup.Checker
	producer *producer.Producer
	logger   *slog.Logger
}

// NewRingbaHandler creates a Ringba webhook handler.
func NewRingbaHandler(
	cfg config.Config,
	integrationStore *store.IntegrationStore,
	dedupChecker *dedup.Checker,
	pingProducer *producer.Producer,
	logger *slog.Logger,
) *RingbaHandler {
	return &RingbaHandler{
		cfg:      cfg,
		store:    integrationStore,
		dedup:    dedupChecker,
		producer: pingProducer,
		logger:   logger,
	}
}

// ServeHTTP handles GET and POST Ringba pixel requests.
// Route: /webhooks/ringba/{webhook_token}
func (h *RingbaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
	defer cancel()

	token := strings.TrimPrefix(r.URL.Path, "/webhooks/ringba/")
	if token == "" || strings.Contains(token, "/") {
		http.Error(w, "invalid webhook token", http.StatusBadRequest)
		return
	}

	integration, err := h.store.LookupByToken(ctx, token)
	if err != nil {
		if errors.Is(err, store.ErrIntegrationNotFound) {
			h.logger.Warn("webhook token not found",
				"service", "ingestion", "client_ip", clientIP(r))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.logger.Error("webhook token lookup failed",
			"service", "ingestion", "client_ip", clientIP(r), "error", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	body, err := readBody(r, h.cfg.MaxBodyBytes)
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	if integration.WebhookSecret == "" {
		h.logger.Error("integration missing webhook secret",
			"service", "ingestion", "tenant_id", integration.TenantID)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !security.VerifyRingbaHMAC(r, body, integration.WebhookSecret) {
		h.logger.Warn("ringba hmac verification failed",
			"service", "ingestion",
			"tenant_id", integration.TenantID,
			"client_ip", clientIP(r),
			"payload_hash", security.PayloadHash(body, r.URL.RawQuery),
		)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload connectors.RingbaPayload
	switch r.Method {
	case http.MethodGet:
		payload, err = connectors.ParseRingbaQuery(r.URL.Query())
	case http.MethodPost:
		if len(body) > 0 && r.Header.Get("Content-Type") == "application/json" {
			payload, err = connectors.ParseRingbaJSON(body)
		} else {
			payload, err = connectors.ParseRingbaQuery(r.URL.Query())
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		h.logger.Warn("ringba parse failed",
			"service", "ingestion", "tenant_id", integration.TenantID, "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	receivedAt := time.Now().UTC()
	ping, err := normalizer.NormalizeRingba(integration.TenantID, payload, h.cfg.EncryptionKey, receivedAt)
	if err != nil {
		h.logger.Error("normalize failed",
			"service", "ingestion", "tenant_id", integration.TenantID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	isDup, err := h.dedup.IsDuplicate(ctx, ping.TenantID, ping.Platform, ping.ExternalCallID)
	if err != nil {
		h.logger.Error("dedup check failed",
			"service", "ingestion", "tenant_id", ping.TenantID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if isDup {
		ping.IsDuplicate = true
	}

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
	)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
