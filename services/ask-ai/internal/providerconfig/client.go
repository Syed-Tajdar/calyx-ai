package providerconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Resolved is providerconfig resolution output.
type Resolved struct {
	ProviderID string         `json:"provider_id"`
	GoAdapter  string         `json:"go_adapter"`
	ConfigJSON map[string]any `json:"config_json"`
	MockMode   bool           `json:"mock_mode"`
	Output     map[string]any `json:"output,omitempty"`
}

// Client calls POST /v1/resolve on providerconfig.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a providerconfig HTTP client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type resolveRequest struct {
	TenantID string `json:"tenant_id"`
	Domain   string `json:"domain"`
	TraceID  string `json:"trace_id"`
}

// Resolve fetches LLM provider config for a tenant.
func (c *Client) Resolve(ctx context.Context, tenantID, traceID string) (Resolved, error) {
	if traceID == "" {
		traceID = "ask"
	}
	body, err := json.Marshal(resolveRequest{TenantID: tenantID, Domain: "llm", TraceID: traceID})
	if err != nil {
		return Resolved{}, fmt.Errorf("providerconfig.Resolve marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/resolve", bytes.NewReader(body))
	if err != nil {
		return Resolved{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Resolved{}, fmt.Errorf("providerconfig.Resolve http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Resolved{}, fmt.Errorf("providerconfig.Resolve: status %d", resp.StatusCode)
	}
	var resolved Resolved
	if err := json.NewDecoder(resp.Body).Decode(&resolved); err != nil {
		return Resolved{}, fmt.Errorf("providerconfig.Resolve decode: %w", err)
	}
	return resolved, nil
}
