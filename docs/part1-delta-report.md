# Part 1 Delta Report (D1–D4)

**Date:** 2026-07-23  
**Deliverable:** closes Part 1 before Part 2 (Ringba parity A + B1).  
**Proof surface:** rebuilt Docker `api` / `enrichment` / `web` unless noted.

---

## Summary scoreboard

| Item | Result |
|------|--------|
| D1 — Duplicate enrichment attributes | **PASS** |
| D2 — Three-script verification gate | **FAIL** |
| D3 — `demoExclusionSql` naming / fixture scope | **PASS** |
| D4 — Deactivate `api-sync-verify-220542` | **PASS** |

---

## D1.2 — 🚨 Republish → re-insert? (BLOCKER for Part 2)

# **YES — before this delta. NO — after the write-path guard shipped in this delta.**

### Path (evidence)

```
platform-sync PublishChanged
  → Kafka topic callloom.pings.v1  (same topic enrichment consumes)
  → enrichment consumer handleMessage
  → Redis IsCached(tenant, ani_hash)?  TTL = ENRICHMENT_CACHE_TTL_HOURS (24h)
       cache hit  → skip
       cache miss → provider.Enrich → InsertAttributes (full 520-row batch)
```

- Writer: `services/enrichment/pkg/clickhouse/clickhouse.go` `InsertAttributes` (batch INSERT; **no** dedup on write historically).
- Guard was **only** Redis TTL (`services/enrichment/internal/cache/cache.go`). Redis uses `volatile-ttl` — keys are eviction-eligible.
- Table: **MergeTree** (not ReplacingMergeTree) — see DDL below. Re-inserts accumulate forever.

### Historical proof (CallGrid call)

`ani_hash=d6736136…` — ten insert batches of 520 rows at different minutes (ratio 10):

```
2026-07-02 … 520
2026-07-03 … 520
…
2026-07-23 … 520
→ total_rows=5200, unique_keys=520
```

Cadence ≈ daily → matches 24h cache expiry + re-consume.

### Post-fix proof (deployed enrichment)

Cleared Redis key for that `ani_hash`, produced a ping onto `callloom.pings.v1`:

```
rows_before=5200
rows_after=5200
log: "enrichment skip existing attrs" … ani_hash=d6736136
```

**Part 2 may proceed** only with this enrichment image deployed (CH existence check before insert). Mass content-hash republish will still republish pings; enrichment will **not** multiply attribute rows for ANIs that already have rows.

---

## D1 — Duplicate enrichment attribute rows

### D1.1 Investigation

**Call:** `/calls/999370e1-b9e3-4cd0-8258-dc72d07b548f` (CallGrid)  
**ani_hash:** `d6736136ea896c1bfdc553e0e86e702c70d060d805696ca3e4e9e0961353860a`

| total_rows | unique_keys | ratio |
|------------|-------------|-------|
| 5200 | 520 | **10** |

**Distribution across all `ani_hash` (tenant demo):**

| hashes ratio=1 | hashes ratio>1 | max_ratio | avg_ratio | total hashes |
|----------------|----------------|-----------|-----------|--------------|
| 3668 | **2484** | **14** | ~1.45 | 6152 |

**Identical vs versioning:** per-key `uniqExact(attribute_value)=1` with `row_count=10` → **row-identical duplicates**, not genuine versioning.

**DDL:**

```sql
ENGINE = MergeTree
ORDER BY (tenant_id, ani_hash, attribute_key)
-- NOT ReplacingMergeTree; no unique constraint
```

**Existing duplicate volume (do not delete in this slice):**

| Metric | Value |
|--------|-------|
| Excess duplicate rows (`sum(cnt-1)` where cnt>1) | **~1,432,600** |
| `ani_hash` values with dups | **2484** |

**Cleanup proposal (not executed):** ClickHouse `OPTIMIZE`/`DELETE`/`ReplacingMergeTree` migration — approved ops slice; prefer `GROUP BY attribute_key` reads until cleaned.

### D1.3 Fix

| Choice | Why |
|--------|-----|
| Displayed count = **`uniqExact(attribute_key)`** | Meaningful attribute set size; raw `count()` is inflated |
| Table = **`GROUP BY attribute_key` + `argMax(..., created_at)`** | No duplicate rows in UI |
| Write path = **`HasAttributes` then skip insert** | Stops unbounded growth on republish / cache miss |

Files:

- `services/enrichment/pkg/clickhouse/clickhouse.go` — `HasAttributes`
- `services/enrichment/internal/consumer/consumer.go` — skip when exists
- `services/api/src/enrichment/enrichment.service.ts` — unique count + grouped select
- `services/api/src/native-integration/deep-query.service.ts` — same for deep-view

### Proof (rebuilt Docker API)

```
GET /enrichment/{ani}?limit=500 → data=500 total=520  (dup_keys_in_payload=0)
GET /native/call-deep-view?callId=999370e1-… → enrichment.attribute_count=520
```

Screenshot: open item for authenticated pixel capture; API + deployed behaviour proven above. Hard-refresh `:3001/calls/999370e1-…`.

### DoD D1

- [x] Ratio query CallGrid  
- [x] Distribution across hashes  
- [x] Row-identical confirmed  
- [x] Writer + DDL  
- [x] Republish→re-insert answered (**YES historically / NO after guard**)  
- [x] Count + table unique  
- [x] Write path addressed  
- [x] Duplicate volume reported; cleanup proposed not executed  

---

## D2 — Verification gate

### Raw results

