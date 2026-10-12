# Calyx — v1 go-live prep report

**Branch:** `v1/golive-prep` from `main` @ `14c3068`  
**Login rule:** admin login unchanged. Demo rows stay in the database; production hides them.

Values below are masked. Nothing in this file is a usable secret.

## Scoreboard

| Item | Result | Proof |
|---|---|---|
| A1 | DONE | `$370` was MockAdSpendSource (`250+120`). v1 returns `[]` spend; UI hides Ad Spend / CPA / ROI; Profit (gross) = revenue − payout. |
| A2 | DONE | Providers and Alerts tabs hidden when `V1_ONLY`. Alerts use `overall_score` from scoring Kafka (`call_scored`) — off in v1. |
| A3 | DONE | `axisDateTicks` caps x-axis to ≤8 labels; points stay daily; tooltip keeps the exact date. |
| A4 | DONE | MOCK_MODE table below. Sidebar shows **Live** / **Test** in v1, never Mock. `MOCK_MODE` is env-driven. |
| A5 | DONE | Tenant name is `tenants.name`. Script `scripts/rename-tenant-devdabs.sql` renames by id only. Name is not a key. |
| A6 | DONE | `DEMO_MODE_ENABLED` forced off when `APP_ENV=production`. API ignores `show_demo`. Web hides Show-demo and Test option. Mock syncs skip in production. |
| B1–B6 | DONE | Answers below with file:line. |

## A1 — Source of the $370

`services/api/src/analytics/adspend.source.ts` `MockAdSpendSource.getSpend` returned two invented rows: `camp_google` **250** + `camp_bing` **120** = **370**. Compose sets `ADSPEND_SOURCE: mock` (`deploy/docker-compose.yml`). The spend is not filtered by `show_demo`, so Reports showed CPA/ROI/profit even with 0 pings.

v1 fix: MockAdSpendSource returns `[]` when `V1_ONLY` is on; UI hides Ad Spend, CPA, ROI; Profit label is **Profit (gross)** = revenue − payout.

## A2 — Alerts

Alert rules store `condition.field = 'overall_score'` and fire from Kafka topic `callloom.calls.call_scored.v1` (`services/api/src/alerts/alert-engine.service.ts` ~40–64). That needs the scoring service, which v1 does not run. **Both Alerts and Providers tabs are hidden in v1.**

## A4 — `MOCK_MODE` table (v1 services)

| Service | What `MOCK_MODE` changes | File:line |
|---|---|---|
| api | When true (and emitter on): synthetic WS pings. Compose already sets `MOCK_EMITTER_ENABLED=false`, so no fake live UUIDs. Also gates Deepgram secret when false. Fallback tenant name "Demo Tenant" if row missing. | `services/api/src/live/mock-live-emitter.service.ts:26`, `env.ts` MOCK_MODE, `auth.service.ts` getMe |
| web | `NEXT_PUBLIC_MOCK_MODE` only drove the old "Mock" badge / provider Active status. Badge now uses Live/Test from `/auth/me`. | `web/src/lib/provider-domains.ts:38`, `Sidebar.tsx` |
| platform-sync | Logged at boot (`mock_mode`). **Does not choose Live vs Test.** Fetch path uses per-integration `sync_mode` (`per_integration_sync_mode=true`). `Registry.Get` returns mock adapter only when `sync_mode != "real"`. | `cmd/server/main.go:92,167`, `adapter/registry.go:34–37` |
| ingestion | Only the Call Loom native stub: empty mock JSON when MOCK_MODE true; 501 when false. Webhooks/API sync unaffected. | `handlers/callloom_native.go:25–46` |
| consumer | **No `MOCK_MODE`.** | — |

**Does `mock_mode=true` stop a Live integration?** No. Live is `sync_mode=real` on the row. Global MOCK_MODE does not override that.

**Does it generate fake data on the server?** Not with current compose defaults: emitter off; mock adapter only runs for Test (`sync_mode=mock`) integrations. A6 skips those when production demo mode is off.

**Proposed server value:** `MOCK_MODE=false` (and keep `MOCK_EMITTER_ENABLED=false`). Documented in `deploy/.env.example`.

