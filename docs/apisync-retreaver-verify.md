# Retreaver REST Adapter — Verification Report

**Date:** 2026-07-07  
**Scope:** Confirm real API pull adapter exists (not credential fields only).  
**Environment:** Windows; `MOCK_MODE=true` for sync verify (CHECK 9); no live Retreaver API key exercised.

---

## CHECK 1 — Adapter file exists + implements shared interface

### Command: adapter file listing
```
PS> Get-ChildItem -Path "services\platform-sync" -Recurse -File | Where-Object { $_.Name -match 'retreaver|adapter' }

services\platform-sync\internal\adapter\callgrid_adapter.go
services\platform-sync\internal\adapter\retreaver_adapter.go
services\platform-sync\internal\adapter\retreaver_adapter_test.go
services\platform-sync\internal\adapter\ringba_adapter.go
services\platform-sync\internal\adapter\trackdrive_adapter.go
```

### Command: implements interface (non-mock)
```
PS> rg -n "func.*Retreaver|FetchCalls|MapToPing|Authenticate|TestAuth" services/platform-sync/ | rg -iv mock

services/platform-sync/cmd\server\main.go:77:		handleTestAuth(w, r, logger, svc)
services/platform-sync/cmd\server\main.go:135:func handleTestAuth(w http.ResponseWriter, r *http.Request, logger *slog.Logger, svc *syncer.Service) {
services/platform-sync/cmd\server\main.go:147:	if err := svc.TestAuth(ctx, req.TenantID, req.IntegrationID); err != nil {
services/platform-sync/internal\adapter\retreaver_adapter.go:22:func NewRetreaverAdapter(http *HTTPClient) *RetreaverAdapter {
services/platform-sync/internal\adapter\retreaver_adapter.go:26:func (a *RetreaverAdapter) Platform() string { return "retreaver" }
services/platform-sync/internal\adapter\retreaver_adapter.go:33:func (a *RetreaverAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
services/platform-sync/internal\adapter\retreaver_adapter.go:38:func (a *RetreaverAdapter) FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string) (PageResult, error) {
services/platform-sync/internal\adapter\retreaver_adapter.go:71:func (a *RetreaverAdapter) fetchCallsWithPerPage(ctx context.Context, creds map[string]string, cs retreaverCursor, perPage int) ([]models.RemoteCall, bool, error) {
services/platform-sync/internal\adapter\retreaver_adapter.go:114:func mapRetreaverCall(raw json.RawMessage) models.RemoteCall {
services/platform-sync/internal\adapter\retreaver_adapter_test.go:8:func TestMapRetreaverCall(t *testing.T) {
services/platform-sync/internal\adapter\retreaver_adapter_test.go:29:func TestRetreaverCursorRoundTrip(t *testing.T) {
services/platform-sync/internal\adapter\callgrid_adapter.go:34:func (a *CallGridAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
services/platform-sync/internal\adapter\ringba_adapter.go:35:func (a *RingbaAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
services/platform-sync/internal\adapter\registry.go:44:func (r *Registry) TestAuth(ctx context.Context, platform string, creds map[string]string) error {
services/platform-sync/internal\adapter\registry.go:49:	return ad.TestAuth(ctx, creds)
services/platform-sync/internal\adapter\trackdrive_adapter.go:32:func (a *TrackDriveAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
services/platform-sync/internal\syncer\syncer.go:62:func (s *Service) TestAuth(ctx context.Context, tenantID, integrationID string) error {
services/platform-sync/internal\syncer\syncer.go:71:	return s.adapter.TestAuth(ctx, item.Platform, creds)
services/platform-sync/internal\adapter\types.go:12:	TestAuth(ctx context.Context, creds map[string]string) error
```

**Note:** Shared `PlatformAdapter` interface uses `TestAuth` / `FetchPage` / `mapRetreaverCall` (plan names `Authenticate` / `FetchCalls` / `MapToPing` map to these). Real `RetreaverAdapter` in `retreaver_adapter.go` — not mock-only.

**Result: PASS**

---

## CHECK 2 — Real Retreaver API endpoint + auth wired

