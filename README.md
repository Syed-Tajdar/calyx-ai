# Calyx AI

Pay-per-call analytics and intelligence platform.

> **Presentation build:** `MOCK_MODE=true` (intelligence) · `MOCK_EMITTER_ENABLED=false` (Kafka → `/live`) · **Retreaver + Ringba CERTIFIED** · **Coverage = US + Canada** · Ping Feed headline = **distinct calls** · API sync Test/Live · Redis `maxmemory 1gb` + `volatile-ttl` · Teal Glass UI · Docs in [`docs/`](docs/README.md)

See [`docs/design-system-v2.md`](docs/design-system-v2.md) for UI/UX specification.  
UI↔backend coverage: [`docs/ui-backend-coverage.md`](docs/ui-backend-coverage.md)  
**10-min demo:** [`docs/DEMO-SCRIPT.md`](docs/DEMO-SCRIPT.md)

## Credits

**Author · UI Design · Code:** Syed Tajdar

---

## System verification (last run: 2026-07-21)

| Layer | Result |
|-------|--------|
| Docker containers | App + infra up (`api` / `platform-sync` / `web` rebuilt for Ringba live + COPY+OPEN) |
| Service health | **PASS** (API :3000, Web :3001, platform-sync :8088) |
| **Ringba LIVE cert** | **YES** — D8/D9/D10/D5/D8b ([`docs/phase2-ringba-live-certified.md`](docs/phase2-ringba-live-certified.md)) |
| **Retreaver FINAL cert** | **YES** ([`docs/retreaver-final-certification-report.md`](docs/retreaver-final-certification-report.md)) |
| **Platform certification** | Retreaver **YES** · Ringba **YES** · TrackDrive next ([`docs/platform-certification.md`](docs/platform-certification.md)) |
| Redis bound memory | **PASS** — `maxmemory=1gb`, `volatile-ttl`, AOF; dedup flat on re-sync ([`docs/redis-maxmemory-report.md`](docs/redis-maxmemory-report.md)) |
| Deep-view (shared query) | **PASS** — 4/4 attributed / unattributed / transcript / Retreaver |
| Open in Ringba | **COPY+OPEN** — clipboard + report page + toast (rebuild `web` required) |

**Deploy note (2026-07-21):** After Ringba deep-link / Redis changes:

```powershell
cd deploy
docker compose build api platform-sync web
docker compose up -d --force-recreate api platform-sync web redis
# Hard-refresh http://localhost:3001 — Open in Ringba must open call-logs report + copy id
```

**Recent (2026-07-20 → 21) — Ringba LIVE certification + ops hardening:**

- **Official Get Call Log:** `POST https://api.ringba.com/v2/{accountId}/calllogs`, `Authorization: Token {apiToken}`, body `reportStart`/`reportEnd`/`filters`/`size`≤1000, response `report.records[]`. Legacy `CallLogs/Date` and `calllogs/details` → 301/404 on current host.
- **D9 YES:** live conversion **0 → 34.75**; `argMax` + distinct once. **D10** API 298 vs CH 262 (gap explained). **D5** WA/TX/LA 2-letter on coverage heatmap.
- **D8b:** two-way ID **operator-confirmed**; Ringba has **no per-call URL** — `Open in Ringba` copies `inboundCallId`, opens `#/dashboard/call-logs/report/new`, toasts paste-into-filter instructions ([`shared/providers/platform-deep-links.json`](shared/providers/platform-deep-links.json)).
- **Deep-view fixes:** JOIN on `call_id` (not `ON 1=1`); `p.state AS state` (avoid `transcripts.state` collision).
- **Redis:** `--maxmemory 1gb --maxmemory-policy volatile-ttl` (keep AOF); not `allkeys-lru` / `noeviction`.
- **Docs:** cert [`docs/phase2-ringba-live-certified.md`](docs/phase2-ringba-live-certified.md), parity audit [`docs/call-detail-parity-ringba-audit.md`](docs/call-detail-parity-ringba-audit.md), deferred gaps [`docs/enterprise-roadmap.md`](docs/enterprise-roadmap.md), quota research [`docs/ringba-quota-architecture-research.md`](docs/ringba-quota-architecture-research.md).
- Flipped Ringba back to **Test** after cert. **TrackDrive Phase 3** unblocked.

**Recent (2026-07-14) — Coverage = US + Canada + Feed distinct-calls header:**

