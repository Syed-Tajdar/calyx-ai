# Calyx — Real-Testing Readiness Verification Report

**Date:** 2026-07-03  
**Scope:** Pre–`MOCK_MODE=false` go/no-go for real Ringba/Deepgram integration and live dashboard + ping feed + coverage.  
**Environment checked:** Local codebase + `node scripts/ws-smoke-test.mjs` + `go test ./internal/state/...` (stack assumed running for WS test).

---

## Executive summary

| Area | Status | Blocker for first real ping test? |
|------|--------|----------------------------------|
| 1 Mock→real switch holds | **PARTIAL** | Yes — several providers are skeleton/stub when `MOCK_MODE=false` |
| 2 Ingestion real-ready (auth, state, call_key) | **PARTIAL** | Yes — `call_key` plaintext at rest; otherwise ingestion path is solid |
| 3 Live surfaces (dashboard/pings live; coverage plan) | **PARTIAL** | No for first test — dashboard WS works in mock; coverage refresh-on-load OK |
| 4 Data integrity (aggregates, tenant, cost, dedup) | **PARTIAL** | Partial — tenant OK; dedup flags not excluded in analytics/coverage aggregates |
| 5 Operational (start, secrets, health) | **PASS** | No — dev stack boots; secrets pattern OK |

### Final verdict

**READY FOR REAL TESTING: NOT-YET**

**Narrow-scope exception:** A **controlled first real Ringba webhook test** (ingestion → Kafka → ClickHouse → `GET /pings` + `GET /coverage/heatmap` on page refresh) can proceed **after** fixing must-fix ingestion/security items below. Full-stack “flip `MOCK_MODE=false` with zero rework” for Deepgram, OpenAI scoring, DNC, and Meta attribution is **not** true today.

### Must-fix before `MOCK_MODE=false` / real customer traffic

1. **`call_key` encryption at rest** — still plaintext in ClickHouse (`services/ingestion/internal/models/ping.go` TODO).
2. **DNC provider silent mock fallback** — `compliancepoint` name still resolves to `MockProvider` (`services/compliance-intelligence/internal/dnc/provider.go:44-45`).
3. **OpenAI scoring not implemented** — `OpenAIProvider.Score` returns error (`services/scoring/internal/scoring/ai.go:52-54`).
4. **Deepgram transcription worker** — scoring service has config fail-fast for keys but **no real Deepgram transcription client** in repo; `MOCK_MODE=false` disables mock emitter — transcript pipeline may stall without new worker.
5. **Meta/Google attribution** — `errNotImplemented` (`services/ad-attribution/internal/platforms/mock.go`).
6. **Spend guards** — Ask AI has token budget; **scoring/Deepgram/OpenAI have no per-tenant spend cap**.
7. **Analytics/coverage dedup** — `is_duplicate=1` rows still included in `sum(revenue)` / `count()` (no `WHERE is_duplicate = 0`).

### Nice-to-have (fast follow, not blocking first Ringba ping test)

- Live coverage via polling hook (`useCoverageHeatmap`, ~zero API change).
- `state` field in WS `ping_received` schema + Kafka consumer mapping.
- Coverage heatmap `from`/`to` date params (dashboard filter parity).
- `make dev-up` from `docker compose down -v` documented as one-shot fresh bootstrap.

---

## SECTION 1 — Mock→real switch (core promise)

### 1.1 — Every external provider behind mode switch

**Command:**
```
rg -n "MOCK_MODE|mockMode|isMock" services/ --glob "*.ts" --glob "*.go" --glob "!*test*" | head -60
```

