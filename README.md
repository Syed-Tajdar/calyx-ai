# Calyx AI

Pay-per-call analytics and intelligence platform — Kaliper competitor.

> **Presentation build:** `MOCK_MODE=true` · Teal Glass UI complete · 100% API↔UI coverage · Management docs in [`docs/`](docs/README.md)

See [`docs/design-system-v2.md`](docs/design-system-v2.md) for UI/UX specification.  
UI↔backend coverage: [`docs/ui-backend-coverage.md`](docs/ui-backend-coverage.md)  
**10-min demo:** [`docs/DEMO-SCRIPT.md`](docs/DEMO-SCRIPT.md)

## Credits

**Author · UI Design · Code:** Syed Tajdar

---

## System verification (last run: 2026-06-29)

| Layer | Result |
|-------|--------|
| Docker containers | **14/14** (10 app services running; postgres, redis, clickhouse, redpanda healthy) |
| Service health (`/healthz` + `/readyz`) | **10/10** (ports 8080–8087, API :3000) |
| API presentation checks | **16/16** (`verify-presentation.ps1`) |
| Web routes (`:3001`) | **13/13** (all product pages + `/calls/demo-call-1`) |
| WebSocket live feed | **PASS** (`ws-smoke-test.mjs` — ≥3 events) |
| Unit tests (Go + API Jest) | **PASS** |
| Web production build | **PASS** (16 routes) |
| Pipeline smoke (signed ping E2E) | **FAIL** — ingestion returns `401 unauthorized` (HMAC/integration lookup); demo dashboard unaffected via ClickHouse fixtures |

**One-command full check:**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-system.ps1
# or: make verify-system
```

---

## Phase 1 — Complete

End-to-end ping pipeline:

```
Ringba Pixel → Ingestion (Go) → Kafka → Consumer (Go) → ClickHouse
                                              ↓
                                    API (NestJS) → Web (Next.js)
```

**Core:** signed webhook ingestion, tenant-scoped API, dashboard, ping feed.

---

## Phase 2 — Mock-first intelligence (MOCK_MODE=true)

```
Ping → Enrichment (Go) → ClickHouse attrs        [pre-bidding]
Ping → Mock emitter → call_transcript_ready → Scoring (Go) → scores + transcripts
Deepgram callback → API → Kafka → Scoring consumer
call_scored → Alert engine (API) → mock notifications
```

**Core:** call quality scoring, transcripts, audience enrichment, financial analytics, coverage heatmaps, alert rules, scorecards.

### Phase 2 API endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /scoring/scores` | List call scores (tenant-scoped) |
| `GET /scoring/transcript?callId=` | Get transcript |
| `POST /scoring/deepgram/callback` | Deepgram → Kafka publish |
| `GET /enrichment/:aniHash` | Enrichment attributes |
| `GET /coverage/heatmap` | Geo demand heatmap |
| `GET /analytics/metrics` | CPA/RPC/ROI |
| `GET /analytics/pnl` | P&L summary |
| `GET /alerts/rules` | Alert rules CRUD |
| `GET /scorecards` | List scorecards |
| `POST /scorecards` | Create scorecard |
| `PATCH /scorecards/:id` | Update scorecard |
| `DELETE /scorecards/:id` | Soft-delete scorecard |

---

## Phase 3 — Intelligence layer (MOCK_MODE=true)

```
Ping → Compliance (Go) → risk scores + violations → ClickHouse + Kafka
Ask AI (Go) → intent → safe SQL → ClickHouse read + Postgres query log
Ping → Attribution bridge → conversions → Ad platforms (mock) → ClickHouse
API native deep view → single call story (ping + score + compliance + attribution)
Provider config (Go) → tenant bindings + secret resolve (Redis cache)
WebSocket /live → real-time ping + conversion events (MOCK_MODE emitter)
```

**Core:** TCPA compliance intelligence, natural-language Ask AI, ad attribution, native deep call view, provider config bindings, live dashboard feed.

### Phase 3 API endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /providers/catalog` | Provider catalog |
| `GET /providers/bindings` | Tenant provider bindings |
| `POST /providers/bindings` | Create binding (invalidates Redis cache) |
| `PATCH /providers/bindings/:id` | Update binding |
| `DELETE /providers/bindings/:id` | Delete binding |
| `POST /providers/bindings/:id/test` | Test binding |
| `POST /providers/resolve` | Resolve provider for tenant+domain |
| `GET /compliance/risk-scores` | TCPA risk scores (ClickHouse) |
| `GET /compliance/violations` | Compliance violations |
| `GET /compliance/exposure` | Liability exposure summary |
| `POST /ask` | Natural language analytics question |
| `GET /ask/history` | Prior Ask AI questions |
| `GET /attribution/events` | Conversion fire log (ClickHouse) |
| `GET /attribution/status` | Per-platform fired/failed summary |
| `GET /native/call-deep-view?callId=` | Full call story (ping + score + compliance + attribution) |

