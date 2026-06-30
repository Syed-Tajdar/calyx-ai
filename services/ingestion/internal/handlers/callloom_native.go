package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// CallLoomNativeHandler serves direct Call Loom data source endpoints (not webhooks).
type CallLoomNativeHandler struct{}

// NewCallLoomNativeHandler creates the Call Loom native source handler.
func NewCallLoomNativeHandler() *CallLoomNativeHandler {
	return &CallLoomNativeHandler{}
}

// ListPings handles GET /sources/callloom-native/v1/pings.
func (h *CallLoomNativeHandler) ListPings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !mockModeEnabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_implemented",
			"message": "Call Loom native direct integration is not configured",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"source":  "callloom-native",
		"status":  "mock",
		"message": "MOCK_MODE stub — configure native connector for production",
		"data":    []any{},
	})
}

func mockModeEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("MOCK_MODE")))
	return v == "true" || v == "1" || v == "yes"
}