**Raw output (truncated — first 54 lines):**
```
services/scoring\internal\mockemitter\emitter.go:16:// Service consumes pings and emits mock transcript_ready events when MOCK_MODE is on.
services/ad-attribution\internal\config\config.go:36:	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)
services/ad-attribution\internal\config\config.go:71:		AttributionProvider: resolveProvider("ATTRIBUTION_PROVIDER", mockMode, "meta_capi"),
services/providerconfig\internal\config\config.go:40:		return Config{}, fmt.Errorf("config.Load: CREDENTIAL_ENCRYPTION_KEY required when MOCK_MODE=false")
services/enrichment\internal\config\config.go:71:		return fmt.Errorf("config.validate: ENROLLHERE_API_KEY required when MOCK_MODE=false and ENRICHMENT_PROVIDER=enrollhere")
services/scoring\internal\config\config.go:54:		TranscriptionProvider: resolveProvider("TRANSCRIPTION_PROVIDER", mockMode, "deepgram"),
services/scoring\internal\config\config.go:55:		AIProvider:            resolveProvider("AI_PROVIDER", mockMode, "openai"),
services/scoring\internal\config\config.go:75:			return fmt.Errorf("config.validate: DEEPGRAM_API_KEY and DEEPGRAM_CALLBACK_SECRET required when MOCK_MODE=false and TRANSCRIPTION_PROVIDER=deepgram")
services/ask-ai\internal\config\config.go:72:		return Config{}, fmt.Errorf("config.Load: OPENAI_API_KEY required when MOCK_MODE=false and LLM_PROVIDER=openai")
services/api\src\live\kafka-live-consumer.service.ts:23:    if (env.MOCK_MODE ?? true) { return; }
services/api\src\live\mock-live-emitter.service.ts:25:    if (!(env.MOCK_MODE ?? true)) { return; }
services/compliance-intelligence\internal\config\config.go:58:		DNCProvider: resolveProvider("DNC_PROVIDER", mockMode, "compliancepoint"),
... (scoring, enrichment, ask-ai, ingestion callloom_native stub, api env — all present)
```

**Verdict: PARTIAL / GAP**

| Provider | Mode switch in config? | Real implementation? |
|----------|------------------------|----------------------|
| Transcription (Deepgram) | Yes — `scoring/internal/config` | **GAP** — keys validated at boot; no Deepgram client/worker in scoring service |
| LLM / AI scoring (OpenAI) | Yes | **FAIL** — `OpenAIProvider.Score` returns `"not implemented in phase 2 skeleton"` |
| DNC (compliance) | Yes — `DNC_PROVIDER` | **FAIL** — `Registry("compliancepoint")` still returns `MockProvider` |
| Attribution (Meta) | Yes — `ATTRIBUTION_PROVIDER` | **FAIL** — `MetaCAPIPlatform.Fire` → `errNotImplemented` |
| Enrichment (EnrollHere/Whitepages) | Yes — fail-fast on keys | Config only — real HTTP adapters not verified in this pass |
| Ask AI (OpenAI) | Yes — fail-fast | Pipeline exists with `cost.BudgetGuard` |
| Ingestion webhooks | **Not MOCK-gated** (always real path) | Ringba/Retreaver/TrackDrive/CallGrid use DB secrets |
| Call Loom native | MOCK stub only | `callloom_native.go` returns stub when `MOCK_MODE=false` |

---

### 1.2 — Fail-fast on missing real secrets

**Command:**
```
rg -n "fail|fatal|panic|throw|required" services/ -i --glob "*.go" --glob "*.ts" | rg -i "key|secret|MOCK_MODE|provider" | head -40
```

**Raw output (sample):**
```
services/enrichment/internal/config/config.go:71: ENROLLHERE_API_KEY required when MOCK_MODE=false
services/enrichment/internal/config/config.go:74: WHITEPAGES_API_KEY required when MOCK_MODE=false
services/api/src/config/env.ts:54: DEEPGRAM_CALLBACK_SECRET required when MOCK_MODE=false
services/scoring/internal/config/config.go:75: DEEPGRAM_API_KEY and DEEPGRAM_CALLBACK_SECRET required when MOCK_MODE=false
services/scoring/internal/config/config.go:79: OPENAI_API_KEY required when MOCK_MODE=false and AI_PROVIDER=openai
services/ask-ai/internal/config/config.go:72: OPENAI_API_KEY required when MOCK_MODE=false
services/providerconfig/internal/config/config.go:40: CREDENTIAL_ENCRYPTION_KEY required when MOCK_MODE=false
services/ingestion/internal/config/config.go:33: ANI_ENCRYPTION_KEY is required (32-byte hex)
```

**Verdict: PARTIAL**

**Real fail-fast (boot):** scoring, enrichment, ask-ai, providerconfig, api (`DEEPGRAM_CALLBACK_SECRET`), ingestion (`ANI_ENCRYPTION_KEY`).

**Silent mock fallback (violates promise):**
- `compliance-intelligence/internal/dnc/provider.go` — non-`mock` names still return `NewMockProvider()`.
- `scoring/internal/scoring/ai.go` — `Registry("openai")` returns provider that errors at **runtime**, not boot (keys pass validate but Score fails).

---

### 1.3 — End-to-end vertical slice (code path)

**Verdict: PARTIAL**

