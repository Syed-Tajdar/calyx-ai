# Calyx AI

Pay-per-call analytics and intelligence platform.

> **Presentation build:** `MOCK_MODE=true` · Phase 0 real ingestion slice · **API sync worker** · Teal Glass UI · 100% API↔UI coverage · Docs in [`docs/`](docs/README.md)

See [`docs/design-system-v2.md`](docs/design-system-v2.md) for UI/UX specification.  
UI↔backend coverage: [`docs/ui-backend-coverage.md`](docs/ui-backend-coverage.md)  
**10-min demo:** [`docs/DEMO-SCRIPT.md`](docs/DEMO-SCRIPT.md)

## Credits

**Author · UI Design · Code:** Syed Tajdar

---

## System verification (last run: 2026-07-08)

| Layer | Result |
|-------|--------|
| Docker containers | **14/14** app + infra running (`verify-system.ps1`) |
| Service health (`/healthz` + `/readyz`) | **12/12** (ports 8080–8088 incl. **platform-sync**, API :3000) |
| Full system gate | **PASS** (`verify-system.ps1` — Docker, health, API 20/20, web 13/13, WS, unit tests, web build, pipeline 4/4) |
| API presentation checks | **20/20** (`verify-presentation.ps1`) |
| **API sync (platform-sync)** | **PASS** (`verify-api-sync.ps1` — per-row `sync_mode=mock` → test + sync → ping +1) |
| **Phase 0 ingestion slice** | **PASS** (`verify-ingestion-slice.ps1` — dedup aggregates + `call_key` ciphertext) |
| Web routes (`:3001`) | **13/13** (all product pages + `/calls/demo-call-1`) |
| Dashboard browser smoke | **10/10** (`check-dashboard-browser.mjs`) |
| Settings sync_mode toggle | **PASS** (API `PATCH /integrations/:id/sync-mode` + `/settings` 200) |
| WebSocket live feed | **PASS** (`ws-smoke-test.mjs`) |
| Web production build | **PASS** (16 routes) |
| Pipeline smoke (multi-platform E2E) | **PASS (4/4)** |

**Recent (2026-07-08) — Per-integration `sync_mode` (Test / Live):**

- **`integrations.sync_mode`** (`mock` | `real`, default `mock`) — migration `000021_api_sync_sync_mode`; manual sync lookup via `get_api_sync_integration()` (`000022`).
- **platform-sync:** adapter choice is **per integration row**, not global `MOCK_MODE`; Retreaver can be Live while Ringba stays Test on the same stack.
- **API:** `POST /integrations` accepts `sync_mode`; `POST /integrations/:id/test` uses per-row mode; **`PATCH /integrations/:id/sync-mode`** for existing rows.
- **Settings UI:** Test / Live radio on add form; Test | Live toggle on API-key rows in the integrations list.
- Report: [`docs/mode-strategy-report.md`](docs/mode-strategy-report.md)

**Recent (2026-07-07) — API Sync / `platform-sync` worker:**

- **`platform-sync` service (Go, :8088):** polls `api_sync` integrations on an interval; REST adapters for Retreaver, Ringba, TrackDrive, CallGrid (`services/platform-sync/internal/adapter/`); publishes to the same `callloom.pings.v1` Kafka topic as webhooks.
- **Settings → Integrations:** choose **Webhook** or **API key** per platform; dynamic credential fields (`AddIntegrationForm`); **Sync now** button on API-key rows (`IntegrationsList`).
- **API:** `GET /integrations/platform-options`, `POST /integrations` (`connection_mode`), `POST /integrations/:id/test`, `POST /integrations/:id/sync`; credentials encrypted at rest (`CREDENTIAL_ENCRYPTION_KEY`).
- **Postgres:** migrations `000019_api_sync_integrations`, `000020_api_sync_status`; `callloom_sync` role for worker reads.
- **Verify mock:** `make verify-api-sync` · **Verify real (per platform):** `make verify-apisync-retreaver-real` (etc.) with env creds + integration **`sync_mode=real`** (Settings → Live).
- Reports: [`docs/apisync-platform-research.md`](docs/apisync-platform-research.md), per-platform reports in [`docs/apisync-*-report.md`](docs/).

