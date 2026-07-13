# Phase 1.6 B5 — D9 API-level value-change (implementation / honesty report)

**Date:** 2026-07-13  
**Integration under test historically:** `4ec62a84-c1ba-48de-bc50-baa79e3fc19c` (“test rtt acc”) — **Test** (`sync_mode=mock`) during this write-up; no Live revenue-edit sync in Phase 1.6 session.  
**Credentials:** never printed.

---

## Why this report exists

Prior [`docs/retreaver-api-d9-proof-report.md`](retreaver-api-d9-proof-report.md) proved **re-fetch MECHANISM**:

- Same call, multiple ClickHouse rows.
- Latest flagged `is_duplicate=1`.
- `argMax(revenue)` counts **once**.

All three rows had revenue **56.46**. Report admitted natural **0→revenue conversion N/A**.

That is **not** a value-change proof. Phase 1.6 B5 must not upgrade it to `D9 API-level: YES`.

---

## Checklist results

### (1) Official docs — does revenue bump `updated_at`?

Source: https://retreaver.github.io/core-api-docs/

| Claim | Evidence |
|-------|----------|
| `updated_at_start` / `updated_at_end` exist | YES (throughput notes + query params) |
| `sort_by=updated_at` | YES (“If you only want recently updated Calls…”) |
| Explicit “revenue/conversion/payout change bumps `updated_at`” | **NO sentence found** |
| Samples with `conversions_determined_at` ≈ `updated_at` | Present — **suggestive, not a guarantee** |

**docs: UNVERIFIED** for revenue-specific bump semantics.

### (2) Live value-change on test account (0→nonzero or old→new)

| Step | Result |
|------|--------|
| Add conversion/payout in Retreaver UI | **Not performed** (no UI operator in this session) |
| Note `updated_at` + revenue before/after | N/A |
| Live Sync #1 → change → Sync #2 | N/A |
| Assert second CH row with **new** revenue; metrics `argMax`; count once; `/pings` both rows | N/A |

Prior stale-gap probe (D9 proof report): `stale_gap_count=0` (zeros on API matched CH).

### (3) Re-fetch path still trusted

| Evidence | Status |
|----------|--------|
| A2 live probe (`cmd/probe-retreaver-updated-at`) | `updated_at_cursor_VIABLE=true` |
| Prior multi-row re-fetch (56.46 × 3) | Mechanism PASS |
| B1 cursor = max seen `updated_at` − 15m | Implemented (supports future value-change pulls) |

**A2 raw (timestamps/revenue only):**

```
created_at=2026-07-06T16:00:13.475Z
updated_at=2026-07-06T16:18:01.106Z
revenue=21.544
probe_updated_at_start_before=… appears=true
probe_updated_at_start_after=… appears=false
probe_updated_at_end_includes appears=true
probe_updated_at_end_before_update appears=false
sort_by_updated_at … stable_repeat=true
updated_at_cursor_VIABLE=true
docs_note=revenue_bump_of_updated_at UNVERIFIED_in_docs
```

---

## Certification mapping (locked)

| Outcome condition | Label |
|-------------------|-------|
| Docs say revenue bumps `updated_at` **and** live old→new proven | `D9 API-level: YES` |
| Re-fetch proven; revenue bump not observed and/or docs silent | **`PARTIAL` — re-fetch proven, revenue-bump assumption UNVERIFIED** |
| Re-fetch path fails | `NO` |

**This session → PARTIAL.**

Written into [`docs/platform-certification.md`](platform-certification.md):

- D9 API column: **PARTIAL**
- Retreaver **CERTIFIED: NO** (do not paper over PARTIAL)

---

## What is implemented vs what is proven

| Layer | Implemented? | Proven end-to-end? |
|-------|--------------|-------------------|
| `updated_at` filter + sort | YES (adapter) | YES (A2) |
| Safety delta 15m + max-seen advance | YES (B1) | Unit tests YES; Live value-change N/A |
| Re-fetch → second CH row + argMax once | YES (pipeline) | YES (prior 56.46 proof — same revenue) |
| Revenue bump → new row → dashboard | Code path ready | **NO / UNVERIFIED** |

---

## How to finish to YES later (manual)

1. Pick a zero-revenue call UUID on the test Retreaver account.
2. Record API `revenue` + `updated_at` (via `verify-retreaver-call` / probe — no secrets).
3. In Retreaver UI, add conversion/payout so revenue becomes nonzero.
4. Re-fetch detail: confirm `updated_at` moved and revenue changed.
5. Flip integration **Live** → Sync #1 (baseline) → Sync #2 after change → back to **Test**.
6. Assert CH second row with new revenue; `/analytics/metrics` `argMax`; distinct count once; `/pings` shows DUP on newer row.
7. Update this report + certification to `D9 API-level: YES`.

---

## Checklist

| Item | Status |
|------|--------|
| Did not claim YES from 56.46×3 | ✅ |
| Docs revenue→`updated_at` cited honestly | ✅ UNVERIFIED |
| Live 0→nonzero | ⬜ not done |
| PARTIAL wording in certification | ✅ |
| Integration left on Test | ✅ |

**PASS/FAIL slice:** **PASS** for honest PARTIAL documentation. **FAIL** if anyone marks D9 API-level YES without a live value-change observation.