| Script | Exit | Notes |
|--------|------|-------|
| `verify-ingestion-slice.ps1` | **0 PASS** | Dedup-correct aggregates still hold after C3 (`6854→6855` analytics; coverage `6843→6844`; content-hash suppress) |
| `verify-api-sync.ps1` | **1 FAIL** | Sync created + triggered; metrics stayed `6856→6856` because **C3 excludes `apisync-%` from metrics**. With `show_demo=true`, metrics = **10046** vs default **6859** — sync still lands data. **Script contract is stale; not weakened.** |
| `verify-system.ps1` | **1 FAIL** (×2 full runs) | Docker/Healthz/API/routes/unit/pipeline/web-build **PASS**. **WebSocket: 0 events / 45s** (attempt1, attempt2, and standalone `ws-smoke-test.mjs`). Not the prior `.next` rename race (web build PASS). |

Logs: `docs/verify-logs/verify-ingestion-slice.txt`, `verify-api-sync.txt`, `verify-system-attempt1.txt`, `verify-system-attempt2.txt`.

### Correct contracts (do not bend product to satisfy scripts)

1. **`verify-api-sync`:** assert growth via `?show_demo=true` or `/pings?show_demo=true` apisync count; soft-delete created integration (roadmap **H**).
2. **WS smoke:** Kafka live consumer → gateway path needs investigation (roadmap **I**). Outside this delta’s product fixes.

### DoD D2

- [x] All three scripts run  
- [x] ingestion-slice PASS  
- [ ] api-sync PASS — **FAIL** (stale assertion)  
- [ ] verify-system exit 0 — **FAIL** (WS; 2 full retries + 1 standalone)  

---

## D3 — Fixture rows vs `apisync-%`

### Fixture contribution (CH, distinct calls)

| Family | Distinct calls | Revenue (non-duplicate) |
|--------|----------------|-------------------------|
| `apisync-%` | 3167 | $35,628.75 |
| `demo-%` (fixture) | **53** | **$802.75** |
| neither | 6800 | $48,144.52 |
| Fixture exposure rows | 12 | $145.20 |

### Options costed

| Option | Action | Effect on headline KPIs (approx) |
|--------|--------|----------------------------------|
| **A — keep fixture** | Rename helper; document fixture intentional | KPIs unchanged (~6873 pings / ~$49.2k) |
| B — exclude `demo-%` too | Extend predicate | Pings ≈ **6820**; revenue ≈ **$48.3k**; exposure ≈ **−$145** |

### Decision: **Option A**

Cold-start dashboard legitimacy; fixture is **~0.8%** of distinct calls. Helper renamed to **`apiSyncExclusionSql`** (file still `demo-rows.ts`; deprecated alias `demoExclusionSql` kept). Comments state fixture `demo-call-*` / `demo-callloom-*` remain counted.

**Publishers card note:** MediaCo / LeadStream / DialPro / CallStream come from hardcoded `web/src/lib/demo-publishers.ts` (`DEMO_PUBLISHERS`), **not** from ClickHouse fixture publisher IDs. Separate from D3 filter scope (found, not fixed).

### DoD D3

- [x] Fixture count + KPI contribution  
- [x] Both options costed  
- [x] Option A implemented; name matches behaviour  
- [x] Documented here  

---

## D4 — Deactivate verify leftover

| Integration | Before | After |
|-------------|--------|-------|
| `api-sync-verify-220542` | `is_active=t` | **`is_active=f`** (API soft-delete 2026-07-23 16:01:39Z) |
| `api-sync-verify-211307` (created by D2 script run) | `is_active=t` | **`is_active=f`** (SQL soft-delete; script does not clean up) |

### Growth sample

| Time | `count()` raw `apisync-%` ping rows |
|------|-------------------------------------|
| t0 deactivate 220542 | **3173** (21:01:39 +05) |
| t1 +2 min | **3177** |
| later (after other mock syncs) | **3199** |

Growth **did not fully stop** — remaining **active mock** integrations: `Test 1` (retreaver), `Real R. Test` (ringba). Out of scope to deactivate those unless requested. All `api-sync-verify-*` rows are inactive.

### Script cleanup

`scripts/verify-api-sync.ps1` **does not** deactivate/delete the integration it creates. Follow-up: roadmap **H**.

### DoD D4

- [x] 220542 deactivated (not deleted); PG `is_active=f`  
- [x] Before/after counts with timestamps (residual growth explained)  
- [x] Script cleanup behaviour reported  

---

## Honest limitations

1. Authenticated UI screenshots not captured; rely on Docker API proofs + hard-refresh.
2. D2 gate red on api-sync assertion + WS smoke — product C3/D1 fixes are not rolled back to green-wash scripts.
3. Existing ~1.4M excess enrichment rows remain until an approved cleanup.
4. `Test 1` / `Real R. Test` still mock-sync → `apisync-%` table growth continues at lower rate.

---

## Found but not fixed

- Remaining active mock API-sync integrations (`Test 1`, `Real R. Test`)
- Publishers card still hardcoded demo names
- WS live smoke 0 events (roadmap I)
- `verify-api-sync.ps1` stale metrics assertion + no teardown (roadmap H)
- Enrichment duplicate row cleanup (propose only)
- Deferred list → `docs/enterprise-roadmap.md` sections **C–I**

---

## Part 2 readiness

| Gate | Status |
|------|--------|
| D1.2 republish multiplies enrichment? | **Mitigated** — CH existence skip in enrichment consumer |
| Part 1 delta report | This file |
| Start Part 2 (Ringba A + B1)? | **Unblocked on D1.2** — recommend accepting this report first; D2 script failures are tracking items, not enrichment blockers |