**Recent fixes (2026-07-03) — Phase 0: Ingestion Slice Real:**

- **`call_key` encryption at rest:** Retreaver `call_key` encrypted with AES-256-GCM at ingestion `producer.Publish` (reuses `ANI_ENCRYPTION_KEY`); stored as hex ciphertext in Kafka + ClickHouse; never in API/UI/logs. Report: [`docs/ingestion-slice-report.md`](docs/ingestion-slice-report.md).
- **Dedup-correct aggregates:** `GET /analytics/metrics`, `/analytics/platform-mix`, and `/coverage/heatmap` exclude `is_duplicate = 1` rows; `/pings` list still shows raw rows (including duplicates).
- **Verification:** `scripts/verify-ingestion-slice.ps1` / `make verify-ingestion-slice` — unique ping +1, duplicate resend does not inflate totals, Retreaver ciphertext length check.
- **Mode scoping (Option B):** ingestion + consumer always real; intelligence services stay `MOCK_MODE=true` until Phases 1–4. Verify dashboard/coverage via **poll/refresh**, not WebSocket.
- **Coverage map labels:** state abbreviations (TX, CA, …) via `d3-geo` `geoCentroid` + `useMapContext` (no `Marker` — avoids React runtime error).
- **Docker web deploy:** `deploy-web-1` serves production Next.js on **:3001** (`docker compose up -d --build web`); local `npm run dev` and Docker web cannot share port 3001.

**Recent fixes (2026-06-22):**

- **Coverage Intelligence map:** interactive US choropleth (`react-simple-maps` + `d3-scale`) on `/coverage` — graded teal heatmap by ping volume, hover tooltip, click → compact detail panel; map sits directly on page background (no card wrapper); Top-15 bar chart with adaptive heading; state border teal pulse animation (like dashboard KPI icons); `web/public/geo/us-states-10m.json` self-contained TopoJSON.
- **State normalizer (ingestion):** `services/ingestion/internal/state` — `NormalizeState()` coerces Ringba/Retreaver full names (`Texas`) and mixed case (`tx`) to 2-letter uppercase at `buildPing` + `NormalizeRingba`; unknown geo → `""` (never drops ping). Verification: [`docs/all-states-verification-report.md`](docs/all-states-verification-report.md), [`docs/coverage-map-report.md`](docs/coverage-map-report.md).
- **Dashboard metric cards:** teal icons (`text-calyx-teal`); Compliance Exposure card aligned with other KPI headers; `ComplianceRiskRing` optional `align="start"`.

**Recent fixes (2026-07-01):**

- **Dashboard metric cards:** left teal gradient accent bar on all 4 KPI cards; layered **drop shadow** on `.calyx-metric-card` so cards lift off `#060B14` background; `MiniSparkline` area fill + stroke; period delta trend chips (from existing sparkline series via `sparkline-utils.ts`); muted 18px icons ($, phone, trending-up, shield); `ComplianceRiskRing` teal gradient arc; `MetricCardFrame` shared shell with hover depth.
- **FilterBar headings:** `Period` / `Platform` labels styled as uppercase section headings (not button-like) on dashboard, pings, and reports — `.filter-group-label` in `globals.css`.
- **Best of Both sidebar (A+B+C+D):** collapsible 64px icon rail (`localStorage` persist), left teal active bar, nav groups (Operations / Intelligence / System), Mock footer pill + tenant name; shared nav in `web/src/lib/app-navigation.ts` (`Radio` icon for Pings — crisp at 16px); `NavIcon` wrapper for desktop + mobile.
- **Dashboard metric pulse:** teal glow pulse on KPI card icons (staggered on dashboard); respects `prefers-reduced-motion`.
- **UI polish pass:** shared `PlatformBadge` + `formatPlatformColor()` (Call Loom = teal); no `mock` platform in charts; compliance ring `29/100`; publishers card (MediaCo, LeadStream, DialPro, CallStream + quality badges); settings alerts (Email/Slack/SMS) and providers (Deepgram, OpenAI, Meta CAPI, DNC.com + Mock/Active status); Top Scores / Publishers table alignment.
- **Ping chart 7-day curve:** dashboard fetches 100 pings for last 7 days via `ping-timeline.ts` (filled daily buckets) — not the capped 50-row live sample.
- **API fix:** `GET /pings?from=...` no longer 500 when date filter is set (ClickHouse `received_at` alias collision resolved in `pings.service.ts`).

