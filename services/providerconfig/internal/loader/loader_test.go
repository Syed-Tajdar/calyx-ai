package loader

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/catalog"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/config"
)

type memBindingStore struct {
	data map[string]*Binding
	err  error
}

func (m *memBindingStore) GetActiveBinding(tenantID, domain string) (*Binding, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := tenantID + ":" + domain
	return m.data[key], nil
}

type memBindingCache struct {
	data map[string]*Binding
}

func (m *memBindingCache) Get(tenantID, domain string) (*Binding, bool, error) {
	b, ok := m.data[tenantID+":"+domain]
	return b, ok, nil
}

func (m *memBindingCache) Set(tenantID, domain string, binding *Binding) error {
	if m.data == nil {
		m.data = map[string]*Binding{}
	}
	m.data[tenantID+":"+domain] = binding
	return nil
}

func (m *memBindingCache) Delete(tenantID, domain string) error {
	delete(m.data, tenantID+":"+domain)
	return nil
}

func testCatalog() catalog.Catalog {
	return catalog.Catalog{
		Version: "1",
		Providers: []catalog.Provider{
			{ID: "mock", Domain: "attribution", GoAdapter: "mock"},
		},
	}
}

func bindingA() *Binding {
	cfg, _ := json.Marshal(map[string]any{"variant": "tenant_a", "label": "A"})
	return &Binding{
		TenantID:   "11111111-1111-1111-1111-111111111111",
		Domain:     "attribution",
		ProviderID: "mock",
		ConfigJSON: cfg,
		UpdatedAt:  time.Now().UTC(),
	}
}

func bindingB() *Binding {
	cfg, _ := json.Marshal(map[string]any{"variant": "tenant_b", "label": "B"})
	return &Binding{
		TenantID:   "22222222-2222-2222-2222-222222222222",
		Domain:     "attribution",
		ProviderID: "mock",
		ConfigJSON: cfg,
		UpdatedAt:  time.Now().UTC(),
	}
}

func newTestResolver(mockMode bool, store BindingStore, cache BindingCache) *Resolver {
	cfg := config.Config{MockMode: mockMode, SingleTenantMode: false, AttributionProvider: "mock"}
	return NewResolver(cfg, testCatalog(), store, cache, nil, NewLRUSecretCache(), nil)
}

// Test 1: catalog validates against registry at boot (see catalog package).

// Test 3: MOCK_MODE=true — tenants get different config_json on mock adapter.
func TestResolveMockModeDistinctConfig(t *testing.T) {
	store := &memBindingStore{data: map[string]*Binding{
		"11111111-1111-1111-1111-111111111111:attribution": bindingA(),
		"22222222-2222-2222-2222-222222222222:attribution": bindingB(),
	}}
	cache := &memBindingCache{data: map[string]*Binding{}}
	r := newTestResolver(true, store, cache)

	resA, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if resA.ProviderID != "mock" || resA.Output["label"] != "A" {
		t.Fatalf("tenant A: got %+v", resA)
	}

	resB, err := r.Resolve("22222222-2222-2222-2222-222222222222", "attribution", "t2")
	if err != nil {
		t.Fatal(err)
	}
	if resB.Output["label"] != "B" {
		t.Fatalf("tenant B: got %+v", resB)
	}
}

// Test 4: second resolve hits Redis cache.
func TestResolveUsesBindingCache(t *testing.T) {
	store := &memBindingStore{data: map[string]*Binding{
		"11111111-1111-1111-1111-111111111111:attribution": bindingA(),
	}}
	cache := &memBindingCache{data: map[string]*Binding{}}
	r := newTestResolver(true, store, cache)

	res1, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if res1.CacheHitBinding {
		t.Fatal("expected cache miss on first resolve")
	}

	res2, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t2")
	if err != nil {
		t.Fatal(err)
	}
	if !res2.CacheHitBinding {
		t.Fatal("expected cache hit on second resolve")
	}
}

// Test 5: cache invalidation after delete.
func TestCacheInvalidation(t *testing.T) {
	b := bindingA()
	store := &memBindingStore{data: map[string]*Binding{
		"11111111-1111-1111-1111-111111111111:attribution": b,
	}}
	cache := &memBindingCache{data: map[string]*Binding{}}
	r := newTestResolver(true, store, cache)

	_, _ = r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t1")
	_ = cache.Delete("11111111-1111-1111-1111-111111111111", "attribution")

	updated := bindingA()
	updated.ConfigJSON, _ = json.Marshal(map[string]any{"variant": "tenant_a", "label": "A-updated"})
	store.data["11111111-1111-1111-1111-111111111111:attribution"] = updated

	res, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t2")
	if err != nil {
		t.Fatal(err)
	}
	if res.Output["label"] != "A-updated" {
		t.Fatalf("expected updated label, got %+v", res.Output)
	}
	if res.CacheHitBinding {
		t.Fatal("expected cache miss after invalidation")
	}
}

// Test 6: MOCK_MODE=false, no binding → fail loud.
func TestResolveFailsLoudWithoutBinding(t *testing.T) {
	store := &memBindingStore{data: map[string]*Binding{}}
	cache := &memBindingCache{data: map[string]*Binding{}}
	r := newTestResolver(false, store, cache)

	_, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t1")
	if err == nil {
		t.Fatal("expected error when no binding in real mode")
	}
}

// Test 7: MOCK_MODE=true, no binding → mock empty config (no fail).
func TestResolveMockModeNoBindingEmptyConfig(t *testing.T) {
	store := &memBindingStore{data: map[string]*Binding{}}
	cache := &memBindingCache{data: map[string]*Binding{}}
	r := newTestResolver(true, store, cache)

	res, err := r.Resolve("99999999-9999-9999-9999-999999999999", "attribution", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ProviderID != "mock" {
		t.Fatalf("expected mock provider, got %s", res.ProviderID)
	}
	if len(res.ConfigJSON) != 0 {
		t.Fatalf("expected empty config, got %+v", res.ConfigJSON)
	}
}

// Test SINGLE_TENANT_MODE env fallback.
func TestResolveSingleTenantEnvFallback(t *testing.T) {
	store := &memBindingStore{data: map[string]*Binding{}}
	cache := &memBindingCache{data: map[string]*Binding{}}
	cfg := config.Config{
		MockMode:            false,
		SingleTenantMode:    true,
		AttributionProvider: "mock",
	}
	r := NewResolver(cfg, testCatalog(), store, cache, nil, NewLRUSecretCache(), nil)

	res, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if res.ProviderID != "mock" {
		t.Fatalf("expected env fallback mock, got %s", res.ProviderID)
	}
}

func TestResolveStoreError(t *testing.T) {
	store := &memBindingStore{err: errors.New("db down")}
	cache := &memBindingCache{data: map[string]*Binding{}}
	r := newTestResolver(true, store, cache)
	_, err := r.Resolve("11111111-1111-1111-1111-111111111111", "attribution", "t1")
	if err == nil {
		t.Fatal("expected store error")
	}
}
