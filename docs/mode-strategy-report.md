# Calyx Mode Strategy — Per-Component Flags

## Step 1 — Current mode control (evidence)

### Global `MOCK_MODE` (single switch)

Multiple services parse `MOCK_MODE` as a global boolean (default `true`):

| Service | File | Pattern |
|---------|------|---------|
| scoring | `services/scoring/internal/config/config.go` | `parseBoolDefault(os.Getenv("MOCK_MODE"), true)` |
| compliance-intelligence | `services/compliance-intelligence/internal/config/config.go` | same |
| ask-ai | `services/ask-ai/internal/config/config.go` | same |
| enrichment | `services/enrichment/internal/config/config.go` | same |
| ad-attribution | `services/ad-attribution/internal/config/config.go` | same |
| platform-sync | `services/platform-sync/internal/config/config.go` | `MockMode` from `MOCK_MODE` |

Provider resolution uses `resolveProvider(..., mockMode, ...)` — when env provider is unset and `mockMode == true`, provider becomes `"mock"`.

`providerconfig` enforces global mock:

- `services/providerconfig/internal/loader/resolver.go`: if `r.cfg.MockMode { resolved.ProviderID = "mock"; ... return }`

API integration test path (before this slice):

- `services/api/src/integrations/integrations.service.ts`: `const mockMode = process.env.MOCK_MODE !== 'false'` skipped real `test-auth` when global mock was on.

### Platform sync was GLOBAL

Before migration `000021`:

- `adapter.NewRegistry(mockMode bool)` stored global `mockMode`
- `Get(platform)` returned mock adapter when `r.mockMode` was true
- `integrations` table had no per-row `sync_mode` — only `connection_mode`, `sync_cursor`, `sync_status`, etc.

**Classification:** `MOCK_MODE` is a global switch. Platform-sync had no per-integration real/mock mode.

---

## Step 3 — Implementation (this slice)

### Database

- Migration `000021_api_sync_sync_mode.up.sql`:
  - `integrations.sync_mode TEXT NOT NULL DEFAULT 'mock'`
  - Check constraint: `sync_mode IN ('mock', 'real')`
  - `list_api_sync_integrations()` returns `sync_mode`

### API (`services/api`)

- `IntegrationRecord.sync_mode` exposed for `api_sync` rows (`null` for webhook)
- Create accepts optional `sync_mode` (default `mock` for API-key integrations)
- `POST /integrations/:id/test` uses **per-integration** `sync_mode`:
  - `mock` → ok without calling platform-sync `test-auth`
  - `real` → calls `platform-sync /test-auth` (fail-fast on bad creds)
- `PATCH /integrations/:id/sync-mode` updates `sync_mode` for API-key rows

### platform-sync

- `adapter.Registry.Get(platform, syncMode)` — mock unless `syncMode == "real"`
- `IntegrationStore` loads `sync_mode` per row; defaults empty to `mock`
- `syncer` passes `item.SyncMode` to `TestAuth` and `FetchAllPages`
- Process-level `cfg.MockMode` no longer selects adapter (registry is mode-agnostic)

### Web UI

- **Add integration form:** Test / Live radio when connection type is API key (default Test)
- **Integrations list:** Test / Live toggle on API-key rows wired to `PATCH /integrations/:id/sync-mode`

### Unchanged (by design)

- Global `MOCK_MODE` still controls scoring, ask-ai, enrichment, compliance, attribution, providerconfig
- No global auto-flip — default everywhere remains `mock`

---

## Verification (2026-07-08)

```text
# Builds
go test ./...          # platform-sync — ok (adapter package)
go build ./...         # platform-sync — PASS
npm run build          # web — PASS
npm run build          # api — PASS

# Migration
docker compose -f deploy/docker-compose.yml run --rm migrate
# 21/u api_sync_sync_mode, 22/u api_sync_get_integration

# Per-integration sync_mode API + verify-api-sync
verify-api-sync.ps1 — PASS (test + sync + ping +1 after 000022 RLS fix)
```

---

## Result

| Area | Before | After |
|------|--------|-------|
| Platform sync adapter | Global `MOCK_MODE` | Per-row `integrations.sync_mode` |
| Integration test | Global `MOCK_MODE` | Per-row `sync_mode` |
| UI | No mode control | Test/Live toggle on API-key rows |
| Default | mock | mock (unchanged) |