### Command
```
PS> rg -n "api\.retreaver\.com|calls\.json|api_key|company_id|created_at_start" services/platform-sync/

services/platform-sync/internal\adapter\ringba_adapter.go:81:		token = creds["api_key"]
services/platform-sync/internal\adapter\callgrid_adapter.go:71:	apiKey := creds["api_key"]
services/platform-sync/internal\adapter\callgrid_adapter.go:73:		return nil, nil, false, fmt.Errorf("callgrid: api_key required")
services/platform-sync/internal\adapter\retreaver_adapter.go:16:const retreaverBaseURL = "https://api.retreaver.com/api/v2/calls.json"
services/platform-sync/internal\adapter\retreaver_adapter.go:29:	CreatedAtStart string `json:"created_at_start"`
services/platform-sync/internal\adapter\retreaver_adapter.go:72:	apiKey := creds["api_key"]
services/platform-sync/internal\adapter\retreaver_adapter.go:73:	companyID := creds["company_id"]
services/platform-sync/internal\adapter\retreaver_adapter.go:75:		return nil, false, fmt.Errorf("retreaver: api_key and company_id required")
services/platform-sync/internal\adapter\retreaver_adapter.go:78:	q.Set("api_key", apiKey)
services/platform-sync/internal\adapter\retreaver_adapter.go:79:	q.Set("company_id", companyID)
services/platform-sync/internal\adapter\retreaver_adapter.go:80:	q.Set("created_at_start", cs.CreatedAtStart)
```

**Note:** `fetchCallsWithPerPage` issues `GET` via shared `HTTPClient.Do` — real HTTP, not stub data (when `MOCK_MODE=false`).

**Result: PASS**

---

## CHECK 3 — Field mapping (Retreaver JSON → internal Ping)

### Command
```
PS> rg -n "uuid|caller_state|revenue|payout|campaign_id|client_afid|NormalizeState" services/platform-sync/ | rg -i retreaver

services/platform-sync/internal\adapter\retreaver_adapter.go:116:		UUID        string   `json:"uuid"`
services/platform-sync/internal\adapter\retreaver_adapter.go:118:		CallerState string   `json:"caller_state"`
services/platform-sync/internal\adapter\retreaver_adapter.go:119:		Revenue     *float64 `json:"revenue"`
services/platform-sync/internal\adapter\retreaver_adapter.go:120:		Payout      *float64 `json:"payout"`
services/platform-sync/internal\adapter\retreaver_adapter.go:121:		CampaignID  int      `json:"campaign_id"`
services/platform-sync/internal\adapter\retreaver_adapter.go:122:		ClientAFID  string   `json:"client_afid"`
services/platform-sync/internal\adapter\retreaver_adapter.go:144:		State:          state.NormalizeState(row.CallerState),
services/platform-sync/internal\adapter\retreaver_adapter_test.go:10:		"uuid": "f1abfb78-ab8a-4146-9946-8169fbcc6d6c",
services/platform-sync/internal\adapter\retreaver_adapter_test.go:12:		"caller_state": "NC",
services/platform-sync/internal\adapter\retreaver_adapter_test.go:13:		"revenue": 5.0,
services/platform-sync/internal\adapter\retreaver_adapter_test.go:14:		"payout": 3.0,
services/platform-sync/internal\adapter\retreaver_adapter_test.go:15:		"campaign_id": 9872,
services/platform-sync/internal\adapter\retreaver_adapter_test.go:16:		"client_afid": "aff123",
services/platform-sync/internal\adapter\retreaver_adapter_test.go:22:		t.Fatalf("uuid: %s", call.ExternalCallID)
```

**Mapping:** `uuid` → `ExternalCallID`, `caller_state` → `State` via `NormalizeState`, `revenue`/`payout`, `campaign_id` → `CampaignID`, `client_afid` → `PublisherID`. No `call_key` in adapter (API pull — expected).

**Result: PASS**

---

## CHECK 4 — Shared downstream reused (Kafka / dedup / cursor)

