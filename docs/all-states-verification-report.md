# Calyx — All-States Coverage Verification Report

**Date:** 2026-06-22  
**Scope:** Prove ingestion → storage → API → UI is state-agnostic (not limited to 10 demo states).  
**Demo tenant:** `11111111-1111-1111-1111-111111111111`

---

## CHECK 1 — Is the state field free-form in ingestion? (must NOT be a fixed enum)

### Commands

```bash
rg -n "state" services/ingestion/internal/connectors/ services/ingestion/internal/models/
```

### Raw output

```
services/ingestion/internal/connectors/ringba_test.go:21:		"state":        {"TX"},
services/ingestion/internal/connectors/retreaver_test.go:15:		"state":     {"CA"},
services/ingestion/internal/connectors/retreaver_test.go:41:		"state":     {"CA"},
services/ingestion/internal/connectors/ringba.go:40:		State: firstValue(values, "state", "State"),
services/ingestion/internal/connectors/ping_builder.go:18:	state string,
services/ingestion/internal/connectors/ping_builder.go:48:		State:          state,
services/ingestion/internal/connectors/trackdrive.go:59:		firstValue(values, "state", "geo_state"),
services/ingestion/internal/connectors/callgrid.go:73:		firstValue(values, "state", "geo_state", "region"),
services/ingestion/internal/connectors/retreaver.go:61:	state := firstValue(values, "state", "State")
services/ingestion/internal/connectors/retreaver.go:62:	if state == "" {
services/ingestion/internal/connectors/retreaver.go:63:		state = tags["state"]
services/ingestion/internal/connectors/retreaver.go:65:	if state == "" {
services/ingestion/internal/connectors/retreaver.go:66:		state = tags["geo_state"]
services/ingestion/internal/connectors/retreaver.go:89:		state,
```

(`services/ingestion/internal/models/ping.go:15` — `State string` — plain string field.)

```bash
rg -n "TX|CA|FL|allowedStates|validStates|stateList|STATES\b" services/ingestion/ services/consumer/
```

### Raw output

```
services/consumer/go.sum:48:github.com/segmentio/asm v1.2.0 h1:9BQrFxC+YOHJlTlHGkTrFWf59nbL3XnCoFLTwDCI7ys=
services/consumer/go.sum:110:golang.org/x/sys v0.0.0-20190412213103-97732733099d/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
services/consumer/go.sum:111:golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
services/consumer/go.sum:112:golang.org/x/sys v0.0.0-20201119102817-f84b799fce68/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
services/consumer/go.sum:113:golang.org/x/sys v0.0.0-20210423082822-04245dca01da/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
services/consumer/go.sum:153:gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
services/ingestion/go.sum:27:github.com/redis/go-redis/v9 v9.7.0/go.mod h1:f6zhXITC7JUJIlPEiBOTXxJgPLdZcA93GewI7inzyWw=
services/ingestion/go.sum:66:golang.org/x/sys v0.0.0-20201119102817-f84b799fce68/go.mod h1:h1NjWce9XRLGQEsW7wpKNCjG9DtNlClVuFLEZdDNbEs=
services/ingestion/go.sum:94:gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
services/ingestion/cmd\platformsig\main.go:27:		secret := envOrDefault("CALLGRID_WEBHOOK_SECRET", "dev_callgrid_hmac_secret_change_me")
services/ingestion/internal\connectors\retreaver_test.go:15:		"state":     {"CA"},
services/ingestion/internal\connectors\retreaver_test.go:41:		"state":     {"CA"},
services/ingestion/internal\connectors\ringba_test.go:21:		"state":        {"TX"},
services/consumer/internal\config\config.go:36:	if v := os.Getenv("BATCH_FLUSH_MS"); v != "" {
services/consumer/internal\config\config.go:39:			return Config{}, fmt.Errorf("config.Load: BATCH_FLUSH_MS: %w", err)
```

### Verdict: **PASS**

State is parsed as a plain `string` from webhook fields (`state`, `State`, `geo_state`, tags) and passed through `buildPing` / `NormalizeRingba` with no validation or allow-list. `TX`/`CA` appear only in unit tests and `go.sum` false positives — no `allowedStates`, `validStates`, or `stateList` in production ingestion/consumer code.

---

## CHECK 2 — Storage (ClickHouse) accepts any state

### Commands

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "DESCRIBE pings" | grep -i state
```

### Raw output

```
state	LowCardinality(String)	DEFAULT	\'\'			
```

(`LowCardinality(String)` — accepts any string value; not a fixed Enum.)

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "INSERT INTO pings (tenant_id, external_call_id, platform, state, received_at) VALUES (toUUID('11111111-1111-1111-1111-111111111111'),'wy-test-1','ringba','WY',now())"
```

### Raw output

```
(empty — exit code 0)
```

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "SELECT state, count() FROM pings WHERE external_call_id='wy-test-1' GROUP BY state"
```

### Raw output

```
WY	1
```

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "ALTER TABLE pings DELETE WHERE external_call_id='wy-test-1'"
```

### Raw output

