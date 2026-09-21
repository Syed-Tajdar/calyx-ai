# Calyx — Pre-Phase-2 Report

**Date:** 2026-09-22  
**Scope:** Parts A, B, C only. No Phase 2. No write-back behaviour change. Ringba Live `ce6fd338-af48-47d8-850a-bd10b855e2f4` left `sync_mode=real`, `is_active=false`, `writeback_enabled=true`.  
**No `DELETE` and no `ALTER TABLE … DELETE` was executed.**  
**Evidence:** `docs/evidence/pre-phase2/`

---

## Scoreboard

| Part | Result |
|---|---|
| A — gate covers platform-sync + pingdedup | **PASS** — `verify-system.ps1` exit 0, Docker **15/15**, Healthz **11/11**, `internal/writeback` and `packages/pingdedup` in the log |
| B1 — three-prefix exclusion, rows kept | **PASS** — rebuilt `deploy-api`; `demo-call-1` still loads |
| B2 — `smoke-` IDs + two-line proof | **PASS** — ingestion slice exit 0; all four connectors accepted `smoke-` |
| B3 — `verify-api-sync.ps1` | **PASS** exit 0; created integration deactivated, not deleted. Not added to the gate |
| B4 — mock api_sync generators | **PASS** — none were active; `apisync-%` count **3 → 3** over 180s |
| C — inventory only | **PASS** — counts below; statements proposed, not run |

---

## Part A

`scripts/verify-system.ps1` gained coverage only. Existing checks were not removed or reordered.

| Addition | Where |
|---|---|
| `platform-sync` | Docker app list (after `web`) |
| `:8088/healthz` | health ports (after `:3000/healthz`, before the existing `:3000/readyz` check) |
| `services\platform-sync`, `packages\pingdedup` | Go test list (after `ad-attribution`) |

First full run **exited 1**. `go test ./...` in platform-sync failed to **compile** `internal/config/config.go`: `Load()` was missing `}, nil` and the function close (`unexpected envOrDefault, expected (`). Write-back tests themselves were **`ok`** (`internal/writeback`). pingdedup was **`ok`**. The close was restored so the package compiles. That is not a write-back behaviour change.

Green run (`docs/evidence/pre-phase2/verify-system.txt`):

```
Docker:     15 / 15
Healthz:    11 / 11
API:        PASS (20/20)
Web routes: 13 / 13
WebSocket:  PASS
Unit tests: PASS
Web build:  PASS
Pipeline:   PASS (4/4)
SYSTEM VERIFY PASS
VERIFY_SYSTEM_EXIT=0
```

`ok …/platform-sync/internal/writeback` and `ok …/packages/pingdedup` are in that log. `[PROVEN]`

---

## Part B

### B1 — exclusion helper

`services/api/src/common/demo-rows.ts`: `testDataExclusionSql` excludes `apisync-%`, `demo-%`, and `smoke-%`. `show_demo=true` returns an empty fragment (all three included). `apiSyncExclusionSql` and `demoExclusionSql` are aliases. Rows are not deleted.

Anti-drift test asserts all three prefixes. `npx jest src/common/demo-rows.spec.ts`: **6 passed**.

API image **rebuilt** and `deploy-api-1` recreated. The image would not compile: `CallDeepViewPing.has_converted` was required and not mapped. The deep-view query now selects and assigns `has_converted`. Not a write-back change. Needed so the B1 image could be built.

The browser live-feed filter in `web/src/lib/use-hybrid-ping-feed.ts` uses the same three prefixes. The **running web container was not rebuilt**. KPI reads go through the rebuilt API. `[PROVEN]` API; web container filter `[UNKNOWN]` until that image is rebuilt.

**Default KPIs** (tenant A, no `show_demo`):

| | Before B1 (old image) | After B1 (rebuilt image) |
|---|---|---|
| `total_pings` | 4167 | 4114 |
| `total_revenue` | 20025.51 | 19222.76 |
| `total_exposure_usd` | 3517.65 | 3372.45 |

Drop: **53 pings**, **$802.75** revenue. That equals the `demo-%` family below (53 calls, row-sum revenue 802.75). `[PROVEN]`

`verify-presentation.ps1` **exit 0**. Deep view, score, and transcript for `demo-call-1` still pass (lookup by id, not by the aggregate filter). Web route `/calls/demo-call-1` passed inside `verify-system`. `[PROVEN]`

### B2 — connectors and scripts

No connector parses call ids as UUIDs. Ringba, Retreaver, TrackDrive, and CallGrid only reject an **empty** id. `[PROVEN]` from connector source, then live: pipeline smoke **4/4** with `smoke-<uuid>` (`E2E PASS` ringba, retreaver, trackdrive, callgrid).

Scripts now generate `smoke-<uuid>` and read `/pings`, `/analytics/metrics`, and `/coverage/heatmap` with `show_demo=true` when they find or count those rows.

`verify-ingestion-slice.ps1` **exit 0**:

```
show_demo=true   total 4168 -> 4169
default view     total 4114 -> 4114
```

Default view did not move. The increment check still fails the run if the inclusive total does not rise. `[PROVEN]`

### B3 — verify-api-sync

**Exit 0.** Growth read with `show_demo=true`: `4170 -> 4172`. Found `apisync-retreaver-20260921-…`.

Created `5011b82c-d66f-417d-9365-2e94d427d4d5`, then `is_active=false` (`UPDATE 1`). Not deleted. Guard refuses `ce6fd338`. Not added to `verify-system.ps1`. `[PROVEN]`