### Command: pipeline grep (first 30 non-test lines)
```
PS> rg -n "kafka|Publish|producer|dedup|cursor|sync_status" services/platform-sync/ | rg -iv test | Select-Object -First 30

services/platform-sync/pkg\kafka\kafka.go:1:package kafka
services/platform-sync/pkg\kafka\kafka.go:8:	"github.com/segmentio/kafka-go"
services/platform-sync/pkg\kafka\kafka.go:12:	writer *kafka.Writer
services/platform-sync/pkg\kafka\kafka.go:23:		writer: &kafka.Writer{
services/platform-sync/pkg\kafka\kafka.go:24:			Addr:         kafka.TCP(brokers...),
services/platform-sync/pkg\kafka\kafka.go:25:			Balancer:     &kafka.Hash{},
services/platform-sync/pkg\kafka\kafka.go:26:			RequiredAcks: kafka.RequireOne,
services/platform-sync/pkg\kafka\kafka.go:34:	err := c.writer.WriteMessages(ctx, kafka.Message{
services/platform-sync/pkg\kafka\kafka.go:40:		return fmt.Errorf("kafka.Write: %w", err)
services/platform-sync/pkg\kafka\kafka.go:46:	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
services/platform-sync/pkg\kafka\kafka.go:49:		return fmt.Errorf("kafka.Ping: %w", err)
services/platform-sync/cmd\server\main.go:16:	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"
services/platform-sync/cmd\server\main.go:17:	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer"
services/platform-sync/cmd\server\main.go:20:	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/pkg/kafka"
services/platform-sync/cmd\server\main.go:59:	kafkaClient := kafka.NewClient(cfg.KafkaBrokers, cfg.KafkaTopic)
services/platform-sync/cmd\server\main.go:60:	defer kafkaClient.Close()
services/platform-sync/cmd\server\main.go:64:	prod := producer.NewProducer(kafkaClient, cfg.KafkaTopic, cfg.ANIEncryptionKey)
services/platform-sync/cmd\server\main.go:65:	dedupChecker := dedup.NewChecker(redisClient, 24*time.Hour)
services/platform-sync/cmd\server\main.go:66:	svc := syncer.NewService(st, ad, prod, dedupChecker, decryptor, logger)
services/platform-sync/go.mod:8:	github.com/segmentio/kafka-go v0.4.47
services/platform-sync/go.sum:28:github.com/segmentio/kafka-go v0.4.47 h1:IqziR4pA3vrZq7YdRxaT3w1/5fvIH5qpCwstUanQQB0=
services/platform-sync/go.sum:29:github.com/segmentio/kafka-go v0.4.47/go.mod h1:HjF6XbOKh0Pjlkr5GVZxt6CsjjwnmhVOfURM5KMd8qg=
```

### Additional: sync_status / cursor / Publish / dedup
```
services/platform-sync/internal\dedup\dedup.go:21:	key := fmt.Sprintf("%s:dedup:%s:%s", tenantID, platform, externalCallID)
services/platform-sync/internal\producer\producer.go:25:func (p *Producer) Publish(ctx context.Context, tenantID, platform string, call models.RemoteCall, isDuplicate bool, receivedAt int64) error {
services/platform-sync/internal\syncer\syncer.go:97:		isDup, err := s.dedup.IsDuplicate(ctx, item.TenantID, item.Platform, call.ExternalCallID)
services/platform-sync/internal\syncer\syncer.go:203:		if err := s.producer.Publish(ctx, item.TenantID, item.Platform, call, isDup, now.UnixMilli()); err != nil {
services/platform-sync/internal\store\integrations.go:39:		SELECT id, tenant_id, platform, name, api_credentials_blob, sync_cursor,
services/platform-sync/internal\store\integrations.go:40:		       last_sync_at, sync_status, last_sync_error
```

### Migration 000020
```
PS> ls db/postgres/migrations/ | grep 000020

000020_api_sync_status.down.sql
000020_api_sync_status.up.sql
```

**Result: PASS**

---

## CHECK 5 — Test connection is REAL (validates key)

