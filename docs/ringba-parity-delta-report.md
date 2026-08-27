# Ringba parity — Part 2 Delta report (P2D1–P2D5)

**Date:** 2026-07-24  
**Scope:** Close out Ringba A+B1 proof gaps. No new parity fields.  
**Evidence:** `docs/evidence/ringba-parity-delta/`  
**Ringba integration:** Live only for remediating sync; returned to **Test** (`sync_mode=mock`).

---

## 1. Scoreboard

| Item | Result |
|------|--------|
| **P2D1** | **PASS** |
| **P2D2** | **PASS** |
| **P2D3** | **PASS** |
| **P2D4** | **PASS** |
| **P2D5** | **PASS** |

---

## 2. P2D1 mechanism (one sentence)

The headline “~538 without B1” is mostly **non-API** rows (`apisync-%` mock + smoke UUIDs); the **45 real `RGB*` calls** lacking B1 were **absent from the first forced-lookback assembled page set** (Redis TTLs were not refreshed at lookback time; zero of those IDs were published that hour) even though a targeted Get Call Log probe later found them — after targeted Redis key deletion + a second lookback, **RGB gap → 0**.

---

## 3. P2D2 cohort table (population-level)

Pre-republish cohort = Ringba `external_call_id` with `min(received_at) < 2026-07-23 18:19:00` UTC (forced-lookback start). Reconstructable from CH; not a saved snapshot file.

| Metric | Before | After |
|--------|--------|-------|
| `uniqExact(external_call_id)` | **933** | **933** |
| `sum(argMax(revenue))` | 9847.76 | 10096.36 |
| `sum(argMax(payout))` | 3408.94 | 3607.82 |
| raw `count()` rows | 999 | 1437 |

- **`uniqExact` held** (no call-count inflation).  
- Row growth is expected republish.  
- Revenue/payout rose only on **9** calls (all `0 → nonzero` late conversions).  
- Three verified against live Ringba API (match):

| Call | CH after | Ringba API |
|------|----------|------------|
| `RGBF9279…2E02` | rev 31.5 / pay 25.2 | `conversionAmount=31.50` `payoutAmount=25.20` |
| `RGB349D9…9F02` | rev 31.5 / pay 25.2 | same |
| `RGBB17EB…8902` | rev 25 / pay 20 | `conversionAmount=25.00` `payoutAmount=20.00` |

(Report’s earlier “931” vs reconstructed **933**: two additional Ringba rows landed in CH with `min(received_at)` still before lookback start; before revenue/payout totals match the Part 2 report exactly.)

---

## 4. Per item

### P2D1 — B1 backfill gap

#### Exact counts (30d, tenant demo)

| Family | uniq | pre-B1 gap (`sync_path=''` ∧ `campaign_name=''`) |
|--------|------|--------------------------------------------------|
| `RGB*` (real) | 2180 → after fix **2335** | **45 → 0** |
| `apisync-%` | 465 | 446 (mock; never from Ringba API) |
| other (smoke UUIDs etc.) | 51 | 47 |
| `webhook-parity-b14-demo` | 1 | 0 (had `sync_path=webhook`) |

#### Mechanism evidence

1. **Redis:** 20/20 sampled RGB gap IDs had dedup keys; TTL remaining ~80–124h of 168h → last write ~pre-lookback (e.g. ~2026-07-22), **not** refreshed at 2026-07-23T18:21 lookback (`Expire` on Unchanged would have reset TTL ≈168h from lookback).  
2. **Not published in lookback hour:** `gap_published_in_lookback = 0` for those IDs; 2133 `api_sync` rows that hour were other calls.  
3. **In Ringba API:** probe of the same 20 IDs over 2026-07-16→23 → **found=20/20** (1488 records, 2 pages).  
4. **Date cluster (RGB gap):** 2026-07-17 (8), 07-20 (23), 07-21 (8), 07-22 (6) — not a single ancient tail.  
5. **`unchanged=1201` on first lookback:** `published+unchanged=3334=calls`; consistent with page1_refresh re-evaluating already-published-in-tick calls as Unchanged — **not** proof those 1201 lacked B1 forever.

Hypothesis “Redis already had B1 hash so Unchanged is fine” is **disproved for the 45**: TTLs show pre-lookback writes and they never got `sync_path=api_sync` until remediation.

#### Remediation