### Phase 3 Web UI

| Route | Description |
|-------|-------------|
| `/login` | Auth (dev credentials below) |
| `/dashboard` | Overview metrics, live activity strip, hybrid WS + poll feed |
| `/pings` | Ping feed — date filters, live WebSocket, CSV export |
| `/scoring` | Call quality scores |
| `/scoring/scorecards` | Scorecard CRUD (rules editor) |
| `/calls/[callId]` | Native deep view (ping + score + compliance + attribution) |
| `/intelligence` | Intelligence hub (Ask AI, compliance, attribution) |
| `/ask` | Natural language analytics chat |
| `/compliance` | TCPA risk scores, violations table, exposure |
| `/attribution` | Conversion events and platform status |
| `/coverage` | Geographic ping demand heatmap |
| `/reports` | Financial analytics (CPA, RPC, ROI, P&L) |
| `/settings` | Integrations, alerts, provider bindings (create/edit/delete/test) |

**Teal Glass design system:** Geist headings + Inter body, glass sidebar/topbar, teal aurora background, teal gradient CTAs. Tokens in [`web/src/styles/tokens/`](web/src/styles/tokens/). Dark theme for presentation demo. Storybook: `cd web && npm run storybook`.

---

## Quick start (local)

```bash
cp .env.example .env
make dev-up
```

| Service | Port | Health |
|---------|------|--------|
| Ingestion | 8080 | http://localhost:8080/healthz |
| Consumer | 8081 | http://localhost:8081/healthz |
| Scoring | 8082 | http://localhost:8082/healthz |
| Enrichment | 8083 | http://localhost:8083/healthz |
| Provider config | 8084 | http://localhost:8084/healthz |
| Compliance | 8085 | http://localhost:8085/healthz |
| Ask AI | 8086 | http://localhost:8086/healthz |
| Ad attribution | 8087 | http://localhost:8087/healthz |
| API | 3000 | http://localhost:3000/healthz |
| Web | 3001 | http://localhost:3001 |

**Dev login:** `admin@calyx.ai` / `calyx_dev`  
**Tenant A UUID:** `11111111-1111-1111-1111-111111111111` (displays as **Demo Tenant**)

### MOCK_MODE switch

Default is `MOCK_MODE=true` — no external API keys required.

To go live later:

```env
MOCK_MODE=false
TRANSCRIPTION_PROVIDER=deepgram
AI_PROVIDER=openai
DNC_PROVIDER=...
ATTRIBUTION_PROVIDER=meta
DEEPGRAM_API_KEY=...
DEEPGRAM_CALLBACK_SECRET=...
OPENAI_API_KEY=...
```

Services **fail fast on boot** if real mode secrets are missing (no silent mock fallback).

### Ringba pixel setup

1. In Ringba: **Integrations → Pixels → Add Pixel**
2. Event: **Incoming** (or Completed)
3. URL (GET or POST JSON):

```
http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me?CallID=[Call:InboundCallId]&caller_id=[tag:InboundNumber:Number]&state=[tag:Geo:State]&event=incoming&call_revenue=[Call:ConversionPayout]
```

4. **HMAC required:** every request must include `X-Ringba-Signature`. Dev secret: `dev_ringba_hmac_secret_change_me`

### Demo seed (populated dashboard)

**Cold start:** ClickHouse fixture `000006_demo_fixture` + Postgres alert rules seed load on `clickhouse-init` / `migrate` — dashboard shows data without running the pipeline.

**Full pipeline demo:**

```powershell
make demo-seed
```

Sends 25 signed Ringba pings across US states, waits for pipeline, then verifies API totals.

### Smoke & verification

```powershell
powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-presentation.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-system.ps1
node scripts/ws-smoke-test.mjs
make step1b-test
make test
```

| Command | What it checks |
|---------|----------------|
| `verify-system` | Docker + all healthz + API + web routes + WS + unit tests + web build |
| `verify-presentation` | Phase 3 API endpoints + scorecard CRUD + fixtures (16 checks) |
| `smoke-test.ps1` | Single signed ping → ingestion → API (live pipeline) |
| `ws-smoke-test.mjs` | WebSocket `/live` receives ≥3 mock events |
| `demo-seed` | 25 pings for demo-ready dashboard |
| `step1b-test` | Go providerconfig unit tests |
| `make test` | Full test matrix (Go + API) |

### Web & API build

```bash
make build-web
make build-api
```

---

## Post-approval (MOCK_MODE=false)

No UI rewrite. Set provider env vars and flip `MOCK_MODE=false` — services fail fast if secrets are missing. See [`docs/10-known-gaps-and-roadmap.md`](docs/10-known-gaps-and-roadmap.md).

---

See `.calyx-ai/rules/` for full architecture rules.