- **Decision:** keep US + Canada (all four platforms support Canada — BC/ON/etc. are real calls, not junk).
- **NormalizeState:** shared US+CA allowlist (US states+DC+territories + Canadian provinces); unknown → `''`. `CA` = **California**; Canada is province codes only. Helpers: `CountryFor` / `web/src/lib/na-regions.ts`.
- **Coverage UI:** North America map (US topojson + Canada geojson), one shared graded-teal scale, `geoAlbers` NA projection, All·US·Canada filter, country in tooltip/detail, header **`N pings across M regions · US + Canada`** + `52 US · 4 CA` split (no “56 states”).
- **Ping Feed:** headline badge = `distinct_total` (`uniqExact(external_call_id)`); table still lists raw rows + DUP. `GET /pings` returns both `total` (raw, pagination) and `distinct_total`.
- Reports: [`docs/north-america-coverage-report.md`](docs/north-america-coverage-report.md), [`docs/coverage-north-america-ui-report.md`](docs/coverage-north-america-ui-report.md), [`docs/feed-coverage-count-report.md`](docs/feed-coverage-count-report.md).
- Follow-ups (not blockers): drop `aniHash` from call-detail URL; default-hide `apisync-%` demo rows ([`docs/url-privacy-and-data-check-report.md`](docs/url-privacy-and-data-check-report.md)).

**Recent (2026-07-13) — Retreaver clean account reset (new login):**

- **Why:** earlier Live pulls used a **different** Retreaver account than the one operators can log into now; old rows mixed with new and broke cross-verification.
- **Wipe:** `ALTER TABLE pings DELETE WHERE platform='retreaver'` → **0** rows; Redis `*:dedup:retreaver:*` cleared; all Retreaver **`api_sync`** integrations deleted (webhook **Retreaver Dev Postback** kept).
- **Next (UI only):** Settings → new Retreaver API-key integration → **Test** first → then **Live** + sync; credentials never in git/logs. Hard-refresh Ping Feed / Coverage / Settings.
- **D8b:** prefix match was already correct — prior “not found” was the **wrong account**. Re-confirm two-way prefix + deep link on the new account after Live.
- Report: [`docs/retreaver-account-reset-report.md`](docs/retreaver-account-reset-report.md).

**Recent (2026-07-13) — Retreaver FINAL certification + Phase 1.6:**

- **D9 API-level YES:** live call `c525ed11-…` revenue **0 → 13.51**; `updated_at` moved; Sync #2 second/third CH row; `argMax` once — [`docs/retreaver-final-certification-report.md`](docs/retreaver-final-certification-report.md).
- **D8b PASS (Case A):** Call Log truncated UUID / detail full UUID = `external_call_id`; exact-then-**prefix** search; copy + **Open in Retreaver** (`shared/providers/platform-deep-links.json`).
- **Update cursor:** `updated_at_start` + `sort_by=updated_at` + **15m safety delta**; empty/backfill window **per-integration** `initial_lookback_days` (default 30) + Settings **“Syncing last N days”** + **Backfill** action.
- **`platform_duplicate` rule (b):** exclude from revenue/payout; keep in ping counts; `duplicate_rate` on analytics.
- **Known limitation (not a blocker):** deep-page backfill can **delay** concurrent updates until the page walk catches up (not data loss) — Phase 2: content-hash dedup + `page=1` refresh ([`docs/platform-certification.md`](docs/platform-certification.md)).
- **Live WS:** API mock ping emitter **off** by default; Kafka consumer feeds `/live`. `ws-smoke-test.mjs` seeds signed webhooks.
- Docker: web build bakes `NEXT_PUBLIC_RETREAVER_CALL_URL_PATTERN` (`web/Dockerfile` + compose args).
- Plans/reports: [`docs/phase1.6-official-evidence-plan.md`](docs/phase1.6-official-evidence-plan.md), [`docs/phase1.6-evidence-report.md`](docs/phase1.6-evidence-report.md), [`docs/phase1.6-b1-b3-report.md`](docs/phase1.6-b1-b3-report.md), [`docs/phase1.6-b4-d8b-report.md`](docs/phase1.6-b4-d8b-report.md), [`docs/phase1.6-b5-d9-report.md`](docs/phase1.6-b5-d9-report.md).

**Recent (2026-07-10) — Retreaver API D9 mechanism + shared foundation:**