**Sidebar design mockups:** [`docs/mockups/`](docs/mockups/) (options A–F comparison, Quick Win + Full Upgrade merge, browser verification screenshot).

**One-command full check** (Docker + health + API + web + WS + tests + build + 4/4 pipeline):

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-system.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-ingestion-slice.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-api-sync.ps1
# or: make verify-system && make verify-ingestion-slice && make verify-api-sync
```

**Pre-push gate:** all three scripts above must exit 0 before pushing to `main`.

---

## Phase 0 — Real ingestion slice (MOCK_MODE=true on intelligence)

First **real** slice of the ping pipeline — webhook auth → Kafka → ClickHouse → API reads. Intelligence (Deepgram, OpenAI, DNC, Meta) stays mock.

```
Webhook (Ringba/Retreaver/…) → Ingestion (Go) → Kafka → Consumer → ClickHouse
                                                              ↓
                                                    API analytics + coverage
```

| What changed | Detail |
|--------------|--------|
| `call_key` | AES-256-GCM encrypt at `producer.Publish`; hex in ClickHouse; API never returns it |
| Aggregates | `is_duplicate = 0` filter on metrics, platform-mix, coverage heatmap |
| Dedup | Redis at ingestion; duplicate rows stored with `is_duplicate=1` but excluded from totals |
| Verify | `make verify-ingestion-slice` |

Full report: [`docs/ingestion-slice-report.md`](docs/ingestion-slice-report.md) · Readiness: [`docs/real-readiness-report.md`](docs/real-readiness-report.md)

**Rebuild after Phase 0 code changes:**

```powershell
cd deploy
docker compose up -d --build ingestion api
```

---

## Phase 1 — Complete

End-to-end multi-platform ping pipeline:

```
Ringba / Retreaver / TrackDrive / CallGrid
        ↓  /webhooks/{platform}/{token}          ↓  API key (Settings)
Ingestion (Go) :8080                    platform-sync (Go) :8088
        ↓                                        ↓
        └──────────── Kafka (callloom.pings.v1) ─┘
                              ↓
                    Consumer (Go) → ClickHouse
                              ↓
                    API (NestJS) → Web (Next.js)

