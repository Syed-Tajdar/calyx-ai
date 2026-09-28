# Calyx — S0 Verify Report

**Spec:** `docs/s0-verify-spec.md`  
**T0 (UTC):** `2026-09-24T20:44:46Z`  
**T0+120m:** `2026-09-24T22:44:46Z` (V1 window closed)  
**Integration:** `ce6fd338-af48-47d8-850a-bd10b855e2f4` (Ringba, `sync_mode=real`, cursor **not** cleared)  
**Evidence root:** `docs/evidence/s0-verify/`  
**Status:** COMPLETE + addendum (V1.4 / V6 / V7 / V8). No fixes. No Phase 2B. No push.

---

## 1. Verdict table

| Item | Verdict | Severity now | Evidence |
|---|---|---|---|
| F003 (a) normal ticks | **false** for permanent freeze; ≤1 sync-interval lag **[PROVEN]** | P2 lag | V1.2 / V1.3 — cursor advances; 15/15 eventually in CH |
| F003 (b) downtime > 2h | **true** — permanent miss of calls in `(last, now−2h)` **[PROVEN]** | **P0** | V1.4: gap ~2h51m; Ringba 5 in critical subgap, CH **0**; 5 ids still absent |
| F003 (c) calls longer than 2h | **true** — post-ingest Ringba field updates can be missed **[PROVEN]** | **P0** / P1 | V7: `RGB7CE620…` CH `has_converted=0` vs Ringba `hasConverted=true` |
| F015 size-flush exit | **inconclusive** — size path (≥500) **not observed**; ticker outage path kept loop alive, all pings landed | size-exit unproven | V2 |
| Kafka retention vs outage | **7d `retention.ms`**, RF=1, `cleanup.policy=delete`, `retention.bytes=-1` | P1 ops | `rpk topic describe -a` |
| Tenant filter — every CH query | tenant reads JWT + `{tenant_id:UUID}`; date **values** parameterised (V8) | P2 demo/`EVENT_AT_EXPR` fragment nuance | V4.1 + V8 |
| Date-clause injection (V8) | **not P0** — `from`/`to` are `{from:String}` / `{to:String}` params, not raw SQL | — | V8 code path |
| Postgres RLS / FORCE | **`ask_ai_query_log` ENABLE=t FORCE=f**; app roles no bypass | P1 | `pg_class` / `pg_roles` |
| Ringba `callDt` timezone | **epoch ms → UTC; matches CH** | — | V5.1 |
| Period-filter timezone | **F053 true** — browser local TZ | P2 | V5.3 |
| API RestartCount=5 (V6) | **5× `api bootstrap failed: KafkaJSNumberOfRetriesExceeded`** (Redpanda down/unreachable at boot) | P1 ops | V6 logs |

---

## Pre-check

`docker compose ps` + `docker inspect` before V1: all app/infra containers **running** except one-shot init (`migrate`, `kafka-init`, `clickhouse-init` exited 0).

| Container | State | RestartCount | RestartPolicy |
|---|---|---|---|
| deploy-ad-attribution-1 | running | 0 | no |
| deploy-api-1 | running | 5 | unless-stopped |
| deploy-ask-ai-1 | running | 0 | no |
| deploy-clickhouse-1 | running | 0 | no |
| deploy-compliance-intelligence-1 | running | 0 | no |
| deploy-consumer-1 | running | 0 | no |
| deploy-enrichment-1 | running | 0 | no |
| deploy-ingestion-1 | running | 0 | no |
| deploy-platform-sync-1 | running | 0 | no |
| deploy-postgres-1 | running | 0 | no |
| deploy-providerconfig-1 | running | 0 | no |
| deploy-redis-1 | running | 0 | no |
| deploy-redpanda-1 | running | 0 | no |
| deploy-scoring-1 | running | 0 | no |
| deploy-web-1 | running | 0 | no |

Runtime consumer env: `BATCH_SIZE=500`, `BATCH_FLUSH_MS=5000`. Sync: `SYNC_INTERVAL_SEC=120`.

---

## V1 — F003: frozen cursor / missed calls

### V1.1 — Code path (precise)

Each scheduled tick (`SYNC_INTERVAL_SEC=120`):