### Command
```
PS> rg -n "TestAuth|testConnection|test.*retreaver|/test" services/platform-sync/ services/api/src/integrations/ | rg -iv mock

services/api/src/integrations/integrations.controller.ts:60:  @Post(':id/test')
services/api/src/integrations/integrations.controller.ts:65:    return this.integrationsService.testConnection(tenantId, id);
services/api/src/integrations/integrations.service.ts:145:  async testConnection(
services/api/src/integrations/integrations.service.ts:175:    const res = await fetch(`${syncUrl}/test-auth`, {
services/platform-sync/cmd\server\main.go:76:	mux.HandleFunc("POST /test-auth", func(w http.ResponseWriter, r *http.Request) {
services/platform-sync/cmd\server\main.go:77:		handleTestAuth(w, r, logger, svc)
services/platform-sync/cmd\server\main.go:135:func handleTestAuth(w http.ResponseWriter, r *http.Request, logger *slog.Logger, svc *syncer.Service) {
services/platform-sync/cmd\server\main.go:147:	if err := svc.TestAuth(ctx, req.TenantID, req.IntegrationID); err != nil {
services/platform-sync/internal\adapter\retreaver_adapter.go:33:func (a *RetreaverAdapter) TestAuth(ctx context.Context, creds map[string]string) error {
services/platform-sync/internal\syncer\syncer.go:62:func (s *Service) TestAuth(ctx context.Context, tenantID, integrationID string) error {
services/platform-sync/internal\syncer\syncer.go:71:	return s.adapter.TestAuth(ctx, item.Platform, creds)
```

**Behavior:**
- `MOCK_MODE=false`: `POST /integrations/:id/test` → API `testConnection` → `platform-sync POST /test-auth` → `RetreaverAdapter.TestAuth` (lightweight `GET calls.json` with `per_page=1`).
- `MOCK_MODE=true`: API returns stored-credentials message without live call (by design; see CHECK 6).

**Result: PASS** (real path implemented; mock bypass only when `MOCK_MODE=true`)

---

## CHECK 6 — Mock intact; real only on MOCK_MODE=false

### Command
```
PS> rg -n "MOCK_MODE|mockMode|isMock" services/platform-sync/ | rg -iv test

services/platform-sync/internal\adapter\registry.go:12:	mockMode bool
services/platform-sync/internal\adapter\registry.go:18:func NewRegistry(mockMode bool) *Registry {
services/platform-sync/internal\adapter\registry.go:21:		mockMode: mockMode,
services/platform-sync/internal\adapter\registry.go:34:	if r.mockMode {
services/platform-sync/internal\config\config.go:48:	mockMode := true
services/platform-sync/internal\config\config.go:49:	if v := os.Getenv("MOCK_MODE"); v == "false" || v == "0" {
services/platform-sync/internal\config\config.go:50:		mockMode = false
services/platform-sync/internal\config\config.go:61:		MockMode:                mockMode,
```

**Behavior:** `Registry.Get()` returns `mockAdapter` when `mockMode=true`; otherwise `RetreaverAdapter`. Missing `api_key`/`company_id` → `fmt.Errorf("retreaver: api_key and company_id required")` (fail-fast, no silent mock).

**Result: PASS**

---

## CHECK 7 — Errors / rate limits / no secret logging

### Command: retreaver error handling
```
PS> rg -n "backoff|429|rate|retry|error" services/platform-sync/ | rg -i retreaver

services/platform-sync/internal\adapter\retreaver_adapter.go:33:func (a *RetreaverAdapter) TestAuth(...)
services/platform-sync/internal\adapter\retreaver_adapter.go:38:func (a *RetreaverAdapter) FetchPage(...)
services/platform-sync/internal\adapter\retreaver_adapter.go:71:func (a *RetreaverAdapter) fetchCallsWithPerPage(...)
services/platform-sync/internal\adapter\retreaver_adapter.go:154:func checkHTTPStatus(status int, body []byte, platform string) error {
```

### Shared HTTP client (used by RetreaverAdapter) — 429 retry + backoff
```
services/platform-sync/internal\adapter\httpclient.go:15:// HTTPClient wraps http.Client with per-host circuit breaker and 429 retry.
services/platform-sync/internal\adapter\httpclient.go:40:// Do executes an HTTP request with circuit breaker and limited 429 retry.
services/platform-sync/internal\adapter\httpclient.go:44:	for attempt := 0; attempt < 3; attempt++ {
services/platform-sync/internal\adapter\httpclient.go:46:			time.Sleep(time.Duration(attempt*attempt) * 500 * time.Millisecond)
services/platform-sync/internal\adapter\httpclient.go:58:			if resp.StatusCode == http.StatusTooManyRequests {
services/platform-sync/internal\adapter\httpclient.go:59:				return nil, fmt.Errorf("rate limited (429)")
```