Call Loom native (direct source): GET /sources/callloom-native/v1/pings
```

**Core:** per-platform webhook auth, tenant-scoped API, dashboard, ping feed.

### Supported platforms

| Platform | Webhook route | Auth |
|----------|---------------|------|
| **Ringba** | `/webhooks/ringba/{token}` | `X-Ringba-Signature` HMAC-SHA256 |
| **Retreaver** | `/webhooks/retreaver/{token}` | `X-Retreaver-Signature` HMAC-SHA256 |
| **TrackDrive** | `/webhooks/trackdrive/{token}` | Token in query (`lead_token` / `auth_token`) |
| **CallGrid** | `/webhooks/callgrid/{token}` | `X-CallGrid-Signature` HMAC-SHA256 |
| **Call Loom native** | `/sources/callloom-native/v1/pings` | Direct API (not webhook) |

Each tenant integration is resolved by path token via PostgreSQL `lookup_integration_by_token`.

**Connection modes (Settings → Integrations)**

| Mode | User setup | Data path |
|------|------------|-----------|
| **Webhook** | Paste signing secret; configure postback URL in platform | Platform → Ingestion (:8080) → Kafka → ClickHouse |
| **API key** | Paste platform credentials only — no callback URL | `platform-sync` (:8088) pulls calls → same Kafka topic → ClickHouse |

API keys are encrypted at rest (`CREDENTIAL_ENCRYPTION_KEY`). Webhook signing and HMAC verification are unchanged.

**Per-integration sync mode (API key rows only):**

| `sync_mode` | UI label | Behavior |
|-------------|----------|----------|
| `mock` (default) | **Test** | Demo `apisync-{platform}-*` calls via mock adapter; test connection passes without real platform auth |
| `real` | **Live** | REST adapter for that platform; test connection calls `platform-sync /test-auth` |

Global `MOCK_MODE` still controls intelligence services (scoring, ask-ai, enrichment, etc.) — not API sync adapter selection.

Per-platform API sync credentials (Settings form fields):

| Platform | Fields |
|----------|--------|
| Retreaver | API key + company ID |
| Ringba | API access token + account ID |
| TrackDrive | Subdomain + public key + private key |
| CallGrid | API key |

**Integrations API** (JWT, tenant-scoped): `GET /integrations`, `GET /integrations/platform-options`, `POST /integrations`, `POST /integrations/:id/test`, `POST /integrations/:id/sync`, **`PATCH /integrations/:id/sync-mode`**, `DELETE /integrations/:id`.

Verify mock: `make verify-api-sync`. Real (per platform): set integration to **Live** in Settings, then `make verify-apisync-retreaver-real` (etc.) with platform env creds.

**State normalization:** all platform connectors normalize incoming `state` / `geo_state` to 2-letter uppercase US codes (`TX`, `CA`, …) via `internal/state.NormalizeState`. Full names (`Texas`) and invalid codes (`ZZ`) are coerced or blanked — ingestion never fails on geo data.

**Ingestion notes**

- **Retreaver** accepts `call_uuid` plus optional `call_key` (private conversion-update credential for async buyer confirmations).
- `call_key` is **encrypted at rest** (AES-256-GCM via `ANI_ENCRYPTION_KEY`) at ingestion publish; stored as hex ciphertext in ClickHouse. It is **never** returned by the API, shown in the UI, or logged (ingestion logs `call_key_present` only).
- **Dedup:** duplicate webhooks are stored with `is_duplicate=1`; dashboard/coverage **aggregates** exclude them; `/pings` feed may still list duplicate rows.

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
| `GET /analytics/platform-mix` | Tenant-wide platform breakdown (`GROUP BY platform` in ClickHouse) |
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
WebSocket /live → real-time ping + conversion events (MOCK_MODE emitter, 5-platform rotation)
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
| `/dashboard` | Overview metrics with **teal accent-bar KPI cards**, gradient sparklines, period delta chips, compliance ring; **7-day ping chart**; platform mix; **FilterBar** (Period/Platform as section headings); live activity strip; hybrid WS + poll feed; publisher quality card; Top Scores table |
| `/pings` | Ping feed — 5-platform filter (Ringba, Retreaver, TrackDrive, CallGrid, Call Loom), **FilterBar** date/platform filters, filtered live WS, accurate total badge, CSV export |
| `/scoring` | Call quality scores |
| `/scoring/scorecards` | Scorecard CRUD (rules editor) |
| `/calls/[callId]` | Native deep view (ping + score + compliance + attribution); demo: `/calls/demo-call-1` |
| `/intelligence` | Intelligence hub (Ask AI, compliance, attribution) |
| `/ask` | Natural language analytics chat |
| `/compliance` | TCPA risk scores, violations table, exposure |
| `/attribution` | Conversion events and platform status |
| `/coverage` | **Coverage Intelligence** — interactive US map (graded teal heatmap), state detail panel, demand bar chart; data from `GET /coverage/heatmap` (all states, not capped at 10) |
| `/reports` | Financial analytics (CPA, RPC, ROI, P&L); **FilterBar** period filter |
| `/settings` | Integrations (webhook vs API key, per-platform credential fields, **Sync now** for API sync), alerts (Email/Slack/SMS channel badges), provider bindings (real provider names + Mock/Active status) |

**Teal Glass design system:** Geist headings + Inter body, glass sidebar/topbar (collapsible grouped nav on desktop), teal aurora background, teal gradient CTAs and active nav left-bar. Dashboard KPI cards: `MetricCardFrame` accent bar, elevated `.calyx-metric-card` drop shadow, `MiniSparkline` gradient fill, `sparkline-utils` delta chips. Shared navigation: `web/src/lib/app-navigation.ts`. Shared platform labels via `web/src/lib/platform-options.ts` (`formatPlatformLabel`, `formatPlatformColor`). `PlatformBadge`, `ScoreBar`, `NavIcon`, `ComplianceRiskRing`, and status pills in `web/src/components/`. Tokens in [`web/src/styles/tokens/`](web/src/styles/tokens/). Dark theme for presentation demo. Storybook: `cd web && npm run storybook`.

---

## Quick start (local)

```bash
cp .env.example .env
make dev-up
```

`make dev-up` starts the full stack including **Web on Docker** at http://localhost:3001 (production Next.js build inside `deploy-web-1`). API at http://localhost:3000.

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
| **Platform sync** | **8088** | http://localhost:8088/healthz |
| API | 3000 | http://localhost:3000/healthz |
| **Web (Docker)** | **3001** | http://localhost:3001 |

**Dev login:** `admin@calyx.ai` / `calyx_dev`  
**Tenant A UUID:** `11111111-1111-1111-1111-111111111111` (displays as **Demo Tenant**)

### Web: Docker vs local dev

Both bind **port 3001** — only one can run at a time.

| Mode | Command | When to use |
|------|---------|-------------|
| **Docker (default)** | `cd deploy && docker compose up -d --build web` | Presentation demo, production-like build |
| **Local dev** | Stop Docker web first, then `cd web && npm run dev -- -p 3001` | Hot reload while editing UI |

```powershell
# Deploy / rebuild web on Docker (stop local dev on :3001 first if running)
cd deploy
docker compose up -d --build web