1. `syncer.syncOne` loads `item.SyncCursor` + `item.LastSyncAt` (`syncer.go:119-128`).
2. `RingbaAdapter.FetchPage` (`ringba_adapter.go:64-97`):
   - Builds **fresh** window: `cs := ringbaDefaultCursor(lastSyncAt, lookbackDays)` (`:65`).
   - `ringbaDefaultCursor` (`:111-124`): `end = now UTC`; if `lastSyncAt` parseable, `start = lastSyncAt − ringbaOverlap` (`ringbaOverlap = 2h`, `:31`); else lookback days.
   - **If `cursor != ""`:** `DecodeCursor(cursor, &cs)` (`:66-67`) **overwrites** `ReportStart`/`ReportEnd`/`Offset`/`Size` with the **stored** absolute window.
   - Fetches `fetchWindowed(ctx, creds, start, end)` (`:81`).
   - On success, **next** cursor = `ringbaDefaultCursor(time.Now().UTC().Format(RFC3339), lookback)` (`:85`) → stores `ReportEnd=now`, `ReportStart=now−2h` (argument is **now**, not DB `last_sync`).
3. `syncer` persists `nextCursor` + `last_sync_at=now` (`syncer.go:147-153`).

**Ambiguity:** none on overwrite. Runtime question was permanent freeze vs one-interval lag.

### V1.2 — Observe over time (cursor not cleared)

| Sample | `sync_cursor` (decoded) | `last_sync_at` | Latest CH `occurred_at` (Ringba) | Latest Ringba `callDt` (last 3h) |
|---|---|---|---|---|
| T0 `20:44:46Z` | start=`18:42:48Z` end=`20:42:48Z` | `20:42:48+00` | `19:47:10` (count 11610) | `19:47:10.177Z` id `…V3OMD01` (13 recs) |
| T0+30m `21:14:46Z` | start=`19:12:48Z` end=`21:12:48Z` | `21:12:48+00` | **`21:02:23`** (11614) | **`21:02:23.687Z`** id `…RGBBE02` (14) |
| T0+60m `21:44:46Z` | start=`19:42:48Z` end=`21:42:48Z` | `21:42:48+00` | **`21:37:03`** (11616) | **`21:37:03.775Z`** id `…RGBD502` (14) |
| T0+120m `22:44:46Z` | start=`20:42:48Z` end=`22:42:48Z` | `22:42:48+00` | **`22:06:30`** (11619) | **`22:44:00.212Z`** id `…RGB0802` (14 in last3h probe) |

**Logs (requested window not separately logged):** ticks log `calls` / `published` only. Examples: `20:58`/`21:02`/`21:04` `published=1`; through `22:10–22:42` mostly `calls=9 published=0 unchanged=9`.

**Observation:** `report_end` **tracks** `last_sync_at` and **advances every ~2m** — not a stuck ancient end. At +30m/+60m CH newest **matched** Ringba newest. At +120m CH lagged newest Ringba calls by ~2–38 minutes at sample time (see V1.3).

### V1.3 — Reconcile `[T0−1h, T0+120m]` = `[19:44:46Z, 22:44:46Z]`

| Source | Distinct calls |
|---|---|
| Ringba Get Call Log | **15** (`v1_3_ringba_reconcile.txt`) |
| ClickHouse at sample | **14** present; **1 missing** at sample: `RGB91F401…RGB0802` (`callDt=22:44:00Z`) |
| ClickHouse after catch-up | **15** — both lagging IDs present: `…RGBCC02` received_at `22:44:48`; `…RGB0802` received_at `22:46:48` (`v1_3_missing_later.txt`) |

**Traffic:** 15 ≥ 10 → window **conclusive** for permanent-miss claim.

**Verdict F003:** audit claim that the window **freezes and new calls are permanently missed** is **false** `[PROVEN]`. Overwrite still causes the fetch end to trail “now” by up to ~one sync interval; newest calls can appear in CH 1–2 ticks later — lag, not freeze.

---

## V2 — F015: size-triggered flush during CH outage

**Run only after V1 closed.** Batch size from runtime = **500** (≤2000 — proceeded). Sent **550** (`batch+50`) `smoke-s0-*` webhooks (`ping_send.txt`: `sent=550 fail=0`).