```
Ringba webhook (HMAC + DB token)
  → services/ingestion (NormalizeState in buildPing)
  → Kafka (PingMessage includes state)
  → services/consumer → ClickHouse pings
  → services/api GET /pings, GET /coverage/heatmap, GET /analytics/*
  → web dashboard (hybrid feed) + coverage (fetch-on-mount)
```

| Step | Real-ready? | Notes |
|------|-------------|-------|
| HMAC + per-tenant secret | **PASS** | `IntegrationStore.LookupByToken` → `webhook_secret` from Postgres |
| State normalization | **PASS** | `usstate.NormalizeState` in `ping_builder.go` |
| Kafka publish | **PASS** | `producer.Publish` |
| ClickHouse insert | **PASS** | consumer batch writer includes `state`, `call_key` |
| API read paths | **PASS** | tenant-scoped queries |
| Dashboard live WS | **MOCK today** | `MockLiveEmitterService` when `MOCK_MODE=true`; `KafkaLiveConsumerService` when `false` |
| Scoring / transcript | **MOCK today** | `mockemitter` generates `transcript_ready`; no Deepgram worker |
| Compliance DNC | **MOCK today** | hash-based mock match |
| Attribution | **MOCK today** | `MockPlatform.Fire` logs success |

---

## SECTION 2 — Ingestion real-data ready

### 2.1 — Real HMAC/auth per platform

**Command:**
```
rg -n "VerifyRingbaHMAC|VerifyRetreaverHMAC|VerifyCallGridHMAC|Authenticate|webhook_secret" services/ingestion/ --glob "!*test*"
```

**Raw output:**
```
services/ingestion/internal/security/hmac.go:22:// VerifyRingbaHMAC
services/ingestion/internal/connectors/ringba_connector.go:16: Authenticate → VerifyRingbaHMAC
services/ingestion/internal/connectors/retreaver.go:20: Authenticate → VerifyRetreaverHMAC
services/ingestion/internal/connectors/callgrid.go:17: Authenticate → VerifyCallGridHMAC
services/ingestion/internal/connectors/trackdrive.go:17: Authenticate (platform-specific)
services/ingestion/internal/store/integrations.go:44: SELECT tenant_id, platform, webhook_secret FROM lookup_integration_by_token($1)
services/ingestion/internal/handlers/webhook.go:100: connector.Authenticate(r, body, integration.WebhookSecret)
services/ingestion/internal/handlers/ringba.go:85: VerifyRingbaHMAC(r, body, integration.WebhookSecret)
```

**Verdict: PASS**

Real secret swap = update `integrations` row / `webhook_secret` in Postgres (via admin flow). No hardcoded production secret in handler path. Dev seed tokens (e.g. `dev_ringba_token_change_me`) are fixtures only.

---

### 2.2 — State normalization live

**Command:**
```
cd services/ingestion && go test ./internal/state/... -v
```

**Raw output:**
```
=== RUN   TestNormalizeState
--- PASS: TestNormalizeState (0.00s)
    --- PASS: TestNormalizeState/Texas (0.00s)
    --- PASS: TestNormalizeState/tx (0.00s)
    --- PASS: TestNormalizeState/TX (0.00s)
    --- PASS: TestNormalizeState/ZZ (0.00s)
    --- PASS: TestNormalizeState/California (0.00s)
    --- PASS: TestNormalizeState/New_York (0.00s)
    ... (all cases PASS)
PASS ok github.com/.../internal/state 0.221s
```

**Wiring:** `services/ingestion/internal/connectors/ping_builder.go:49` — `State: usstate.NormalizeState(state)`.

**Verdict: PASS**

---

### 2.3 — call_key encryption before real Retreaver data

**Command:**
```
rg -n "call_key|CallKey|encrypt|AES" services/ingestion/ services/consumer/ --glob "!*test*" | head
```

**Findings:**
- `services/ingestion/internal/models/ping.go:23` — **TODO**: encrypt at rest before `MOCK_MODE=false`.
- `services/consumer/pkg/clickhouse/clickhouse.go` — inserts `call_key` column **plaintext**.
- ANI **is** encrypted (`crypto.EncryptANI`); `call_key` is **not**.

**Verdict: FAIL / OPEN BLOCKER** for real Retreaver traffic.

---

### 2.4 — Unknown/edge inputs

**Code review (`webhook.go`, `ringba.go`):**

| Input | Behavior |
|-------|----------|
| Unknown platform in path | `404 unknown platform` |
| Invalid / unknown token | `401 unauthorized` |
| Platform mismatch | `401 unauthorized` |
| Missing webhook secret | `401 unauthorized` |
| HMAC fail | `401` + payload hash logged (no ANI) |
| Malformed payload | `400 bad request` |
| Oversized body | `413 payload too large` |
| Dedup hit | Still published with `is_duplicate=true` (not dropped) |
| Missing/unknown state | `NormalizeState` → `""`; ping stored; excluded from coverage heatmap (`state != ''`) |

