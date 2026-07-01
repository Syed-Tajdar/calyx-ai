# Calyx AI

Pay-per-call analytics and intelligence platform.

> **Presentation build:** `MOCK_MODE=true` · Teal Glass UI polish complete · 100% API↔UI coverage · Management docs in [`docs/`](docs/README.md)

See [`docs/design-system-v2.md`](docs/design-system-v2.md) for UI/UX specification.  
UI↔backend coverage: [`docs/ui-backend-coverage.md`](docs/ui-backend-coverage.md)  
**10-min demo:** [`docs/DEMO-SCRIPT.md`](docs/DEMO-SCRIPT.md)

## Credits

**Author · UI Design · Code:** Syed Tajdar

---

## System verification (last run: 2026-07-01)

| Layer | Result |
|-------|--------|
| Docker containers | **14/14** (10 app services running; postgres, redis, clickhouse, redpanda healthy) |
| Service health (`/healthz` + `/readyz`) | **10/10** (ports 8080–8087, API :3000) |
| API presentation checks | **20/20** (`verify-presentation.ps1`) |
| Web routes (`:3001`) | **13/13** (all product pages + `/calls/demo-call-1`) |
| WebSocket live feed | **PASS** (`ws-smoke-test.mjs` — ≥3 events, multi-platform rotation) |
| Web production build | **PASS** (16 routes) |
| Pipeline smoke (multi-platform E2E) | **PASS (4/4)** — `verify-system.ps1` runs `smoke-test-all-platforms.ps1` |

**Recent fixes (2026-07-01):**

- **UI polish pass:** shared `PlatformBadge` + `formatPlatformColor()` (Call Loom = teal); no `mock` platform in charts; compliance ring `29/100`; publishers card (MediaCo, LeadStream, DialPro, CallStream + quality badges); settings alerts (Email/Slack/SMS) and providers (Deepgram, OpenAI, Meta CAPI, DNC.com + Mock/Active status); Top Scores / Publishers table alignment.
- **Ping chart 7-day curve:** dashboard fetches 100 pings for last 7 days via `ping-timeline.ts` (filled daily buckets) — not the capped 50-row live sample.
- **API fix:** `GET /pings?from=...` no longer 500 when date filter is set (ClickHouse `received_at` alias collision resolved in `pings.service.ts`).

**One-command full check** (Docker + health + API + web + WS + tests + build + 4/4 pipeline — hard-fail if any layer fails):

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-system.ps1
# or: make verify-system
```

---

## Phase 1 — Complete

End-to-end multi-platform ping pipeline:

```
Ringba / Retreaver / TrackDrive / CallGrid
        ↓  /webhooks/{platform}/{token}
Ingestion (Go) → Kafka → Consumer (Go) → ClickHouse
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

**Ingestion notes**

- **Retreaver** accepts `call_uuid` plus optional `call_key` (private conversion-update credential for async buyer confirmations).
- `call_key` is stored in ClickHouse at ingestion time for a future conversion-update flow; it is **never** returned by the API, shown in the UI, or logged (ingestion logs `call_key_present` only).
- Before `MOCK_MODE=false` production go-live, encrypt `call_key` at rest (AES-256-GCM).

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
| `/dashboard` | Overview metrics, **7-day ping chart** (filled timeline), platform mix (ClickHouse aggregate + colored badges), platform filter, live activity strip (colored event pills), hybrid WS + poll feed, publisher quality card, Top Scores table |
| `/pings` | Ping feed — 5-platform filter (Ringba, Retreaver, TrackDrive, CallGrid, Call Loom), date filters, filtered live WS, accurate total badge, CSV export |
| `/scoring` | Call quality scores |
| `/scoring/scorecards` | Scorecard CRUD (rules editor) |
| `/calls/[callId]` | Native deep view (ping + score + compliance + attribution); demo: `/calls/demo-call-1` |
| `/intelligence` | Intelligence hub (Ask AI, compliance, attribution) |
| `/ask` | Natural language analytics chat |
| `/compliance` | TCPA risk scores, violations table, exposure |
| `/attribution` | Conversion events and platform status |
| `/coverage` | Geographic ping demand heatmap |
| `/reports` | Financial analytics (CPA, RPC, ROI, P&L) |
| `/settings` | Integrations, alerts (Email/Slack/SMS channel badges), provider bindings (real provider names + Mock/Active status) |

**Teal Glass design system:** Geist headings + Inter body, glass sidebar/topbar, teal aurora background, teal gradient CTAs and active nav. Shared platform labels via `web/src/lib/platform-options.ts` (`formatPlatformLabel`, `formatPlatformColor`). `PlatformBadge`, `ScoreBar`, and status pills in `web/src/components/ui/`. Tokens in [`web/src/styles/tokens/`](web/src/styles/tokens/). Dark theme for presentation demo. Storybook: `cd web && npm run storybook`.

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
powershell -ExecutionPolicy Bypass -File scripts/verify-system.ps1
node scripts/ws-smoke-test.mjs
make step1b-test
make test
```

| Command | What it checks |
|---------|----------------|
| `verify-system` | Full pre-push gate: Docker + healthz + API 20/20 + web 13/13 + WS + unit tests + web build + **pipeline 4/4** |
| `verify-presentation` | Phase 3 API + `platform-mix` + `demo-call-1` deep view/score/transcript + scorecard CRUD + fixtures (**20 checks**) |
| `smoke-test.ps1` | Single signed ping → ingestion → API (`-Platform ringba\|retreaver\|trackdrive\|callgrid`) |
| `smoke-test-all-platforms.ps1` / `make smoke-test-all` | E2E smoke for all four webhook platforms |
| `ws-smoke-test.mjs` | WebSocket `/live` receives ≥3 mock events (rotates ringba/retreaver/trackdrive/callgrid/callloom-native) |
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

No UI rewrite. Set provider env vars and flip `MOCK_MODE=false` — services fail fast if secrets are missing.

---

See `.calyx-ai/rules/` for full architecture rules.
