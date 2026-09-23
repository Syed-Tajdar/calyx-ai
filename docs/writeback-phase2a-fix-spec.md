# Calyx — Phase 2A-fix: Stable Ringba Pagination + Per-Call Required Duration

**Scope:** the Ringba sync must stop silently skipping calls, and must capture the per-call
conversion requirement Ringba already sends. **No Phase 2B. No UI. No status logic. No
deletions. No write to Ringba.**

**Deliverable:** `docs/writeback-phase2a-fix-report.md`

---

## ⛔ RULE

Do exactly this. If a step is ambiguous or reality differs from this document, STOP and
report — do not infer intent. Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`.
Evidence from rebuilt Docker images.

---

## Why

**1. The sync silently skips calls.** `[PROVEN]` across Phase 2A and the probe: repeated
walks of the same 30-day window return the **same row count** (4,025 / 4,043) but
**different distinct-id sets** (3,384 / 3,563; 3,709 / 3,400 / 3,598). 431 calls that exist
on Ringba were never republished. Nothing errored and nothing logged a warning. Offset
pagination over a live, changing result set is the cause class — the same failure found
earlier with 45 calls.

**2. Ringba sends each call's own conversion requirement.** `[PROVEN]` probe:
`ringTreeWinningBidDynamicDuration` (title *Winning Bid Dynamic Duration*) was present on
3,315 of 4,043 rows, and **all 696 converted calls** had connected duration ≥ that value —
zero exceptions. On one target the value varies per call (90, 100, 120, 10, 60 …), so no
per-target or global threshold can replace it. It is absent on 728 rows (non-ring-tree).

`GET /targets` exposes **no** conversion settings, and the targets on the calls (`PI…` ids)
are not in that list. `[PROVEN]` — so this per-call field is the only authentic source.

---
---

# PART 1 — Remove offset paging from normal sync

## 1.1 — Windowed requests

Replace "one window + offset pages" with **time windows small enough to fit one page**.

- Split the requested range into sub-windows (start with one day).
- Request each sub-window with `size = 1000`, **no offset**.
- **If a sub-window returns exactly `size` rows, it may be truncated** — split it in half
  and request both halves. Repeat until every window returns fewer than `size` rows.
- Minimum window: 1 minute. If a 1-minute window still returns `size` rows, fall back to
  offset paging **for that window only**, and log it loudly with a count metric. Report
  every occurrence. Do not silently accept truncation.
- Keep the certified rate posture (5/sec, 80/min per account). Report request count for a
  full 30-day backfill.

## 1.2 — Existing behaviour to preserve or report

- The ongoing tick uses a trailing overlap window. Confirm it now uses the windowed path.
- `page1_refresh` was added for deep-page delays. **Report** how it interacts with windowed
  requests. Do not remove it without reporting why it is no longer needed.
- Deterministic ordering: check the current Ringba API documentation for a sort/order
  parameter on `POST /calllogs`. The saved Postman collection is no longer in the
  workspace — re-fetch it from the Postman Documenter JSON endpoint used earlier if needed.
  Use a sort only if documented; report either way.

## 1.3 — 🔴 Rows vs distinct calls — understand before trusting

`[PROVEN]` one walk returns more rows than distinct calls (4,043 rows / 3,598 ids).
Part of that may be the pagination drift above. But Ringba's own Revenue Recovery
documentation states rerouted calls produce **separate call logs for each target** — so
some calls may legitimately have **more than one row**.

With stable single-page windows, report:
- rows vs distinct `inboundCallId` per window
- for calls with more than one row: what differs between the rows (target, durations,
  revenue, `hasConverted`, timestamps)

**The risk this checks:** Calyx dedups on `(platform, external_call_id)`. If one call has two
legitimately different rows, the sync may publish both, flip the content hash back and
forth on every tick, and **republish forever** — with no error. See 3.4.

Report findings only. Do not change dedup identity in this phase.

## 1.4 — Retreaver (report only)

State whether the Retreaver adapter paginates with offsets over a changing set. **Do not
change it.** Retreaver sync stays off.

---
---

# PART 2 — Capture the per-call requirement

## 2.1 — Columns

Next free ClickHouse migration (`000014`), **with a `down`**, wired into compose init after
`000013`.

| Column | Source | Type |
|---|---|---|
| `required_duration_seconds` | `ringTreeWinningBidDynamicDuration` | `Nullable` numeric |
| `max_required_duration_seconds` | `ringTreeWinningBidMaxDynamicDuration` | `Nullable` numeric |

Why the second one now: every field added to the content hash forces a full republish.
Adding it now avoids a second republish later. It was present on only 81 rows; its meaning
is `[UNKNOWN]` and it is captured, not used.

## 2.2 — 🔴 NULL is not 0

This differs from the Phase 2A booleans. For these fields:

| Ringba record | Store |
|---|---|
| key absent | **NULL** — no requirement was sent |
| key present, value `0` | **0** — requirement present and zero |
| key present, value N | N |

`[PROVEN]` probe: one converted call carried a present value of `0`. Collapsing absent into
`0` would make every non-ring-tree call look like "no duration required" — every connected
call would then appear to have met its rule. Unit-test all three cases.

**Confirm the raw format.** The column type is `timespan`. The probe reported integers
(90, 100 …). Show the raw JSON for a few rows and confirm the unit is seconds.

## 2.3 — Mapper and hash

Add both fields to the adapter `valueColumns`, `RemoteCall`, the Kafka message, the
consumer insert, and the `pingdedup` canonical content.

---
---

# PART 3 — Backfill and prove stability

## 3.1 — Full backfill

After both changes are deployed: clear the Ringba integration's `sync_cursor` and
`last_sync_at` (same method as Phase 2A, reported), and let the 30-day backfill run through
the windowed path. Do not change `sync_mode`, `is_active`, or `writeback_enabled`.

## 3.2 — 🔴 Stability test — closed window

Pick a window that has **fully ended** (e.g. the 30 days ending one hour ago), so new calls
cannot drift the result.

Walk it **twice** through the new windowed method. Report:

| | Walk 1 | Walk 2 |
|---|---|---|
| rows | | |
| distinct `inboundCallId` | | |
| ids in walk 1 not in walk 2 | | — |
| ids in walk 2 not in walk 1 | — | |

**Pass condition: both difference counts are 0.** If not zero, the fix is not complete —
report and STOP.

## 3.3 — Reconciliation

For the same closed window:

| | Count |
|---|---|
| Ringba API distinct calls | |
| ClickHouse distinct Ringba calls | |
| In API, not in ClickHouse | |
| In ClickHouse, not in API | |

**Pass condition: both difference counts are 0.** Explain any non-zero with specific ids.
The 431 calls missed in Phase 2A should now be present — confirm.

## 3.4 — 🔴 Steady-state: no republish loop

After the backfill settles, let **two consecutive ticks** run with no real-world change to
old calls. Report `published` / `unchanged` for each tick.

Old calls must report **unchanged**. If the same calls are published again on every tick,
that is the hash ping-pong from 1.3 — report which calls and which fields differ. **Do not
fix it in this phase.**

---
---

# PART 4 — Verify the rule on stable data

Repeat the probe's decisive check on the **reconciled** ClickHouse data (latest row per
call), using **connected duration** (`duration_seconds`):

| | `required` present, duration ≥ required | `required` present, duration < required | `required` NULL |
|---|---|---|---|
| Converted (`has_converted` AND revenue > 0) | | | |
| Connected, not converted, not blocked, not adjusted | | | |

Report:
- **Converted calls below their requirement: expected 0.** Any exception listed by id.
- The "connected, not converted, duration ≥ required" calls — full list of ids. These are
  the true ⚠ Miss candidates. The probe found **1**.
- Calls where `required` is `0`: count and whether converted.
- The `required` NULL pool (no rule sent): count, converted vs not, duration distribution.
  This sizes the "rule unknown" group Phase 2B must present honestly.
- Repeat the check with whole-call length (`call_length_seconds`) and state which clock the
  field aligns with.

**Report the numbers only.** The status rule is decided after this report.

---

## Gates

Run and report raw output + exit codes, with sync live:
```
scripts/verify-system.ps1            → exit 0, Docker 15/15, Healthz 11/11
scripts/verify-ingestion-slice.ps1   → exit 0
scripts/verify-presentation.ps1      → exit 0
```
Also run the Go unit tests for the new windowing logic (full-page split, minimum-window
fallback) and the NULL/0/N mapping.

Enrichment row count before/after the republish (ReplacingMergeTree should converge).

## 🚦 STOP

Write `docs/writeback-phase2a-fix-report.md` and stop. Do not start Phase 2B.

## Out of scope

- Phase 2B (status derivation, filter, list UI)
- Choosing any threshold; using `required_duration_seconds` in any product logic yet
- Changing dedup identity or fixing a hash loop if one is found (report only)
- Retreaver changes or activation
- Deleting rows, including the 1,047 older rows outside the backfill window
- Any write to Ringba; authenticating `/sync` and `/test-auth`