# Switch to local dev (free port 3001)
docker compose stop web
cd ..\web
npm run dev -- -p 3001
```

After UI changes, rebuild the Docker image — `npm run dev` does **not** update the container.

### MOCK_MODE switch

Default is `MOCK_MODE=true` — no external API keys required. Web container sets `NEXT_PUBLIC_MOCK_MODE=true` for provider status badges in Settings.

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

### Multi-platform dev tokens

| Platform | Path token | Secret / auth |
|----------|------------|---------------|
| Ringba | `dev_ringba_token_change_me` | HMAC secret: `dev_ringba_hmac_secret_change_me` |
| Retreaver | `dev_retreaver_token_change_me` | HMAC secret: `dev_retreaver_hmac_secret_change_me` |
| TrackDrive | `dev_trackdrive_token_change_me` | Query param `lead_token=dev_trackdrive_auth_token_change_me` |
| CallGrid | `dev_callgrid_token_change_me` | HMAC secret: `dev_callgrid_hmac_secret_change_me` |

```powershell
# All four webhook platforms
powershell -ExecutionPolicy Bypass -File scripts/smoke-test-all-platforms.ps1
# or: make smoke-test-all

# Single platform (Retreaver includes call_key in query)
powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1 -Platform retreaver
```

Retreaver postback example (query string):

```
call_uuid=[uuid]&call_key=[key]&caller=...&state=CA&event=incoming&revenue=9.75
```

### Demo seed (populated dashboard)

**Cold start:** ClickHouse fixture `000006_demo_fixture` + Postgres alert rules seed load on `clickhouse-init` / `migrate` — dashboard shows data without running the pipeline. Fixture backfills when `demo-call-1` or `demo-callloom-1` is missing (safe after smoke tests). Includes:

- 50 webhook demo pings (`demo-call-*`) across ringba, retreaver, trackdrive, callgrid, **callloom-native** (no `mock` platform)
- Pings spread across **7 days** (`received_at` varied by day) for dashboard ping chart curve
- Named publishers (MediaCo, LeadStream, DialPro, CallStream) in fixture rows
- 3 **Call Loom native** demo pings (`demo-callloom-1/2/3`) — filterable on `/pings` as **Call Loom**
- Scores, transcripts, compliance, attribution, enrichment for `/calls/demo-call-1`
- Postgres alert rules seeded with `email` / `slack` / `sms` channels (not `mock`)

Re-run fixture on existing stacks (delete demo rows first if dates need refresh):

```powershell
docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "ALTER TABLE pings DELETE WHERE tenant_id=toUUID('11111111-1111-1111-1111-111111111111') AND startsWith(external_call_id, 'demo-')"
cd deploy
docker compose run --rm clickhouse-init
```

**Full pipeline demo:**

```powershell
make demo-seed
```

Sends 25 signed Ringba pings across US states, waits for pipeline, then verifies API totals.

### Smoke & verification

```powershell
powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-presentation.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-ingestion-slice.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-api-sync.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify-system.ps1
node scripts/ws-smoke-test.mjs
make step1b-test
make test
```

| Command | What it checks |
|---------|----------------|
| `verify-system` | Full gate: Docker + healthz + API 20/20 + web 13/13 + WS + unit tests + web build + **pipeline 4/4** |
| `verify-presentation` | Phase 3 API + `platform-mix` + `demo-call-1` deep view/score/transcript + scorecard CRUD + fixtures (**20 checks**) |
| **`verify-ingestion-slice`** | **Phase 0:** dedup-correct analytics/coverage totals + Retreaver `call_key` ciphertext (**required pre-push**) |
| **`verify-api-sync`** | API key integration (`sync_mode=mock`) → `platform-sync` sync → ping count increases (**required pre-push**) |
| `verify-apisync-*-real` | Real REST pull per platform (requires platform creds + integration **`sync_mode=real`**) |
| `smoke-test.ps1` | Single signed ping → ingestion → API (`-Platform ringba\|retreaver\|trackdrive\|callgrid`) |
| `smoke-test-all-platforms.ps1` / `make smoke-test-all` | E2E smoke for all four webhook platforms |
| `ws-smoke-test.mjs` | WebSocket `/live` receives ≥3 mock events (rotates ringba/retreaver/trackdrive/callgrid/callloom-native) |
| `check-dashboard-browser.mjs` | Playwright smoke: login → dashboard, sidebar, **metric accent bars / delta chips**, charts (**10 checks** + screenshot to `docs/mockups/`) |
| `check-coverage-map.mjs` | Playwright smoke: login → `/coverage`, map paths, pulse border, API state count |
| `check-coverage-localhost.mjs` | Localhost coverage map: SVG paths, state labels, API heatmap states |
| `demo-seed` | 25 pings for demo-ready dashboard |
| `step1b-test` | Go providerconfig unit tests |
| `make test` | Full test matrix (Go incl. `platform-sync` + API) |

### Web & API build

```bash
make build-web
make build-api
make build-platform-sync
```

**Rebuild Docker images** after ingestion/API/platform-sync or UI changes:

```powershell
cd deploy
docker compose up -d --build ingestion api platform-sync   # ingestion + API + API sync worker
docker compose up -d --build web                         # production web on :3001
```

Then hard-refresh the browser at http://localhost:3001 (`Ctrl+Shift+R`).

---

## Staging and real-time testing

### Staging process (recommended order)

1. **Local Docker (mock-default):** `MOCK_MODE=true` for intelligence; all API-key integrations on **Test** — run `make verify-system`, `make verify-ingestion-slice`, `make verify-api-sync`.
2. **Staging namespace (EKS):** separate Postgres / ClickHouse / Redis; secrets via AWS Secrets Manager; Kong gateway as the only public entry.
3. **One platform Live at a time:** flip a single integration to **Live** in Settings → Integrations; run `make verify-apisync-retreaver-real` (or ringba / trackdrive / callgrid) with platform creds in env — do not flip global `MOCK_MODE` until intelligence providers are ready.
4. **Webhook path:** keep webhook integrations on real ingestion; validate with `smoke-test-all-platforms.ps1`.
5. **Rollback:** set integration back to **Test** — no global flag change required.

### Real-time testing

| Layer | Tool | Notes |
|-------|------|-------|
| Live feed | `node scripts/ws-smoke-test.mjs` | MOCK_MODE emitter; multi-platform rotation |
| Aggregates | `verify-ingestion-slice.ps1` | Poll / refresh dashboard — dedup excludes `is_duplicate=1` |
| API sync | `verify-api-sync.ps1` | Scheduled + manual sync; mock adapter generates `apisync-*` pings |
| Real REST | `verify-apisync-*-real.ps1` | Requires platform creds + integration `sync_mode=real` |
| UI | `check-dashboard-browser.mjs` + `/settings` | Playwright (Edge channel); Test / Live toggle on API-key rows |

---

## Post-approval (MOCK_MODE=false)

No UI rewrite. Set provider env vars and flip `MOCK_MODE=false` — services fail fast if secrets are missing.

---

See `.calyx-ai/rules/` for full architecture rules.