### Before (`docs/evidence/s0-verify/v2/before_*`)

| Check | Value |
|---|---|
| poison_total (log) | not on `/metrics` (404); logs later show `poison_total=0` |
| container | `running` pid=`3862` restartCount=`0` |
| group | Stable, TOTAL-LAG=`0` |
| `/healthz` | `ok` |
| `/readyz` | `ok` |

### During (CH stopped ≥60s)

| Check | Value |
|---|---|
| container | **still `running`** same pid=`3862` restartCount=`0` |
| group | Stable, TOTAL-LAG=**550** (partition 2) |
| `/healthz` | **`ok`** |
| `/readyz` | **`clickhouse unavailable` http=503** (not green) |
| logs | `batch insert failed … count=308` → per-message transient failures; then `per-message flush done written=272 retry=36 poison_total=0`; **`batch flush failed`** (this log string is emitted only from **ticker** `flushLoop`) |

### After (`docker start deploy-clickhouse-1`, healthy)

| Check | Value |
|---|---|
| container | `running` **same pid=`3862`** restartCount=`0` |
| group | TOTAL-LAG=**0** |
| `/healthz` / `/readyz` | `ok` / `ok` http=200 |
| smoke rows | **`550` distinct `smoke-s0-%`** in `default.pings` |
| recovery flushes | `batch flushed count=36` then `count=242` (272+36+242=550) |
| `consumer stopped` | **not observed** in logs for this window |

### Verdict F015

Spec criteria need the **size** path (`len(batch) >= BatchSize` → `append` returns error → `Run` exits).

**Runtime:** only observed flush during outage had **`count=308` (< 500)** and failure logged as ticker `"batch flush failed"`. Size path **was not exercised**. Ambiguous why batch never reached 500 before ticker (5s flush vs fetch rate under lock) — **not inferred**.

- Loop alive + all 550 landed → does **not** confirm “size path kills Run”.
- `/readyz` was **not** green during outage (contrary to audit’s “health stays green” for both probes).
- **Verdict: inconclusive** for F015 size-flush exit. Conclusive re-test would need a flush log with `count≥500` during outage, or an explicit size-path marker.

---

## V3 — Live Kafka / Redis / restart policies

### Topics (all 12 `callloom.*`)

Uniform runtime config (`rpk topic describe -a`):

| Field | Value |
|---|---|
| partitions | 3 |
| replicas | 1 |
| `retention.ms` | **604800000** (7d) |
| `retention.bytes` | **-1** |
| `cleanup.policy` | **delete** |
| `retention.local.target.ms` | 86400000 (1d) |

### Consumer groups (sample at V3 capture)

| Group | State | Members | TOTAL-LAG (then) |
|---|---|---|---|
| callloom-consumer | Stable | 1 | 0 |
| callloom-scoring | Stable | 1 | 66 |
| callloom-enrichment | Stable | 1 | 0 |
| callloom-compliance | Stable | 1 | 4315 |
| callloom-attribution | Stable | 1 | 50 |
| callloom-attribution-bridge | Stable | 1 | 4228 |
| calyx-api-live-feed | Stable | 1 | (see evidence) |

Full describe: `docs/evidence/s0-verify/v3_groups_describe.txt`.

### Restart policies (complete)

| Container | RestartPolicy |
|---|---|
| deploy-api-1 | **unless-stopped** |
| all other deploy-* listed in pre-check / `v3_restart_policies.txt` | **no** |

Consumer has **no** restart policy `[PROVEN]`.

### Redis

`maxmemory=1073741824` (1GiB); `maxmemory-policy=volatile-ttl`; `evicted_keys=0`.

---

## V4 — Tenant isolation

### V4.1 — Every ClickHouse query in v1 `services/api`

v1 services `ingestion` / `consumer` / `platform-sync` / `web`: **no CH SELECT** of tenant analytics data.

