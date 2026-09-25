# Calyx — S1 Part A: Ringba Sync Window, Sweep, Hash-After-Publish, Backfill

**Scope:** stop the Ringba sync from permanently losing calls and missing late updates,
then recover what was already lost. **No other S1 items. No Phase 2B. No Retreaver. No
push.**

**Deliverable:** `docs/s1a-sync-window-report.md`

---

## ⛔ RULE

- Do exactly this. If a step is ambiguous or the code differs from this document, STOP
  and report — do not infer.
- **Do not wait real hours.** Time-dependent behaviour is tested with fake clocks (unit)
  and moved timestamps (live). A test that "waits 6 hours" is not acceptable.
- Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt images.
- **Permitted operator-level state changes:** moving `last_sync_at` for `ce6fd338`
  (A6.2); triggering a sweep / backfill (A6.3, A7). Nothing else — no deletes, no config
  outside this scope, no Ringba writes.

---

## Why — proven in S0

| Finding | Evidence |
|---|---|
| **F003(b)** Downtime > 2h permanently loses calls | Laptop restart, sync down 2h51m: 5 Ringba calls in the critical sub-gap, **0 in ClickHouse**, still missing. The first tick after downtime fetches the **stale stored window** `[last−2h, last]`, then saves `[now−2h, now]` — the middle is never fetched. |
| **F003(c)** Updates after the window closes are missed | `RGB7CE620…` (14,399s call): ClickHouse `has_converted=0`, Ringba `hasConverted=true`. Max call duration in 30 days: **13,994s ≈ 3.9h** — longer than the 2h overlap. |
| Structural | Ringba's Get Call Log filters by `callDt` (call **start**) only — no "updated since" filter. Offline conversions, payout adjustments and voids made hours or days later can never be caught by any incremental window alone. |
| **F029** Hash written before publish | `pingdedup` stores the content hash **before** Kafka publish. A failed publish leaves "unchanged" recorded → never retried → silent loss. Must be fixed **before** the backfill in this file, or the backfill itself can lose data. |
| **F068** | `WasPayoutAdjusted` mapped but not in the content hash — a sweep would not detect payout-only adjustments. |

---
---

## A1 — Incremental window: stop trusting the stored window

In the Ringba adapter, **each scheduled tick fetches**:

```
[ last_sync_at − overlap ,  now ]
```

- The stored `sync_cursor` window must **no longer override** these bounds. Report what the
  cursor is still used for after the change, if anything.
- `overlap` configurable (e.g. `RINGBA_INCREMENTAL_OVERLAP`), **default 6h**. Justify
  against the observed maximum call duration (13,994s) with margin.
- **`last_sync_at` semantics:** set it to the **window end actually fetched** (the `now`
  captured at fetch start), **only after a fully successful run** — never after a failure,
  never to a time later than what was fetched. Report the current behaviour first, then
  the change.
- **Long-downtime cap:** if `last_sync_at − overlap` is older than the integration's
  lookback (`initial_lookback_days`), cap the window at the lookback **and log it with a
  distinct message + counter** (`window_capped`). A capped window means data older than
  the lookback may be missing — it must be visible, not silent.
- **Log the requested window** (start, end) on every tick. S0 found this was not logged.

## A2 — Sweep: catch late updates

A second, slower pass per Ringba integration:

- Every `RINGBA_SWEEP_INTERVAL` (default **60 min**), re-fetch
  `[ now − RINGBA_SWEEP_DAYS , now ]` (default **7 days**) through the existing windowed
  path.
- Uses the **same single-flight lock** as ticks — a sweep and a tick never overlap.
- Content-hash dedup means only genuinely changed calls are republished.
- Log per sweep: window, requests, `calls`, `published`, `unchanged`, duration.
- **Manual trigger** for testing, callable only with the existing internal bearer token
  (do not add another unauthenticated route). Must accept an explicit range, so the same
  mechanism can run the A7 backfill.

### A2.1 — 🔴 Hash TTL vs sweep range (check before relying on the sweep)

Dedup hash TTL is `DEDUP_TTL_HOURS=168` (7 days) — the **same length** as the sweep range.
If a hash expires before the sweep stops covering that call, the sweep sees it as new and
republishes it — **every hour**, for every call near the 7-day edge.