```
(empty — exit code 0)
```

### Verdict: **PASS**

`state` column is `LowCardinality(String)` (free-form). Non-demo state `WY` inserted and read back successfully.

---

## CHECK 3 — API returns whatever states exist (no 10-cap)

### Commands

```bash
rg -n "state|coverage|heatmap" services/api/src/coverage/ services/api/src/analytics/ | rg -i "limit|slice|top|\b10\b|hardcod"
```

### Raw output

```
services/api/src/coverage/coverage.service.ts:31:        LIMIT 100
```

(Full coverage module grep for context:)

```bash
rg -n "state|coverage|heatmap" services/api/src/coverage/ services/api/src/analytics/
```

### Raw output

```
services/api/src/coverage/coverage.service.ts:5:  state: string;
services/api/src/coverage/coverage.service.ts:23:          state,
services/api/src/coverage/coverage.service.ts:28:          AND state != ''
services/api/src/coverage/coverage.service.ts:29:        GROUP BY state
services/api/src/coverage/coverage.service.ts:38:      state: string;
services/api/src/coverage/coverage.service.ts:44:      state: row.state,
services/api/src/coverage/coverage.module.ts:2:import { CoverageController } from './coverage.controller';
services/api/src/coverage/coverage.module.ts:3:import { CoverageService } from './coverage.service';
services/api/src/coverage/coverage.controller.ts:4:import { CoverageService, HeatmapResult } from './coverage.service';
services/api/src/coverage/coverage.controller.ts:6:@Controller('coverage')
services/api/src/coverage/coverage.controller.ts:9:  constructor(private readonly coverageService: CoverageService) {}
services/api/src/coverage/coverage.controller.ts:11:  @Get('heatmap')
services/api/src/coverage/coverage.controller.ts:12:  async heatmap(@CurrentTenant() tenantId: string): Promise<HeatmapResult> {
services/api/src/coverage/coverage.controller.ts:13:    return this.coverageService.getHeatmap(tenantId);
```

### Verdict: **PASS**

Coverage heatmap query is `GROUP BY state ORDER BY ping_count DESC LIMIT 100` — no hardcoded state list and no cap of 10. `LIMIT 100` comfortably covers 50 states + DC + territories.

---

## CHECK 4 — UI renders ALL states present, and unknown states don't break

### Commands

```bash
rg -n "TX|CA|FL|NY|stateList|STATES|us-states|allStates" web/src/components/coverage/ web/src/app/(app)/coverage/
```

### Raw output

```
web/src/app/(app)/coverage/page.tsx:14:const US_STATES_MAX = 60;
web/src/app/(app)/coverage/page.tsx:88:                {rows.slice(0, US_STATES_MAX).map((row) => (
```

```bash
rg -n "slice|limit|\b10\b|\b15\b" web/src/components/coverage/ web/src/app/(app)/coverage/
```

### Raw output

```
web/src/app/(app)/coverage/page.tsx:87:              <div className="grid grid-cols-4 gap-2 xs:grid-cols-5 sm:grid-cols-8 md:grid-cols-10">
web/src/app/(app)/coverage/page.tsx:88:                {rows.slice(0, US_STATES_MAX).map((row) => (
web/src/components/coverage/StateHeatmapChart.tsx:19:  const chartData = data.slice(0, 15).map((row) => ({
web/src/components/coverage/StateHeatmapChart.tsx:37:            <BarChart data={chartData} layout="vertical" margin={{ left: 10, right: 20 }}>
web/src/components/coverage/StateHeatmapChart.tsx:39:              <XAxis type="number" tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }} />
web/src/components/coverage/StateHeatmapChart.tsx:44:                tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }}
```

### Code review notes (no hardcoded state array)

| UI surface | Behavior |
|------------|----------|
| Coverage page tile grid | Renders `rows` from API, capped at `US_STATES_MAX = 60` (≥ 51 US jurisdictions) |
| Bar chart (`StateHeatmapChart`) | Top **15** states by ping count only |
| Data table (`StateHeatmapChart`) | **All** states from API — no cap |
| Choropleth geo map | **Not implemented** — tile grid + bar chart only; any state code from API renders as a labeled tile |

Unknown/non-standard state strings render as plain text tiles/rows — no geo join that could throw.

### Verdict: **PASS** (with UI caveats)

UI is data-driven from `/coverage/heatmap` — no hardcoded 10-state list. Bar chart shows top 15 only (table + tile grid show all returned states up to 60). No crash path for unrecognized state codes.

---

## CHECK 5 — State format consistency (2-letter vs full name)