## A5 — Tenant rename

- Stored: Postgres `tenants.name` for id `11111111-1111-1111-1111-111111111111`.
- Shown: sidebar + top bar via `/auth/me` → `tenant_name`.
- Slug / cache / ClickHouse partition: **none** use the name; tenant id is the key.
- Server step: `scripts/rename-tenant-devdabs.sql` (read-only SELECT, then UPDATE by id).

## A6 — Demo mode off in production

- `isDemoModeEnabled()` returns **false** when `APP_ENV=production` (`demo-rows.ts`). Explicit `DEMO_MODE_ENABLED=true` does **not** override production (owner: production must never show/create demo data).
- Local default remains true.
- API: every `testDataExclusionSql` path keeps excluding `apisync-` / `demo-` / `smoke-` even if `show_demo=true`. Call detail for those ids → 404. Create/update with `sync_mode=mock` rejected.
- Web: no Show-demo toggle; new integrations only Live; list does not offer Test switch when demo off.
- platform-sync: `syncOne` skips non-`real` when demo off — log `"demo sync skipped"` (`syncer.go`).

Existing Test/mock rows stay. Laptop seed (and typical server seed from same migrations): Ringba/Retreaver/TrackDrive/CallGrid webhook "Dev" integrations + any paused api_sync Test rows.

### Rehearsal proof (`-p calyx-rehearsal`, `APP_ENV=production`, fake keys only)

| Check | Result |
|---|---|
| `/auth/me` | `tenant_name=Devdabs`, `demo_mode_enabled=false`, `has_live_integration` true with active Live / false when paused |
| CH demo rows still present | **58** distinct `demo-` / `apisync-` / `smoke-` ids |
| Metrics / PnL / pings with `show_demo=true` | all **0** / `$0` (demo ignored) |
| Coverage heatmap | `total_pings=0` |
| CSV export | header only (no demo rows) |
| `/native/call-deep-view?callId=demo-call-49` | **404** `Call not found` |
| Settings | Integrations tab only; data source **Live** only (no Test option); no Show-demo after `/auth/me` |
| Sidebar | **Live · Devdabs** with active fake Live; **Test · Devdabs** after pause |
| Fake Live sync | `sync_status=error`, `last_sync_error` starts with `authentication error: ringba HTTP 401` (fake token) |
| platform-sync | `"demo sync skipped"` for mock `rehearsal-dummy` (`reason=demo mode off`) |

Evidence PNGs under `docs/evidence/v1-golive-prep/` (local only, not committed).

---

## Part B

### B1 — New Live integration, first pull

- **First range:** `initial_lookback_days` defaults to **30** (`integrations.service.ts` `DEFAULT_LOOKBACK_DAYS = 30`). Settings column **"Lookback"** shows `Syncing last N days` from that field (`IntegrationsList.tsx`).
- First incremental window: adapters use lookback from "now" back N days when `last_sync_at` is empty (e.g. Retreaver `BuildRetreaverWindow`, Ringba `buildRingbaIncrementalWindow`).
- **Full 30-day backfill:** saving Live with lookback 30 already pulls ~30 days on first sync. To force a full re-pull: UI **History / backfill** button → `POST` backfill (`triggerBackfill` clears `sync_cursor` and `last_sync_at`, then syncs). Or after save, Sync once.
- Exact API path (Aryan, if UI unavailable): authenticated `POST /integrations/{id}/backfill` (Bearer + CSRF as usual for browser; script client for automation).
- **Duration estimate** (laptop volumes: Ringba 4,716 / Retreaver 3,493 per 30d):
  - Ringba: page size 1000, rate 75/min → ~5 pages ≈ **under 1–3 minutes** if one clean window (more if many windows / retries).
  - Retreaver: `per_page=100`, ~2/s → ~35 pages ≈ **under ~1–5 minutes** of request time; allow **10–20 minutes** with throttling and shortfall retries.

### B2 — Sweeps under production

Yes. Tickers are unconditional on APP_ENV (`cmd/server/main.go` ~174–220):