| `file:line` | Endpoint / caller | Filters `tenant_id`? | Tenant value from | Parameterised or interpolated? |
|---|---|---|---|---|
| `clickhouse.service.ts:52` | `/readyz` ping | no | none | literal `SELECT 1` |
| `analytics.service.ts:74` | `GET /analytics/metrics` | yes | JWT/auth (`@CurrentTenant`) | `{tenant_id:UUID}` param; demo SQL fragment + date clause **interpolated** |
| `analytics.service.ts:154` | `GET /analytics/platform-mix` | yes | JWT/auth | same as metrics |
| `pings.service.ts:87` | pings list count | yes | JWT/auth | `{tenant_id:UUID}`; `EVENT_AT_EXPR` **interpolated** |
| `pings.service.ts:103` | pings list rows | yes | JWT/auth | same |
| `pings.service.ts:136` | ping-by-id / detail path | yes | JWT/auth | same |
| `coverage.service.ts:26` | coverage | yes | JWT/auth | `{tenant_id:UUID}` |
| `scoring.service.ts:30` | score by call | yes | JWT/auth | `{tenant_id:UUID}` + `{call_id:String}` |
| `scoring.service.ts:48` | transcript by call | yes | JWT/auth | same |
| `scoring.service.ts:72` | scores list | yes | JWT/auth | `{tenant_id:UUID}` |
| `enrichment.service.ts:30` | enrichment count | yes | JWT/auth | `{tenant_id:UUID}` + `{ani_hash:…}` |
| `enrichment.service.ts:42` | enrichment attrs | yes | JWT/auth | same |
| `compliance.service.ts:44` | risk list | yes | JWT/auth | `{tenant_id:UUID}` |
| `compliance.service.ts:69` | violations list | yes | JWT/auth | same |
| `compliance.service.ts:88` | summary | yes | JWT/auth | same |
| `attribution.service.ts:33` | attributed list | yes | JWT/auth | `{tenant_id:UUID}` |
| `attribution.service.ts:53` | attribution summary | yes | JWT/auth | same |
| `deep-query.service.ts:139` | call deep view | yes (joins) | JWT/auth | `{tenant_id:UUID}` + `{call_id:String}` |
| `deep-query.service.ts:342` | enrichment count (deep) | yes | JWT/auth | `{tenant_id:UUID}` |
| `deep-query.service.ts:354` | enrichment sample (deep) | yes | JWT/auth | same |

**Findings (strict “filtered, from auth, parameterised”):**

1. `clickhouse.service.ts:52` — no tenant (health) — acceptable probe.
2. Several queries **interpolate** demo-exclusion / `EVENT_AT_EXPR` / date fragments into SQL while **tenant_id remains parameterised from JWT** — not body/query tenant. Count as **nuance findings**, not cross-tenant bypass `[PROVEN` for param tenant`; interpolation = code reading]`.

No CH read takes tenant from request body/query.

### V4.2 — Postgres RLS (runtime)

```
ask_ai_query_log         ENABLE=t FORCE=f   ← finding
(all other tenant tables) ENABLE=t FORCE=t
schema_migrations        ENABLE=f FORCE=f
```

Roles: `callloom` **rolbypassrls=t**; `callloom_api`, `callloom_ingestion`, `callloom_sync` **bypass=f**.  
Compose: API → `callloom_api`; sync → `callloom_sync`; ingestion → `callloom_ingestion`.

---

## V5 — Time handling

### V5.1 — Ringba `callDt`

- **Raw** Get Call Log: JSON number, **epoch milliseconds** (e.g. `1790023993594`).
- Adapter: `parseFlexibleTime` → `time.UnixMilli(n).UTC()` (`ringba_adapter.go:551-554`). No offset string; assumed UTC via Unix ms.
- Same call in CH: `occurred_at` matches UTC from ms. **Conversion correct** `[PROVEN]`.

**Operator cross-check:** call id `RGBF5C80033AB3A6ECB43AB3CFC51211509148A8F11RGBDE02` — Ringba UI should show **2026-09-21 20:53:13 UTC** (or local equivalent of that instant).

### V5.2 — Retreaver `created_at` (code only; sync off)

`parseRetreaverTime`: RFC3339Nano / RFC3339 / `2006-01-02T15:04:05.999Z` / `2006-01-02 15:04:05` → **`.UTC()`** (`retreaver_adapter.go:256-270`). Not runtime-verified.

### V5.3 — Period filters