- **Update re-fetch (mechanism):** Retreaver incremental cursor is `updated_at_start` (not `created_at_start`); empty-cursor / backfill lookback default **30 days** (now per-integration). Early proof: [`docs/retreaver-api-d9-proof-report.md`](docs/retreaver-api-d9-proof-report.md). **Value-change YES:** [`docs/retreaver-final-certification-report.md`](docs/retreaver-final-certification-report.md).
- **Update-blindness fix:** analytics/coverage aggregates **no longer** filter `is_duplicate = 0`; identity is `(platform, external_call_id)` + `argMax(..., received_at)`. `/pings` still shows raw rows (DUP badge).
- **`platform_duplicate` column:** webhook payload “duplicate lead” split from Redis `is_duplicate` (CH migration `000009`); **rule (b)** — exclude from revenue; include in counts; `duplicate_rate` metric.
- **Timeline dedupe:** dashboard ping chart buckets distinct `(platform, external_call_id)`.
- **`occurred_at`:** Ping Feed / filters prefer platform call time (Retreaver `created_at`); fallback `received_at`.
- **Conformance harness:** `services/platform-sync/internal/adapter/conformance_test.go` — Retreaver certified; others pending.
- Rollout status: [`docs/platform-certification.md`](docs/platform-certification.md) · Phase reports: [`docs/phase0-shared-foundation-report.md`](docs/phase0-shared-foundation-report.md), [`docs/phase1-retreaver-certified.md`](docs/phase1-retreaver-certified.md), [`docs/phase1.5-close-gaps-report.md`](docs/phase1.5-close-gaps-report.md).

**Recent (2026-07-09) — Retreaver hardening + durable dedup (pre-Live):**

- **Credential redaction:** platform-sync never logs or persists raw `api_key` / `company_id` from HTTP URLs — `SanitizeErr` / `ClassifySyncError` on every sync log + `last_sync_error`. Transport-error path proven with bogus key + unresolvable host (`REDACTED >= 1`).
- **Durable aggregates:** analytics + coverage group by **`(platform, external_call_id)`** with `argMax(..., received_at)` (matches Redis dedup identity). **Update (2026-07-10):** `is_duplicate = 0` filter **removed** from aggregate inner queries so API re-syncs (revenue updates) are visible; `/pings` still shows DUP rows.
- **Dedup TTL:** `DEDUP_TTL_HOURS=168` (7 days) shared by ingestion + platform-sync (was 24h hardcoded on sync).
- **Retreaver V2 mapper:** unwraps `[{"call":{...}}]`; field fallbacks (`uuid`/`call_uuid`, `afid`/`client_afid`, `system_campaign_id`/`campaign_id`); empty `external_call_id` skipped (not published).
- **DNS resilience:** HTTP client retries transient network errors; platform-sync uses explicit DNS (`1.1.1.1`, `8.8.8.8`) in local compose; `last_sync_error` prefixed `network` / `authentication` / `mapping`.
- **Dev webhook re-seed:** `make seed-dev-integrations` restores Ringba/Retreaver/TrackDrive/CallGrid dev tokens (idempotent).
- Reports: [`docs/retreaver-hardening-report.md`](docs/retreaver-hardening-report.md), [`docs/pre-live-gate-report.md`](docs/pre-live-gate-report.md), [`docs/final-live-gate-report.md`](docs/final-live-gate-report.md), [`docs/dedup-path-answer.md`](docs/dedup-path-answer.md)

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
- **Dedup-correct aggregates:** `GET /analytics/metrics`, `/analytics/platform-mix`, and `/coverage/heatmap` use `(platform, external_call_id)` + `argMax` (as of 2026-07-10 they **do not** exclude `is_duplicate=1` — re-syncs are updates); `/pings` list still shows raw rows (including duplicates).
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
| Aggregates | `GROUP BY (platform, external_call_id)` + `argMax(..., received_at)` on metrics, platform-mix, coverage (**no** `is_duplicate = 0` filter — re-syncs are updates) |
| Dedup | Redis SET NX (`DEDUP_TTL_HOURS`, default **168h**) on webhook **and** API-sync; duplicates stored with `is_duplicate=1`, never dropped |
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
| Retreaver | API key + company ID | Incremental: `updated_at_start` + 15m safety delta; lookback `initial_lookback_days` (default 30); `occurred_at` ← `created_at` |
| Ringba | API access token + account ID | Date-range `CallLogs/Date` (`occurred_at` mapping pending Phase 2) |
| TrackDrive | Subdomain + public key + private key | `created_at_from` cursor (`occurred_at` mapping pending Phase 3) |
| CallGrid | API key | `startDate`/`endDate` + cursor (`occurred_at` mapping pending Phase 4) |

**Integrations API** (JWT, tenant-scoped): `GET /integrations`, `GET /integrations/platform-options`, `POST /integrations`, `POST /integrations/:id/test`, `POST /integrations/:id/sync`, **`POST /integrations/:id/backfill`**, **`PATCH /integrations/:id/sync-mode`** (optional `initial_lookback_days`), `DELETE /integrations/:id`.

