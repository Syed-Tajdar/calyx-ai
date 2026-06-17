package loader

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/catalog"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/registry"
)

// Resolver resolves tenant provider configuration.
type Resolver struct {
	cfg         config.Config
	catalog     catalog.Catalog
	store       BindingStore
	cache       BindingCache
	secrets     SecretStore
	secretCache SecretCache
	decryptor   *crypto.Decryptor
}

// NewResolver creates a provider resolver.
func NewResolver(
	cfg config.Config,
	cat catalog.Catalog,
	store BindingStore,
	cache BindingCache,
	secrets SecretStore,
	secretCache SecretCache,
	decryptor *crypto.Decryptor,
) *Resolver {
	return &Resolver{
		cfg:         cfg,
		catalog:     cat,
		store:       store,
		cache:       cache,
		secrets:     secrets,
		secretCache: secretCache,
		decryptor:   decryptor,
	}
}

// Resolve applies CLARIFY 1-2 precedence and returns adapter output.
func (r *Resolver) Resolve(tenantID, domain, traceID string) (Resolved, error) {
	resolved := Resolved{MockMode: r.cfg.MockMode}

	binding, cacheHit, err := r.loadBinding(tenantID, domain)
	if err != nil {
		return Resolved{}, err
	}
	resolved.CacheHitBinding = cacheHit

	var configMap map[string]any
	if binding != nil {
		configMap, err = adapter.ParseConfigJSON(binding.ConfigJSON)
		if err != nil {
			return Resolved{}, err
		}
	} else {
		configMap = map[string]any{}
	}

	if r.cfg.MockMode {
		resolved.ProviderID = "mock"
		resolved.GoAdapter = "mock"
		resolved.ConfigJSON = configMap
		platform, err := registry.NewPlatform("mock", "mock", configMap, nil)
		if err != nil {
			return Resolved{}, err
		}
		out, err := platform.Execute()
		if err != nil {
			return Resolved{}, err
		}
		resolved.Output = out
		return resolved, nil
	}

	if binding == nil {
		if r.cfg.SingleTenantMode {
			envProvider := r.cfg.EnvProviderForDomain(domain)
			if envProvider != "" {
				goAdapter := registry.GoAdapterForProvider(envProvider)
				resolved.ProviderID = envProvider
				resolved.GoAdapter = goAdapter
				resolved.ConfigJSON = map[string]any{}
				platform, err := registry.NewPlatform(goAdapter, envProvider, map[string]any{}, nil)
				if err != nil {
					return Resolved{}, err
				}
				out, err := platform.Execute()
				if err != nil {
					return Resolved{}, err
				}
				resolved.Output = out
				return resolved, nil
			}
		}
		return Resolved{}, fmt.Errorf("resolve: no provider binding for tenant_id=%s domain=%s trace_id=%s", tenantID, domain, traceID)
	}

	providerID := binding.ProviderID
	goAdapter := registry.GoAdapterForProvider(providerID)
	secretKey := SecretCacheKey(tenantID, providerID, binding.UpdatedAt.UTC().Format(time.RFC3339Nano))
	secrets, secretHit := r.secretCache.Get(secretKey)
	resolved.CacheHitSecret = secretHit
	if !secretHit && r.secrets != nil && r.decryptor != nil {
		loaded, loadErr := r.secrets.GetSecrets(tenantID, providerID)
		if loadErr != nil {
			return Resolved{}, loadErr
		}
		secrets = loaded
		r.secretCache.Set(secretKey, secrets)
	}

	resolved.ProviderID = providerID
	resolved.GoAdapter = goAdapter
	resolved.ConfigJSON = configMap
	platform, err := registry.NewPlatform(goAdapter, providerID, configMap, secrets)
	if err != nil {
		return Resolved{}, err
	}
	out, err := platform.Execute()
	if err != nil {
		return Resolved{}, err
	}
	resolved.Output = out
	return resolved, nil
}

func (r *Resolver) loadBinding(tenantID, domain string) (*Binding, bool, error) {
	cached, hit, err := r.cache.Get(tenantID, domain)
	if err != nil {
		return nil, false, err
	}
	if hit {
		return cached, true, nil
	}
	binding, err := r.store.GetActiveBinding(tenantID, domain)
	if err != nil {
		return nil, false, err
	}
	if binding != nil {
		if setErr := r.cache.Set(tenantID, domain, binding); setErr != nil {
			return binding, false, setErr
		}
	}
	return binding, false, nil
}

// PostgresSecretStore loads encrypted credentials.
type PostgresSecretStore struct {
	db        *sql.DB
	decryptor *crypto.Decryptor
}

// NewPostgresSecretStore creates a secret store using the same pool as bindings.
func NewPostgresSecretStore(db *sql.DB, decryptor *crypto.Decryptor) *PostgresSecretStore {
	return &PostgresSecretStore{db: db, decryptor: decryptor}
}

// GetSecrets decrypts platform credentials for a tenant.
func (s *PostgresSecretStore) GetSecrets(tenantID, platform string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("GetSecrets: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant', $1, true)`, tenantID); err != nil {
		return nil, fmt.Errorf("GetSecrets: set tenant: %w", err)
	}

	var blob []byte
	err = tx.QueryRowContext(ctx, `
		SELECT credential_blob FROM platform_credentials
		WHERE tenant_id = $1::uuid AND platform = $2
		LIMIT 1
	`, tenantID, platform).Scan(&blob)
	if err == sql.ErrNoRows {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetSecrets: query: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if s.decryptor == nil {
		return map[string]string{}, nil
	}
	plain, err := s.decryptor.Decrypt(blob)
	if err != nil {
		return nil, err
	}
	var secrets map[string]string
	if err := json.Unmarshal(plain, &secrets); err != nil {
		return nil, fmt.Errorf("GetSecrets: unmarshal secrets json: %w", err)
	}
	return secrets, nil
}

// DB exposes the underlying sql.DB from PostgresBindingStore for secret store reuse.
func (s *PostgresBindingStore) DB() *sql.DB {
	return s.db
}