Web `presetToStart('7d')`: local midnight 7 days ago → `toISOString()` (`date-filter.tsx`).  
This host: local `2026-09-18T00:00:00+05:00` → API `from=2026-09-17T19:00:00.625Z`.  
API uses the supplied ISO `from`/`to` as-is in CH date clause. **F053 confirmed** — browser local TZ, not tenant TZ.

---

## 3. New findings discovered during verification

1. **F015 size path not hit** despite sending `batch+50` — ticker flushed at 308 first; size-exit claim remains unproven at runtime.
2. **`/readyz` returns 503** when ClickHouse is down (healthz stays `ok`) — audit’s “both stay green” is inaccurate for readyz on this build.
3. **Consumer `/metrics` is 404** — `poison_total` only visible in structured logs (`poison_total=0` during V2).
4. **Compliance / attribution-bridge groups** carry multi-thousand lag (pre-existing; not caused by V2).
5. **Only `deploy-api-1` has `unless-stopped`**; consumer and most services have restart policy `no`.
6. **F003 (b) downtime > 2h gap** — 5 Ringba calls permanently missing from CH after ~2h51m sync outage (V1.4).
7. **F003 (c) long-call updates** — at least one call with duration > 2h has Ringba `hasConverted=true` while latest CH row is `has_converted=0` (V7).

---

## 4. Container state after V2 recovery

All compose services **running** again; ClickHouse **healthy**. Consumer **same process** (pid 3862, restartCount 0). Kafka consumer group lag **0**. `smoke-s0-%` count **550**.

---

## Addendum (read-only) — V1.4 / V6 / V7 / V8

Temporary probe `cmd/probe-s0-addendum` was created, run, and **deleted** (no lasting code change).

### V1.4 — Downtime gap (> 2h overlap)

**Container timeline (laptop / Docker restart 2026-09-24):**

| Event | UTC |
|---|---|
| Last `api sync completed` for `ce6fd338` before gap | **2026-09-24T17:37:54Z** |
| No further `ce6fd338` ticks until container stop | (empty `--since 17:40 --until 20:24`) |
| `platform-sync` FinishedAt | `2026-09-24T20:24:21Z` |
| `platform-sync` StartedAt | `2026-09-24T20:26:47Z` |
| First tick after restart | **2026-09-24T20:28:49Z** (`published=0`); next `20:30:48` `published=9` |

**Gap duration:** last tick → first tick ≈ **2h 51m** → **> 2h overlap → exercised** (not “not exercised”).

| Window | Ringba distinct | CH distinct | Missing |
|---|---|---|---|
| Full gap `[17:37:54, 20:28:49)` | **14** | **9** | **5** |
| Critical `(last, now−2h)` = `(17:37:54, 18:28:49)` | **5** | **0** | **5** |

**Missing ids (absent from CH entirely — count 0):**

1. `RGB02246DCF267ADF1A7FFDDC39F684979F2C80FD2BRGBDC02` (`callDt` 18:19:32Z)
2. `RGB5090F80269C565B1C27D30C5B875BB99886983E0RGB8602` (17:59:26Z)
3. `RGBA47E8638A3C758DCEDBF40F45A8EBD2B3662D0B9RGB3502` (17:57:47Z)
4. `RGB782856E991EB8C551D54B49618B9C6C2EC2E25A3RGBBD02` (17:52:46Z)
5. `RGB31DF11E9CAEF9AA6F1361C0DDB53B7152D7AD797RGBE302` (17:44:55Z)

Evidence: `v1_4_ringba_gap.txt`, `v1_4_ringba_critical_subgap.txt`, `v1_4_ch_gap_ids.txt`, `v1_4_missing_in_ch.txt`.

**Verdict F003 (b):** downtime > 2h **permanent miss is true** `[PROVEN]`.

---

### V6 — API RestartCount=5

| Fact | Value |
|---|---|
| `RestartCount` | **5** |
| `RestartPolicy` | `unless-stopped` |
| Container `Created` | `2026-09-23T23:00:28Z` |
| `"api server started"` log lines in current container | **2** — `2026-09-23T23:00:40Z`, `2026-09-24T20:26:36Z` |

**Cause (from `docker logs`):** exactly **five** process-level failures:

```
api bootstrap failed: KafkaJSNumberOfRetriesExceeded: Connection timeout
```

(repeated 5×). Preceding kafkajs errors: `ECONNREFUSED` / `ENOTFOUND redpanda` / `Connection timeout` while Redpanda was not ready or DNS-missing during stack bring-up / laptop wake. Docker `unless-stopped` then restarted the container → `RestartCount=5`.

No OOM (`OOMKilled=false`). Per-restart exit codes beyond the final `FinishedAt` are not retained by Docker; the five bootstrap-failed lines are the attributable reasons `[PROVEN]`.

---

### V7 — Long calls vs 2h window

**Max `duration_seconds` (Ringba, last 30 days):** **13994** (`v7_max_dur_30d.txt`).  
**Calls with connected duration > 7200s in CH:** 7 ids (6 with activity in 30d window).

**Ringba create vs update:** `callDt` / CH `occurred_at` sit at an early timestamp while `connectedCallLengthInSeconds` is already multi-hour on first ingest; several calls later gain a second CH row with `has_converted` flipping **0 → 1** at a later `received_at` without changing `occurred_at`. That is runtime evidence Ringba **updates** the same call-log record after creation (conversion / flags), and Calyx only picks those up when a later sync still returns the row.

| Call id | CH latest (argMax by received_at) | Ringba now | Diff? |
|---|---|---|---|
| `RGB7CE620…RGB8802` | dur=14399, **has_converted=0**, revenue=18 | connectedLen=14399, **hasConverted=true**, conversionAmount=18 | **yes** — conversion flag |
| `RGBB40312…RGB2702` | dur=13994, has_converted=1, revenue=19.8 | 13994 / true / 19.80 | no |
| `RGBE7DE72…RGB6002` | 12567 / 1 / 30.6 | 12567 / true / 30.60 | no |
| `RGBF13772…V3_7D01` | 8668 / 1 / 19.8 | 8668 / true / 19.80 | no |
| `RGBE1E581…RGB4602` | 8097 / 1 / 33.3 | 8097 / true / 33.30 | no |
| `RGBDD86A0…V38ID01` | 7922 / 1 / 27 | 7922 / true / 27.00 | no |
| `RGB6E7CC4…V3OEB01` | 9071 / 1 / 14.04 | **MISS** in Ringba window probe | **inconclusive** (API miss) |

**Verdict F003 (c):** long-call / late-field updates **can be missed** by the `callDt`-anchored 2h incremental window — **true** `[PROVEN]` via `RGB7CE620…`.

---

### V8 — Date-clause interpolation / injection

**HTTP → SQL path (`GET /analytics/metrics`, `/pnl`, `/platform-mix`):**

1. `analytics.controller.ts:12-16` — Zod `dateQuerySchema`: `from` / `to` are **`z.string().optional()`** (no datetime format regex).
2. Passed to `getMetrics` / `getPlatformMix` as strings.
3. `buildDateClause` (`analytics.service.ts:203-208`):

```ts
if (from) parts.push(`AND ${EVENT_AT_EXPR} >= parseDateTimeBestEffort({from:String})`);
if (to) parts.push(`AND ${EVENT_AT_EXPR} < parseDateTimeBestEffort({to:String})`);
```

4. Values set as `params.from` / `params.to` and sent via ClickHouse **`query_params`** (`:71-72`, `:87`, `:150-151`, `:166`).

**Same pattern for pings list:** `pings.filters.ts:27-37` — `{from:String}` / `{to:String}` params.

What is “interpolated” into the SQL **string** is only the **fixed** clause text + constant `EVENT_AT_EXPR` / demo-exclusion prefixes (not user `from`/`to`). User `from`/`to` are **bound parameters**, not concatenated literals. They are **not** parsed into a JS `Date` and re-formatted; they are passed through as strings into `parseDateTimeBestEffort`.

**Crafted `from` alter SQL?** **No** (assuming `@clickhouse/client` parameter binding holds). A crafted value can make `parseDateTimeBestEffort` fail or select a weird instant; it cannot close quotes / inject statements via this path. **Not P0.**

(Correction to V4.1 wording: date **clause template** is interpolated; date **values** are parameterised.)

---

STOP. No fixes. No Phase 2B. No push.