**Verdict: PASS** (no crash path found; missing state does not fail ingestion).

---

## SECTION 3 — Three live surfaces

### 3.1 — Dashboard + ping feed live

**Command:**
```
node scripts/ws-smoke-test.mjs
```

**Raw output:**
```
  event #1: ping_received
  event #2: ping_received
  event #3: ping_received
WS SMOKE PASS: 3 events, types: ping_received
```

**Architecture:**
- `MOCK_MODE=true` → `MockLiveEmitterService` → Socket.IO `/live` → `useLiveFeed` → `useHybridPingFeed`.
- `MOCK_MODE=false` → `KafkaLiveConsumerService` subscribes to `KAFKA_TOPIC_PINGS` and emits same envelope to tenant room.

**Verdict: PASS** (mock mode verified; real mode path exists in code, not run with real Kafka in this session).

---

### 3.2 — WS payload carries `state`

**Command:**
```
rg -n "state" services/api/src/live/
```

**Raw output:** *(no matches for state in live module)*

**Schema (`live-event.types.ts`):** `pingEventPayloadSchema` has `call_id`, `platform`, `tenant_id`, etc. — **no `state` field**.

Kafka `PingMessage` (`ingestion/internal/producer/producer.go`) **does** include `state`, but API consumer does not map it into WS payload.

**Verdict: GAP** — required for WS-driven live coverage; not required for first real ping test if coverage uses polling + ClickHouse.

---

### 3.3 — Coverage fetch-on-mount

**Code:** `web/src/app/(app)/coverage/page.tsx` — single `useEffect` → `fetchCoverageHeatmap(token)`; no `useLiveFeed`, no `pollMs`.

**Verdict: GAP (expected)** — not live today. **Option A** (poll hook, zero API change) is small and non-breaking.

---

### 3.4 — Date filter gap on coverage

**Command:**
```
rg -n "from|to|date|interval" services/api/src/coverage/coverage.service.ts
```

**Result:** No date filters — query is all-time `GROUP BY state` with `LIMIT 100`.

**Verdict: GAP (enhancement)** — dashboard `FilterBar` dates do not apply to coverage. Not a blocker for first real test.

---

## SECTION 4 — Data integrity at scale

### 4.1 — No last-50 sample bugs in aggregates

**Command:**
```
rg -n "limit 50|LIMIT 50|slice(0, 50)|take(50)" services/api/src/ web/src/ --glob "!*test*"
```

**Raw output:**
```
services/api/src/integrations/integrations.service.ts:25: LIMIT 50
web/src/components/enrichment/EnrichmentPanel.tsx:34: attributes.slice(0, 50)
web/src/components/enrichment/EnrichmentPanel.tsx:41: .slice(0, 50)
```

**Review:**
- `analytics.service.ts` — `count()`, `sum(revenue)` over full tenant (+ optional date) — **PASS**
- `coverage.service.ts` — `GROUP BY state LIMIT 100` — **PASS**
- `pings.service.ts` — list endpoint paginated (UI limit), not used for dashboard totals — **PASS**
- Integrations list `LIMIT 50` — admin UI only — **OK**
- Enrichment panel display cap — UI only — **OK**

**Verdict: PASS**

---

### 4.2 — Tenant isolation

**Command:**
```
rg -n "withTenant|tenant_id|RLS|CurrentTenant" services/api/src/ --glob "!*test*" | wc -l
→ 175 lines

rg -n "tenant" services/api/src/coverage/ services/api/src/analytics/ services/api/src/pings/
```

**Findings:**
- All three modules use `@CurrentTenant()` from JWT and `WHERE tenant_id = {tenant_id:UUID}` in ClickHouse.
- Analytics ad spend uses `postgres.withTenant(tenantId, ...)`.
- Ingestion resolves tenant from webhook token (never from body).

**Verdict: PASS** (application-layer; Postgres RLS assumed from migrations — not re-audited in this pass).

---

### 4.3 — Cost guards for real provider spend

**Command:**
```
rg -n "budget|cost|limit|guard|cap|quota" services/ -i --glob "*.go" --glob "*.ts" --glob "!*test*" | rg -i "deepgram|openai|token|minute|spend|cost" | head
```