### B4 — mock generators

Before and after, this query returned **0 rows**:

```sql
connection_mode='api_sync' AND is_active=true AND sync_mode='mock'
```

Webhook dev rows (`Ringba Dev Pixel`, `* Dev Postback`) left active. Ringba Live untouched.

`apisync-%` row count: **3** at T0, **3** at T0+180s. Growth stopped. `[PROVEN]`

---

## Part C — inventory (nothing deleted)

ClickHouse `default.pings` after the green gate. Revenue below is **row-sum**, not the KPI `argMax` figure, except where noted.

| Family | How identified | Distinct calls | Rows | Revenue in default KPIs |
|---|---|---|---|---|
| `apisync-%` | prefix | 3 | 3 | excluded (already, and still) — row-sum $33.75 |
| `demo-%` | prefix | 53 | 53 | excluded after B1 — row-sum $802.75 (matches the KPI drop) |
| `smoke-` | prefix (new, this work) | 9 | 10 | excluded after B1 — row-sum $141.25 |
| Old smoke | see provenance | 14 | 15 | **still counted** — row-sum $202.25; KPI-style `argMax` sum **$195.00** |
| Real Ringba | `RGB%` and `sync_path='api_sync'` | 4100 | 4100 | counted — row-sum $20194.06 |
| Real Retreaver | `platform=retreaver` and `sync_path='api_sync'` **and not** `apisync-%` | 0 | 0 | none. The 2 Retreaver `api_sync` rows are `apisync-%` |
| Unclassified besides old smoke | leftover after the families above | 0 | 0 | — |

`sync_path=''` is **exactly** the `demo-%` fixture: 53 empty-and-demo, 0 empty-and-not-demo, 0 demo-with-a-path. `[PROVEN]`

### Provenance of old smoke

Every leftover row is `sync_path='webhook'`, a bare UUID, on Ringba / Retreaver / TrackDrive / CallGrid, with the smoke scripts’ states and amounts (TX $12.50, WA $7.25/$42.50, CA $9.75/$5.50, FL $15 buyer_converted, NY $11.25). Several ids match this session’s pre-prefix gate runs (for example `1906f701-…`, `f2b1d903-…`). `[PROVEN]` they are script output **for this database today**.

| Candidate | Separates old smoke from a real call? | False positive |
|---|---|---|
| `sync_path='webhook'` | No. A real call posted to a dev webhook token is stored the same way. There is no second origin column. | **Yes** — any real webhook call |
| `sync_path=''` | No. On this DB it is the demo fixture, not smoke. | Would delete fixtures if used as “old smoke” |
| UUID shape | No. Real Retreaver API-sync ids are also UUIDs. None of those non-`apisync` rows exist here, but the shape is not safe. | **Yes** |
| Dev webhook token | Not stored on the ping row. Cannot be filtered in ClickHouse. | n/a |

No stored field cleanly separates “script” from “real webhook call” without using the id list we already observed. That is an acceptable answer. The **safe** delete is the explicit id list (14 ids, 15 rows). A `sync_path='webhook' AND NOT LIKE 'smoke-%'` predicate matches those same 15 rows **right now**, and is not safe as a standing rule.

### Proposed statements — not executed

Would remove — old smoke, exact ids, **15 rows**:

```sql
ALTER TABLE default.pings DELETE WHERE external_call_id IN (
  '0e92eece-73fb-4d06-b9c7-858862aca994',
  'ea023ec0-927b-43b9-af44-8befd5afe113',
  'ba472c0e-3212-4013-a760-c197b3877617',
  'cb51ea8b-6234-41cc-8697-3aad6729381a',
  'dbbdef75-8c22-4b08-bd82-5bdbb1bd08fe',
  '128fb18f-3738-4ddc-9b51-dd367b302778',
  '1906f701-2bdb-4b56-9f9c-f5dca66334af',
  '195c1e62-d264-4d6f-a9e1-a28347152e3f',
  '7afabab6-90d8-4a14-a255-4b648ded24b1',
  'cb45766e-04d7-4510-b117-075bbcb25305',
  'db04ac18-cce8-44de-ae41-1b9b896c8856',
  'f2b1d903-b545-4b14-8df6-dc6154060154',
  '1f149b5f-b772-4216-b397-fd7d8e65f920',
  'b9ff1104-aa1f-4abd-9a7a-00f69f68e240'
);
```

Would remove — synthetic mock rows, **3 rows** (already excluded from KPIs):

```sql
ALTER TABLE default.pings DELETE WHERE external_call_id LIKE 'apisync-%';
```

Would **not** remove:

- `demo-%` (53 rows) — spec decision is keep and exclude at read time
- `smoke-%` (10 rows) — correct prefix, already excluded
- `RGB%` + `sync_path='api_sync'` (4100 rows) — real Ringba

**Nothing above was run.**

---

## Final verification

| Script | Exit | Notes |
|---|---|---|
| `scripts/verify-system.ps1` | **0** | Docker 15/15, Healthz 11/11 |
| `scripts/verify-ingestion-slice.ps1` | **0** | two-line proof above |
| `scripts/verify-presentation.ps1` | **0** | `demo-call-1` present |
| `scripts/verify-api-sync.ps1` | **0** | not part of the gate |

---

## Stop

Parts A–C are done. Phase 2 has not been started. Deletions are waiting on an explicit approval of the statements above.