```
rgb_gap_count=45
redis_keys_deleted=45   # only {tenant}:dedup:ringba:{id} for those IDs
# clear cursor + last_sync_at, sync_mode=real, restart platform-sync
api sync completed … calls=3297 published=200 unchanged=3097 page1_refresh=true
```

Post-fix: **`still_gap=0`** for `RGB*`. Example `RGB87B657F7…` gained row `2026-07-24T10:47:20Z` with `sync_path=api_sync`, `campaign_name=Final Expense`, `duration_seconds=748`.

#### Residual

| Residual | Count | Reason | Roadmap |
|----------|------:|--------|---------|
| `apisync-%` without B1 names | ~446 | Mock sync IDs — **never returned by Ringba API**; already KPI-excluded via `apiSyncExclusionSql` | Not a customer backfill item |
| Smoke UUID “other” | ~47 | Webhook/smoke test IDs | Naming convention (P2D3); not Ringba API backfill |

No remaining **real RGB** gap. Offset/multi-page assembly can miss IDs across runs (first lookback 3334 vs second 3297) — if recurrence appears, trigger: day-sized windows or support ticket on stable pagination; **not opened as a new code item in this delta**.

---

### P2D2 — No inflation (cohort)

See §3. Evidence → change (none; proof only) → proof: cohort queries + 3 API matches.

---

### P2D3 — Synthetic row + real webhook

**Remove:**

```sql
ALTER TABLE pings DELETE WHERE external_call_id = 'webhook-parity-b14-demo'
```

- Rows before: **1**; after mutation visible count: **0**.  
- Synthetic had `revenue=0` → KPI revenue delta **effectively zero**; uniq for that id removed (was uncounted-for by C3 filters).

**Naming convention (documented):** any hand/script synthetic row **must** use `apisync-` prefix so `apiSyncExclusionSql` excludes it. Recorded in `services/api/src/common/demo-rows.ts` file comment and this report. Do not invent prefixes like `webhook-parity-`.

**Real webhook path:**

```
ingest_http=200 callId=a46b67b8-d494-47d9-8ded-2b605624c553
CH: sync_path=webhook, campaign_name='', campaign_id='', duration_seconds=0,
    recording_known=0, publisher_name='', target_name='', buyer_name=''
```

Deep-view JSON: `docs/evidence/ringba-parity-delta/real-webhook-deep-view.json`  
UI contract: `sync_path=webhook` + empty B1 fields → *“Not provided via webhook — available in API mode”* (empty string, not SQL NULL; panel treats `''` / `0` / `recording_known=false` as missing for webhook).

---

### P2D4 — Full verification gate

Run log: `docs/evidence/ringba-parity-delta/p2d4-verify.txt`  
Clean shell (`GOOS`/`GOARCH` unset). Single end-to-end run — not component cherry-picks.

| Check | Exit | Notes |
|-------|-----:|-------|
| `verify-ingestion-slice.ps1` | **0** | INGESTION SLICE VERIFY PASS |
| `verify-system.ps1` | **0** | SYSTEM VERIFY PASS (Docker/health/API/routes/WS/units/web build/pipeline 4/4) |
| `ws-smoke-test.mjs` | **0** | 3 events: `ping_received`, `conversion_attributed`, `call_scored` |

`verify-api-sync.ps1` not run (roadmap H).

---

### P2D5 — Roadmap (docs only)

- **§G** rewritten: `recordingUrl` obtainable; **PII/policy** is the blocker.  
- **§J** added: Ringba converted-state unmapped — **blocking** before write-back.  
- Summary table updated. **No product code** in this item (comment-only on `demo-rows.ts` is P2D3).

---

## 5. Honest limitations

1. Cohort size **933** vs Part 2 narrative **931** — explained above; money before totals identical.  
2. First lookback miss of 45 RGB IDs is evidenced as “not in assembled set”; root Ringba pagination stability not instrumented beyond before/after call counts.  
3. Real webhook proof uses a UUID call id (counts in KPIs like other smokes); synthetics for UI-only fixtures must use `apisync-`.  
4. PNG screenshots not taken; deep-view JSON is the UI contract proof.

---

## 6. Found but not fixed

| Finding | Action |
|---------|--------|
| Offset/multi-page Get Call Log can omit IDs across runs | Report only; remediate by re-sync if gap recurs |
| ~446 `apisync-%` rows without B1 columns | Expected (mock); excluded from KPIs |
| `converted` still unmapped | Roadmap **J** (blocking for write-back) |
| `verify-api-sync.ps1` stale | Roadmap H |