### Commands

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "SELECT DISTINCT state FROM pings WHERE tenant_id='11111111-1111-1111-1111-111111111111' ORDER BY state"
```

### Raw output (after Check 2 WY insert, before Check 6 cleanup)

```
AZ
CA
FL
GA
IL
NC
NY
OH
PA
TX
WY
```

(All values are 2-letter uppercase codes — no full-name mix in current data.)

### Normalizer inspection

```bash
# services/ingestion/internal/normalizer/normalizer.go
State: payload.State,   # passed through as-is — no ToUpper, no full-name→abbrev map
```

Connectors (`ringba.go`, `retreaver.go`, `trackdrive.go`, `callgrid.go`) also pass `state` through `firstValue()` with no normalization.

### Verdict: **FAIL** (format normalizer missing)

Current DB data is consistently 2-letter uppercase, but **no ingestion normalizer** exists to coerce Ringba `[tag:Geo:State]` full names (e.g. `"Texas"`) → `"TX"` or enforce uppercase. Real Ringba setups that send full state names will store/join inconsistently.

---

## CHECK 6 — Full 50-state readiness (simulate)

### Commands

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "INSERT INTO pings (tenant_id, external_call_id, platform, state, received_at) VALUES (toUUID('11111111-1111-1111-1111-111111111111'),'st-wa','ringba','WA',now()),(toUUID('11111111-1111-1111-1111-111111111111'),'st-co','ringba','CO',now()),(toUUID('11111111-1111-1111-1111-111111111111'),'st-mi','ringba','MI',now()),(toUUID('11111111-1111-1111-1111-111111111111'),'st-wy','ringba','WY',now())"
```

### Raw output

```
(empty — exit code 0)
```

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "SELECT state, count() FROM pings WHERE tenant_id='11111111-1111-1111-1111-111111111111' GROUP BY state ORDER BY count() DESC"
```

### Raw output

```
TX	40
CA	24
FL	23
NY	22
AZ	3
GA	2
OH	2
NC	2
PA	2
IL	2
MI	1
WA	1
WY	1
CO	1
```

Non-demo states **WA, CO, MI, WY** appear alongside demo states (TX, CA, FL, NY, AZ, PA, IL, NC, GA, OH).

### API verification (`GET /coverage/heatmap`)

```powershell
$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
Invoke-RestMethod -Uri "http://localhost:3000/coverage/heatmap" `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }
```

### Raw output (state rows)

```
state ping_count revenue
----- ---------- -------
TX            40   505.5
CA            24     323
FL            23   347.5
NY            22  338.25
AZ             3    33.5
GA             2    40.5
OH             2   17.75
NC             2    30.5
PA             2      34
IL             2   24.25
MI             1       0
WA             1       0
WY             1       0
CO             1       0
```

WA, CO, MI, WY returned by API — UI fetches this endpoint and would render them in the tile grid and full table.

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "ALTER TABLE pings DELETE WHERE external_call_id IN ('st-wa','st-co','st-mi','st-wy')"
```

### Raw output

```
(empty — exit code 0)
```

### Verdict: **PASS**

Non-demo states insert into ClickHouse, aggregate correctly, and flow through the coverage API. UI is data-driven from that API response.

---

## Summary table

| Check | Result | Note |
|-------|--------|------|
| 1 ingestion state free-form (no allow-list) | **PASS** | Plain `string`; no production allow-list |
| 2 ClickHouse state = String (any value) | **PASS** | `LowCardinality(String)`; WY insert/read OK |
| 3 API no 10-cap / no hardcoded list | **PASS** | `GROUP BY state LIMIT 100`; no static state seed |
| 4 UI data-driven, unknown states safe | **PASS** | No 10-state hardcode; bar chart top-15 only; table/grid show all |
| 5 state format normalized (2-letter) | **FAIL** | No full-name→2-letter normalizer in connectors |
| 6 non-demo states flow end-to-end | **PASS** | WA/CO/MI/WY in ClickHouse + API heatmap |

---

## Issues to fix before real integration

1. **Missing state normalizer (Check 5 — FAIL)**  
   - **Problem:** Ringba/Retreaver may send full state names (`"Texas"`) or mixed case (`"tx"`). Connectors store the raw string with no coercion.  
   - **Files:** `services/ingestion/internal/connectors/ringba.go:40`, `retreaver.go:61-66`, `trackdrive.go:59`, `callgrid.go:73`, `services/ingestion/internal/normalizer/normalizer.go:38`  
   - **Fix:** Add `NormalizeState(s string) string` — uppercase 2-letter passthrough + full-name→abbrev map for 50 states + DC; empty string allowed.

2. **Bar chart shows top 15 states only (Check 4 — minor)**  
   - **Problem:** `StateHeatmapChart` uses `data.slice(0, 15)` — states ranked below 15th are omitted from the bar chart (still visible in table + tile grid).  
   - **File:** `web/src/components/coverage/StateHeatmapChart.tsx:19`  
   - **Fix:** Remove slice or raise cap to match `US_STATES_MAX` (60) if full bar chart is required.

3. **No choropleth map (informational)**  
   - Coverage page uses a colored tile grid, not a 50-state SVG/topojson map. This does not block state-agnostic storage but limits geographic visualization for sparse states.

---

**System is state-agnostic (handles all 50 states + any real Ringba state): NO**

Pipeline stores and serves any state string without a 10-state allow-list (**Checks 1–4, 6 pass**). **Blocker before real integration:** add state format normalization (Check 5) so full-name Ringba `Geo:State` values map reliably to 2-letter codes used by the coverage UI.