| Sweep | Window | Interval | Defaults |
|---|---|---|---|
| Hourly | last 7 days | `RingbaSweepInterval` / Retreaver equivalent | **1h** (`RINGBA_SWEEP_INTERVAL`, `RETREAVER_SWEEP_INTERVAL`) |
| Daily deep | last 30 days | deep interval | **24h** |

Times are **relative to process start**, not a fixed UTC clock hour. Only integrations whose adapter `SupportsRange(platform, sync_mode)` run (Live Ringba/Retreaver). Test/mock adapters do not.

### B3 — Health in the UI

Settings → Integrations table:

| Field | Where |
|---|---|
| `sync_status` | Pill: ok / syncing / error / degraded / idle (`IntegrationsList.tsx` ~278–285) |
| `last_sync_at` | Column (formatted) |
| `last_sync_error` | Hover `title` on the status pill |
| degraded | Style + label "Degraded — sync held" |

Fake Live keys → decrypt/test fail → status error / "Re-enter credentials" (hardening B5).

### B4 — Reconciliation queries (copy-paste)

**Converted columns (two meanings):**

| Column | Definition | Matches |
|---|---|---|
| `converted` | `countIf(has_converted = 1)` — platform **flag** only | Ringba / Retreaver conversion reports (their flag) |
| `converted_paid` | `countIf(has_converted = 1 AND revenue > 0)` — flag **and** revenue > 0 | Calyx **Converted** filter (`conversionFilterSql` / `conversionState` → `converted`) |

Calyx splits the flag into **Converted** (paid) and **Converted · $0** (`converted_zero`). Do not compare Ringba/Retreaver report totals to Calyx "Converted" alone — use `converted` for the platforms, `converted_paid` for Calyx.

**All-time by platform** (no date filter; laptop proof vs dashboard):

```sql
SELECT
  platform,
  count() AS calls,
  round(sum(if(dup = 0, rev, 0)), 2) AS revenue,
  countIf(conv = 1) AS converted,
  countIf(conv = 1 AND rev > 0) AS converted_paid
FROM (
  SELECT
    platform,
    external_call_id,
    argMax(revenue, received_at) AS rev,
    argMax(platform_duplicate, received_at) AS dup,
    argMax(has_converted, received_at) AS conv
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
    AND external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
)
GROUP BY platform
ORDER BY platform;
```

**Laptop proof (deploy stack):** Retreaver → **3,652** calls / **$33,143.85** revenue / converted **1,136** / converted_paid **1,017**. Same calls and revenue as `GET /analytics/metrics?platform=retreaver` all-time.

**Per platform per day** (deduped like the dashboard; day in account timezone):

```sql
-- Set timezone, e.g. America/New_York
SELECT
  platform,
  toDate(toTimeZone(event_at, 'America/New_York')) AS day,
  count() AS calls,
  round(sum(if(dup = 0, rev, 0)), 2) AS revenue,
  countIf(conv = 1) AS converted,
  countIf(conv = 1 AND rev > 0) AS converted_paid
FROM (
  SELECT
    platform,
    external_call_id,
    argMax(revenue, received_at) AS rev,
    argMax(platform_duplicate, received_at) AS dup,
    argMax(has_converted, received_at) AS conv,
    max(if(occurred_at > toDateTime(0), occurred_at, received_at)) AS event_at
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
    AND external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
)
WHERE toDate(toTimeZone(event_at, 'America/New_York')) >= today('America/New_York') - 7
  AND toDate(toTimeZone(event_at, 'America/New_York')) < today('America/New_York')
GROUP BY platform, day
ORDER BY platform, day;
```

(Dashboard uses the same `argMax` / event-at expression as `EVENT_AT_EXPR` in `ping-event-time.ts`.)

**Sync errors last 24h (Postgres):**

```sql
SELECT id, platform, name, sync_status, last_sync_at, left(COALESCE(last_sync_error, ''), 200) AS last_sync_error
FROM integrations
WHERE deleted_at IS NULL
  AND connection_mode = 'api_sync'
  AND (
    sync_status IN ('error', 'degraded')
    OR (last_sync_error IS NOT NULL AND last_sync_at > NOW() - INTERVAL '24 hours')
  )
ORDER BY last_sync_at DESC NULLS LAST;
```

