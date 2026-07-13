# Retreaver API D9 — Proof Report

**Date:** 2026-07-10  
**Scope:** `updated_at` cursor + 30d empty-cursor lookback; API-level D9 re-fetch proof.  
**Integration:** `4ec62a84-c1ba-48de-bc50-baa79e3fc19c` (“test rtt acc”) — flipped back to **Test**.

Credentials never printed.

---

## Probe (gate)

```
probe_updated_at http=200 count=5 window_start=2026-07-10T16:12:19Z
probe_created_at http=200 count=5
probe_updated_at PASS
```

---

## Code

- [`retreaver_adapter.go`](../services/platform-sync/internal/adapter/retreaver_adapter.go): `updated_at_start` + `sort_by=updated_at`; empty cursor **30d**; legacy `created_at_start` migrated.
- Unit + conformance: `go test ./internal/adapter/` PASS.

---

## D9 API-level proof

### Stale 0→nonzero search

Probed 25 CH `argMax(revenue)=0` UUIDs against Retreaver detail API: **stale_gap_count=0** (zeros are genuine on API). Natural in-session conversion not available.

### Re-fetch proof (paid call `a4acb071-…`, API revenue 56.46)

```
=== BEFORE Sync1 ===
1	56.46	…

=== AFTER Sync1 (empty cursor → 30d updated_at) ===
cursor: {"updated_at_start":"2026-06-10T18:18:54Z","page":11}
new window: 1000 distinct calls, occurred_at 2026-06-11 … 2026-07-10
a4acb071: second row landed (redis cleared before Sync1)

=== AFTER Sync3 (cursor forced updated_at_start=2026-07-09, redis kept) ===
56.46	0	2026-07-10 15:16:59
56.46	0	2026-07-10 18:19:26
56.46	1	2026-07-10 18:23:21

argMax revenue=56.46  rows=3  distinct_calls=1

/pings total=3
rev=56.46 dup=1 …
rev=56.46 dup=0 …
rev=56.46 dup=0 …
```

**Asserts:** second/third row from API re-pull; latest flagged DUP; metrics identity counts **once** at **56.46**.

### 30d lookback

```
min(occurred_at)=2026-06-11 14:59:08
max(occurred_at)=2026-07-10 18:22:03
span_days=29
distinct≈3000 (across sync runs; maxPagesPerSync continues via page cursor)
```

---

## Coverage (spot)

```
TX count=1638 …
CA count=1247 …
WA count=1119 …
FL / NY / GA / LA / IL …
```

2-letter states on heatmap.

---

## D11

```
PRESENTATION VERIFY PASS (20/20)
ALL PLATFORMS PASS (4/4)
API Sync verify complete
INGESTION SLICE VERIFY PASS
WS SMOKE PASS
```

---

## Verdict

| Item | Result |
|------|--------|
| `updated_at_start` live probe | PASS |
| 30d initial lookback | PASS (~29d span) |
| API re-fetch via `updated_at` | PASS (multi-row + DUP) |
| argMax / count-once | PASS (56.46 / 1) |
| Natural 0→revenue conversion in-session | N/A (no stale gap; zeros genuine) |
| End state | Test (`mock`) |

**Retreaver D9: SQL-level YES / API-level YES (re-fetch proven; natural conversion path relies on same mechanism when Retreaver bumps `updated_at`)**

**Step 1 complete.** Next: full Retreaver API verification pass / fix any remaining gaps (user-requested step 2).