Verify mock: `make verify-api-sync`. Real (per platform): set integration to **Live** in Settings, then `make verify-apisync-retreaver-real` (etc.) with platform env creds.

**State normalization:** all platform connectors normalize incoming `state` / `geo_state` to 2-letter uppercase US codes (`TX`, `CA`, …) via `internal/state.NormalizeState`. Full names (`Texas`) and invalid codes (`ZZ`) are coerced or blanked — ingestion never fails on geo data.

**Ingestion notes**

- **Retreaver** accepts `call_uuid` plus optional `call_key` (private conversion-update credential for async buyer confirmations).
- `call_key` is **encrypted at rest** (AES-256-GCM via `ANI_ENCRYPTION_KEY`) at ingestion publish; stored as hex ciphertext in ClickHouse. It is **never** returned by the API, shown in the UI, or logged (ingestion logs `call_key_present` only).
- **Dedup:** webhook and API-sync both use Redis `{tenant}:dedup:{platform}:{external_call_id}` (TTL `DEDUP_TTL_HOURS`, default 168h). Duplicates are **stored-and-flagged** (`is_duplicate=1`), never dropped with 409. Dashboard/coverage aggregates use `(platform, external_call_id)` + `argMax` (**do not** filter `is_duplicate`) so re-synced revenue updates win; `/pings` still lists raw rows (DUP badge). Platform-declared duplicate leads use `platform_duplicate` — **rule (b):** excluded from revenue/payout, included in ping counts, surfaced as `duplicate_rate`.

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
WebSocket /live → Kafka live consumer (default); mock WS emitter off via `MOCK_EMITTER_ENABLED=false`
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
| `/pings` | Ping feed — platform filter, **exact-then-prefix** call-ID search, Call ID copy, **FilterBar**, hybrid Kafka WS + poll, CSV export |
| `/scoring` | Call quality scores |
| `/scoring/scorecards` | Scorecard CRUD (rules editor) |
| `/calls/[callId]` | Native deep view; **Open in Retreaver** (per-call URL); **Open in Ringba** (COPY+OPEN → call-logs report); demo: `/calls/demo-call-1` |
| `/intelligence` | Intelligence hub (Ask AI, compliance, attribution) |
| `/ask` | Natural language analytics chat |
| `/compliance` | TCPA risk scores, violations table, exposure |
| `/attribution` | Conversion events and platform status |
| `/coverage` | **Coverage Intelligence** — North America map (US + Canada), graded teal heatmap, region detail panel, Top-15 chart, All·US·Canada filter; `GET /coverage/heatmap` |
| `/reports` | Financial analytics (CPA, RPC, ROI, P&L, **duplicate rate**); **FilterBar** period filter |
| `/settings` | Integrations (webhook vs API key, Test/Live, **Syncing last N days**, Sync / **Backfill**), alerts, provider bindings |

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

### MOCK_MODE / live feed switches

| Env | Default (compose) | Meaning |
|-----|-------------------|---------|
| `MOCK_MODE` | `true` | Intelligence providers (scoring/ask-ai/enrichment/…) stay mock unless flipped |
| `MOCK_EMITTER_ENABLED` (API) | `false` | Off = Kafka → `/live` only (no synthetic ping UUIDs). On = demo WS emitter |
| `NEXT_PUBLIC_MOCK_MODE` | `true` | Settings provider Mock/Active badges |

To go live on external AI/transcription later:

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
# Re-seed missing dev webhook tokens (idempotent — run if smoke returns unauthorized)
make seed-dev-integrations

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
| **`verify-ingestion-slice`** | **Phase 0:** durable dedup analytics/coverage + Retreaver `call_key` ciphertext (**required pre-push**) |
| **`verify-api-sync`** | API key integration (`sync_mode=mock`) → `platform-sync` sync → ping count increases (**required pre-push**) |
| `verify-apisync-*-real` | Real REST pull per platform (requires platform creds + integration **`sync_mode=real`**) |
| **`seed-dev-integrations`** | Re-seed Ringba/Retreaver/TrackDrive/CallGrid dev webhook tokens (idempotent) |
| `smoke-test.ps1` | Single signed ping → ingestion → API (`-Platform ringba\|retreaver\|trackdrive\|callgrid`) |
| `smoke-test-all-platforms.ps1` / `make smoke-test-all` | E2E smoke for all four webhook platforms |
| `ws-smoke-test.mjs` | WebSocket `/live` ≥3 events (seeds signed webhooks when API mock emitter is off) |
| `check-dashboard-browser.mjs` | Playwright smoke: login → dashboard, sidebar, **metric accent bars / delta chips**, charts (**10 checks** + screenshot to `docs/mockups/`) |
| `check-coverage-north-america.mjs` | Playwright: login → `/coverage`, US+Canada header, map paths, CA≠California collision, screenshot |
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
# Full stack app rebuild (recommended after Phase 1.6 / cert changes)
docker compose build api platform-sync web
docker compose up -d --force-recreate api platform-sync web

