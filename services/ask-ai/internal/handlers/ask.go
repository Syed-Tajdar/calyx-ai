package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Syed-Tajdar/calyx-ai/services/ask-ai/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ask-ai/internal/pipeline"
)

// AskHandler serves POST /v1/ask and GET /v1/history.
type AskHandler struct {
	pipeline *pipeline.Service
	logger   *slog.Logger
}

// NewAskHandler creates an Ask handler.
func NewAskHandler(pipe *pipeline.Service, logger *slog.Logger) *AskHandler {
	return &AskHandler{pipeline: pipe, logger: logger}
}

type askBody struct {
	TenantID string `json:"tenant_id"`
	Question string `json:"question"`
	TraceID  string `json:"trace_id"`
}

// ServeHTTP routes ask endpoints.
func (h *AskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/ask":
		h.handleAsk(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/history":
		h.handleHistory(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *AskHandler) handleAsk(w http.ResponseWriter, r *http.Request) {
	var body askBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.TenantID == "" || body.Question == "" {
		http.Error(w, "tenant_id and question required", http.StatusBadRequest)
		return
	}
	if len(body.Question) > 2000 {
		http.Error(w, "question too long", http.StatusBadRequest)
		return
	}

	resp, err := h.pipeline.Ask(r.Context(), models.AskRequest{
		TenantID: body.TenantID,
		Question: body.Question,
		TraceID:  body.TraceID,
	})
	if err != nil {
		h.logger.Error("ask failed", "service", "ask-ai", "tenant_id", body.TenantID, "error", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *AskHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		http.Error(w, "tenant_id required", http.StatusBadRequest)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	records, err := h.pipeline.ListHistory(r.Context(), tenantID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": records})
}