### B5 — Laptop integrations (no secrets)

| platform | name | mode | is_active | sync_status |
|---|---|---|---|---|
| callgrid | CallGrid Dev Postback | webhook / Test | true | idle |
| retreaver | My test | api_sync / **Test** | **false** (paused) | ok |
| retreaver | Retreaver Dev Postback | webhook / Test | true | idle |
| ringba | Ringba Dev Pixel | webhook / Test | true | idle |
| trackdrive | TrackDrive Dev Postback | webhook / Test | true | idle |

**None is Live** (`sync_mode=real`).

**Go-live day — stop laptop pulls:**

1. Settings: pause every api_sync that might be Live (My test already paused).
2. `docker compose -p deploy -f deploy/docker-compose.yml stop platform-sync` (or full `stop`). **Never `down -v`.**

### B6 — Laptop vs server defaults (no secrets)

| Setting | Laptop (this machine) | Server / compose default |
|---|---|---|
| `APP_ENV` | development (local) | production |
| `MOCK_MODE` | true | propose **false** |
| `MOCK_EMITTER_ENABLED` | false | false |
| `DEMO_MODE_ENABLED` | true (effective) | forced **false** in production |
| `DEDUP_TTL_HOURS` | 240 | 240 |
| `WRITEBACK_SETTLE_SECONDS` | 20 | 20 |
| `RINGBA_RATE_PER_MIN` | unset → **75** | unset → **75** |
| `SYNC_INTERVAL_SEC` | 120 | 120 |
| Sweep 7d / 30d | 1h / 24h | same defaults |
| Chart / KPI day | viewer local TZ (web) | same; B4 query uses account TZ param |
| `WRITEBACK_ENABLED` | true only via gitignored local `.env` | **false** |
| Encryption keys | laptop `.env` (unchanged) | new server secrets |

---

## Server deploy note (Part A)

1. Deploy `v1/golive-prep` (or merge) on the server.
2. Rebuild images: **api**, **web**, **platform-sync** (and ingestion/consumer if compose pulled them).
3. Env (in `.env`, never paste values into chat):
   - `APP_ENV=production` (already)
   - `MOCK_MODE=false`
   - `DEMO_MODE_ENABLED` may be omitted; production forces demo off
   - Keep `MOCK_EMITTER_ENABLED=false`, `WRITEBACK_ENABLED=false`
4. `up -d` for changed services. Intelligence stay off.
5. Rename tenant:

```bash
DC="docker compose -p deploy -f deploy/docker-compose.yml"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT id, name FROM tenants ORDER BY created_at;"
$DC exec -T postgres psql -U callloom -d callloom -f - < scripts/rename-tenant-devdabs.sql
# or: UPDATE tenants SET name = 'Devdabs', updated_at = NOW() WHERE id = '11111111-1111-1111-1111-111111111111';
```

6. Confirm sidebar **Devdabs**, badge **Test** until a Live integration is active, Reports show no Ad Spend / CPA / ROI, no Show-demo toggle.

## Gates and laptop baseline

- Retreaver all-time after restoring `deploy`: **3,652** calls / **$33,143.85** (`GET /analytics/metrics?platform=retreaver`).
- Demo CH rows on laptop still present (**53** distinct `demo-` ids); hidden when Show demo is off.
- Gates: `pwsh-preview` `verify-system.ps1 -V1`, `verify-presentation.ps1 -V1`, `verify-ingestion-slice.ps1` (no `-V1` switch).

## Found, not fixed

- Sweep tickers are interval-from-start, not cron-at-UTC-hour.
- B4 laptop dashboard match for last 7 days should be re-checked by Aryan after timezone is confirmed with the owner.
- Rehearsal screenshot files are evidence-only; not committed.
- Brief Show-demo flash until `/auth/me` on older web images (fixed in source: default `demoModeEnabled=false`).
- `stage4/r3` untouched. Laptop encryption key untouched. Laptop `.env` untouched.