Report:
- Is the hash TTL refreshed when a call is re-seen as unchanged?
- If not, the hash TTL must exceed the sweep range with margin (e.g. sweep 7d → TTL ≥ 10d).
  Choose and justify.

**Proof required in A6:** two consecutive sweeps with no real-world change → the second
publishes **~0**.

## A3 — F029: write the content hash only after a successful publish

- Store / update the dedup hash **only after** Kafka confirms the publish.
- If publish fails, the hash is not recorded, so the next tick or sweep retries the call.
- Keep single-flight in mind: no race where two writers both publish. Use an atomic
  compare-and-set if needed (F056 context).
- Unit test: publish fails → hash absent → next run publishes. Publish succeeds → hash
  present → next run reports unchanged.

## A4 — F068: payout adjustments in the hash

Add `WasPayoutAdjusted` to the canonical content (and producer/consumer if missing), so a
payout-only adjustment triggers republish. This changes every hash → one-time republish,
absorbed by the A7 backfill.

## A5 — Unit tests (fake clock)

| Case | Expected window |
|---|---|
| Normal tick, `last_sync_at` = 2 min ago | `[now−2m−6h, now]` |
| Previous run **failed** | `last_sync_at` unchanged → next window still starts at old `last_sync_at − 6h` |
| Downtime 8h | `[now−8h−6h, now]` — nothing between old `last_sync_at` and `now` skipped |
| Downtime longer than lookback | capped at lookback, `window_capped` logged + counted |
| Stored cursor present with an old window | **ignored** for bounds |
| Sweep | `[now−7d, now]` |

## A6 — Live proof (no real waiting)

### A6.1 — Normal ticks
Deploy. Show 3 consecutive ticks' logged windows; each end ≈ that tick's `now`, start ≈
`last_sync_at − 6h`.

### A6.2 — Simulated downtime
With the operator's permission (granted in this file): set `ce6fd338` `last_sync_at` to
**8 hours** before now. Let the next tick run. Report the requested window and reconcile
**that 8h range**: Ringba API distinct vs ClickHouse distinct (by `occurred_at`) — must be
equal. Report any missing ids.

### A6.3 — Sweep
Trigger one sweep manually (7 days). Report its log line. Trigger a **second** sweep
immediately — it must publish **~0**. If not, report which calls republished and why
(A2.1).

## A7 — Backfill and recover what was lost

Using the manual trigger, run **one 30-day range** through the windowed path.

### A7.1 — Call-level reconciliation (30 days, closed window)
| | Count |
|---|---|
| Ringba API distinct | |
| ClickHouse distinct | |
| In Ringba, not in CH | **must be 0** |
| In CH, not in Ringba | explain any |

Confirm the **5 ids from S0 V1.4** are now present:
`RGB02246DCF…RGBDC02`, `RGB5090F802…RGB8602`, `RGBA47E8638…RGB3502`,
`RGB782856E9…RGBBD02`, `RGB31DF11E9…RGBE302`.

### A7.2 — Field-level reconciliation
For every call in the 30-day window, compare ClickHouse latest (argMax) against Ringba now:
`has_converted`, conversion amount, payout, `was_conversion_adjusted`,
`was_payout_adjusted`.

| Field | Mismatches |
|---|---|

**Target: 0.** `RGB7CE620…RGB8802` must now show `has_converted=1`. List every remaining
mismatch with ids — a mismatch here means updates are still being missed.

### A7.3 — Enrichment
Row count before / after the backfill republish (ReplacingMergeTree should converge).

## A8 — Gates, commit, scan

- All three gates exit 0 with sync live.
- Commit locally. Secret scan of every commit since the last push (same patterns as
  before — show them). **Do not push.** No hash-only follow-up commit.

## Output — `docs/s1a-sync-window-report.md`
1. Scoreboard A1–A8: PASS / FAIL.
2. Before/after code path for the window, with `file:line`.
3. A2.1 decision (TTL vs sweep) with reasoning.
4. A6 and A7 tables with raw evidence.
5. Honest limitations; anything found but not fixed.

STOP. Do not start S1 Part B. Do not push.

## Out of scope
S1 Part B (F015, F013, F014, F016, restart policies, API boot without Kafka, F050, F028,
F018, F019) · Retreaver sync and paging · Phase 2B · UI · any deletion · any Ringba write.
