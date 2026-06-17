package loader

import (
	"time"
)

// Binding is a tenant provider binding row.
type Binding struct {
	ID         string
	TenantID   string
	Domain     string
	ProviderID string
	ConfigJSON []byte
	UpdatedAt  time.Time
}

// Resolved is the outcome of provider resolution.
type Resolved struct {
	ProviderID      string         `json:"provider_id"`
	GoAdapter       string         `json:"go_adapter"`
	ConfigJSON      map[string]any `json:"config_json"`
	MockMode        bool           `json:"mock_mode"`
	CacheHitBinding bool           `json:"cache_hit_binding"`
	CacheHitSecret  bool           `json:"cache_hit_secret"`
	Output          map[string]any `json:"output,omitempty"`
}

// BindingStore loads bindings from Postgres.
type BindingStore interface {
	GetActiveBinding(tenantID, domain string) (*Binding, error)
}

// BindingCache caches binding rows in Redis.
type BindingCache interface {
	Get(tenantID, domain string) (*Binding, bool, error)
	Set(tenantID, domain string, binding *Binding) error
	Delete(tenantID, domain string) error
}

// SecretStore loads encrypted credentials.
type SecretStore interface {
	GetSecrets(tenantID, platform string) (map[string]string, error)
}

// SecretCache caches decrypted secrets in-process.
type SecretCache interface {
	Get(key string) (map[string]string, bool)
	Set(key string, secrets map[string]string)
}