# Or targeted:
docker compose up -d --build ingestion api platform-sync
docker compose up -d --build web
```

After Live-gate / certification / hardening:

```powershell
cd deploy
docker compose build api platform-sync web
docker compose up -d --force-recreate api platform-sync web
# Confirm: http://localhost:8088/healthz and http://localhost:3000/healthz → 200
# Web: http://localhost:3001 — hard-refresh Ctrl+Shift+R
```

---

## Staging and real-time testing

### Staging process (recommended order)

1. **Local Docker (mock-default):** `MOCK_MODE=true` for intelligence; all API-key integrations on **Test** — run `make verify-system`, `make verify-ingestion-slice`, `make verify-api-sync`, `make smoke-test-all` (seed first if needed: `make seed-dev-integrations`).
2. **Staging namespace (EKS):** separate Postgres / ClickHouse / Redis; secrets via AWS Secrets Manager; Kong gateway as the only public entry.
3. **One platform Live at a time:** paste credentials in Settings UI only (never in code/logs/git); flip a single integration to **Live**; confirm `sync_status=ok`, real UUIDs + state in ClickHouse, and **no `api_key` in `docker logs deploy-platform-sync-1`**.
4. **Webhook path:** keep webhook integrations on real ingestion; validate with `smoke-test-all-platforms.ps1`.
5. **Rollback:** set integration back to **Test** — no global flag change required.

### Retreaver Live checklist (CERTIFIED · clean slate)

Status: [`docs/platform-certification.md`](docs/platform-certification.md) · Final proof: [`docs/retreaver-final-certification-report.md`](docs/retreaver-final-certification-report.md) · Account reset: [`docs/retreaver-account-reset-report.md`](docs/retreaver-account-reset-report.md).

Local DB was wiped of prior Retreaver rows/integrations so the **new** account does not mix with old test data. Create a **new** API-key row in Settings (do not reuse deleted ids).

1. Paste **new-account** Retreaver `api_key` + `company_id` in Settings (UI only — never commit).
2. Keep Data source = **Test** → Save → confirm mock sync ok.
3. Flip **only that row** to **Live** → Sync (or **Backfill** for empty cursor / lookback — UI shows **Syncing last N days**).
4. Verify: `sync_status=ok`; real UUIDs + 2-letter state + `occurred_at` + revenue; logs contain **no** raw `api_key`; `/pings` (exact **or prefix** call ID) + coverage update.
5. Cross-check in the **same** Retreaver account you can log into (prefix search works; prior miss = wrong account).
6. After a revenue change on the platform → Sync again → `/pings` may show DUP row; metrics count once with **latest** revenue (`argMax`).
7. Call detail → **Open in Retreaver** opens `retreaver.com/calls/{full-uuid}` (config-driven).
8. If anything is off → flip back to **Test** (one click). Keep only one platform Live at a time.

**Known limitation:** during deep-page initial backfill, concurrent updates may be delayed until the page walk catches up (not data loss) — see certification doc.

### Real-time testing

| Layer | Tool | Notes |
|-------|------|-------|
| Live feed | `node scripts/ws-smoke-test.mjs` | Kafka → `/live` when `MOCK_EMITTER_ENABLED=false`; seeds webhooks |
| Aggregates | `verify-ingestion-slice.ps1` | Poll / refresh — `(platform, external_call_id)` + `argMax` (no `is_duplicate` filter) |
| API sync | `verify-api-sync.ps1` | Scheduled + manual sync; mock adapter generates `apisync-*` pings |
| Real REST | Settings → Live + sync (or `verify-apisync-*-real.ps1`) | Creds in UI/env only; never commit secrets |
| UI | `check-dashboard-browser.mjs` + `/settings` | Playwright; Test/Live + Backfill on API-key rows |

---

## Post-approval (MOCK_MODE=false)

No UI rewrite. Set provider env vars and flip `MOCK_MODE=false` — services fail fast if secrets are missing.

---

See `.calyx-ai/rules/` for full architecture rules.