**Findings:**
- `ask-ai/internal/cost` — `BudgetGuard` with `TokenBudgetDaily` — **PASS for Ask AI only**
- Scoring / Deepgram / OpenAI — **no spend cap** in codebase
- `scoring/internal/scoring/ai.go` — OpenAI stub only

**Verdict: GAP / TODO** — real Deepgram/OpenAI volume can incur unbounded cost.

---

### 4.4 — Idempotency / dedup

**Command:**
```
rg -n "idempoten|dedup|ON CONFLICT|unique|already processed" services/ --glob "!*test*" | head
```

**Mechanisms:**
| Layer | Mechanism |
|-------|-----------|
| Ingestion | Redis `SetNX` key `{tenant_id}:dedup:{platform}:{external_call_id}` TTL 24h (`ingestion/internal/dedup`) |
| Scoring | Redis idempotency guard `AlreadyScored` / `MarkScored` |
| Attribution | Redis dedup checker |
| Enrichment | Redis cache |

**GAP:** Duplicate pings are **still written** to ClickHouse with `is_duplicate=1`. Analytics and coverage queries **do not filter** `is_duplicate = 0`, so revenue/ping counts may include duplicates within TTL window.

**Verdict: PARTIAL**

---

## SECTION 5 — Operational readiness

### 5.1 — Clean one-command start

**Makefile:**
```makefile
dev-up:
	docker compose -f deploy/docker-compose.yml up -d --build
```

**Verdict: PARTIAL**
- `make dev-up` builds and starts stack with ClickHouse init SQL (fixtures).
- **Not** a single command from `docker compose down -v` — volumes persist unless manually cleared.
- No automatic `demo-seed` in `dev-up` (separate `make demo-seed`).

---

### 5.2 — Secrets not in git

**Command:**
```
rg -n "DEEPGRAM_API_KEY|OPENAI_API_KEY|_hmac_secret|password" --glob "*.yml" --glob "*.env*" . | rg -iv "change_me|example|\.example"
```

**Findings:**
- `.env.example` — empty placeholders for API keys; `dev_*_change_me` for dev HMAC.
- `deploy/docker-compose.yml` — `callloom_dev` ClickHouse password (local dev only).
- `.gitignore` — `.env`, `.env.*` ignored; `!.env.example`.

**Verdict: PASS** (no production secrets found; dev passwords documented).

---

### 5.3 — Health + readiness endpoints

**Command:**
```
rg -n "healthz|readyz" services/ --glob "!*test*" | wc -l
→ 20
```

Services with `/healthz` + `/readyz`: ingestion, consumer, scoring, enrichment, compliance-intelligence, ask-ai, ad-attribution, providerconfig, api.

**Verdict: PASS**

---

## Copy-paste cross-verification commands

```powershell
# 1. State normalizer
cd services/ingestion; go test ./internal/state/... -v

# 2. WebSocket live feed (MOCK_MODE=true, API up)
cd ../..; node scripts/ws-smoke-test.mjs

# 3. Coverage API (after login token)
# Invoke-RestMethod http://localhost:3000/coverage/heatmap -Headers @{Authorization="Bearer <token>"}

# 4. MOCK_MODE grep
rg -n "MOCK_MODE|mockMode" services/ --glob "*.ts" --glob "*.go" --glob "!*test*" | Select-Object -First 60

# 5. HMAC / integration secrets
rg -n "webhook_secret|VerifyRingbaHMAC" services/ingestion/ --glob "!*test*"

# 6. Sample limits (should NOT hit analytics/coverage)
rg -n "LIMIT 50|slice\(0, 50\)" services/api/src/ web/src/ --glob "!*test*"
```

---

## How to read this for first real test (Syed)

1. **Day-one real test scope:** Ringba (or one platform) → see pings on dashboard (refresh/poll) and coverage (page refresh). WS live in real mode needs `MOCK_MODE=false` on API so Kafka consumer feeds `/live`.

2. **Do NOT expect yet:** real Deepgram transcripts, real OpenAI scores, real DNC vendor, real Meta CAPI — flip `MOCK_MODE=false` on those services without code fixes will fail or silently mock.

3. **10 states / 126 pings** in local demo = ClickHouse fixture + seed data for demo tenant, **not** an API cap. Real pings with valid `state` codes populate heatmap dynamically (up to `LIMIT 100` states in API).

4. **Fast follows:** live coverage (Option A poll hook), WS `state` field, coverage date filters.

---

**Report generated by:** automated grep + `go test` + `ws-smoke-test.mjs` against repo at `c:\code base`.