### Logging grep (no credential values logged from retreaver adapter)
```
PS> rg -n "api_key|apiKey|log" services/platform-sync/ | rg -i "retreaver|log" | Select-Object -First 15

services/platform-sync/cmd\server\main.go:6:	"log/slog"
services/platform-sync/cmd\server\main.go:31:	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
... (structured logs on integration_id/platform/error only; no api_key in retreaver_adapter.go)
```

**Note:** `retreaver_adapter.go` has zero `log`/`slog` calls; credentials only passed as query params, never logged.

**Result: PASS**

---

## CHECK 8 — Builds + tests

### Command
```
PS> cd services/platform-sync; go build ./...; go test ./...

?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/cmd/server	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/config	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/state	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/syncer	[no test files]
?   	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/pkg/kafka	[no test files]
ok  	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter	(cached)
```

**Exit code:** 0

**Result: PASS**

---

## CHECK 9 — Actual pull proof (mock mode OK)

### Command: verify-api-sync (make unavailable; ran script directly)
```
PS> powershell -ExecutionPolicy Bypass -File scripts/verify-api-sync.ps1

=== API Sync verify ===
PASS: platform-sync healthz
Before: analytics.total_pings=374
Created integration b2abfd5f-ca3c-40bb-9a9e-0582b3c87e77 mode=api_sync
PASS: test connection - MOCK_MODE: API key stored for retreaver. Sync worker will pull demo calls.
Sync triggered: sync_triggered
PASS: analytics.total_pings increased (374 -> 375)
PASS: found apisync ping apisync-retreaver-20260707-87b28ef991e79e30 platform=retreaver
=== API Sync verify complete ===
```

### ClickHouse row count
```
PS> docker exec deploy-clickhouse-1 clickhouse-client --query "SELECT count(), min(received_at), max(received_at) FROM pings WHERE external_call_id LIKE 'apisync-retreaver-%'"

130	2026-07-06 13:36:29	2026-07-07 12:34:11
```

**Result: PASS** (pull → map → Kafka → ClickHouse path runs; rows are demo `apisync-retreaver-*` under `MOCK_MODE=true`)

---

## Summary

| Check | Result | Note |
|-------|--------|------|
| 1 adapter file + interface | **PASS** | `retreaver_adapter.go` implements `PlatformAdapter` (`TestAuth`, `FetchPage`, `mapRetreaverCall`) |
| 2 real endpoint + auth | **PASS** | `https://api.retreaver.com/api/v2/calls.json`; query `api_key`, `company_id`, `created_at_start` |
| 3 field mapping + NormalizeState | **PASS** | uuid, caller_state, revenue, payout, campaign_id, client_afid; no `call_key` |
| 4 shared Kafka/dedup/cursor | **PASS** | Same producer/dedup/syncer; migration `000020` adds `sync_status` / `last_sync_error` |
| 5 real test connection | **PASS** | Live `TestAuth` when `MOCK_MODE=false`; API `POST :id/test` → `/test-auth` |
| 6 mock intact + fail-fast | **PASS** | `Registry.Get()` switches on `MOCK_MODE`; missing creds error, no silent mock |
| 7 errors/limits/no key logging | **PASS** | Shared `HTTPClient` 429 retry + circuit breaker; no credential logging in adapter |
| 8 build + tests | **PASS** | `go build ./...` and `go test ./...` exit 0; adapter unit tests pass |
| 9 pull path runs | **PASS** | Sync verify + 130 `apisync-retreaver-%` rows in ClickHouse |

---

**Retreaver REAL pull adapter implemented: YES**

The real HTTP client, endpoint wiring, field mapping, test-auth, and shared sync pipeline are all present in `services/platform-sync/internal/adapter/retreaver_adapter.go`. This verification used `MOCK_MODE=true` for end-to-end sync proof (demo rows). A live pull against `api.retreaver.com` requires `MOCK_MODE=false` plus valid Retreaver credentials (`scripts/verify-apisync-retreaver-real.ps1`).
