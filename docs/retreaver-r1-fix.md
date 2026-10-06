# Calyx — Stage 4 R1-fix: owner's review of R1

**Applies to:** `stage4/retreaver`, PHASE R1 as reported in `docs/retreaver-stage4-report.md`
**Status:** R1 is **not yet accepted**. The code work is strong. A few items must close before
the integration goes Live in R2. **No `GO R2` yet.**
Where this file and the report or amendments disagree, this file wins.

---

## What is accepted as-is ✅

- B2 is complete. The real leak in `formatHTTPStatusError` was found and fixed in shared
  code, with 4 tests. B2(1) is proven: header auth is not possible, so redaction is the only
  defence.
- B1: the key is in 1 transcript file. The owner accepts keeping that file private to the
  operator's machine (no rotation, A1).
- C1 frozen window with window-end splitting. C4 table. C5 hash test in both directions. C7
  downtime, long-call and crash-resume tests. C9 throttle. C10 Go parity.
- D: `RangeFetcher` capability replaces both `"ringba"` guards.
- Gate E: all three PowerShell gates pass. The six portability fixes and `npm ci` are
  accepted, except the `verify-presentation` change, which F5 below covers.
- The `verify-system` port fix 3000 → 3100 is accepted. It exposed a real gate blind spot
  that existed since the port remap.

---

## Owner decisions

**D-C3 — `null` revenue/payout: no migration, with monitoring.** The 100-call evidence
(`revenue = null` ⟺ `converted = false`, zero cases of `0`) is accepted, so `null → 0.00`
beside `converted` is fine. The assumption must be **watched, not trusted**. See F3.

**D-C8 — account check.** The owner will send one call UUID from the Retreaver web UI
through the operator. Run C8 as part of this fix and report `status`, `created_at` and
`revenue` for him to compare.

---

## Fixes required before R2

### F1 — Finish C2. A short window must never stall silently. 🔴
Today `FetchRange` "refuses to advance" when rows < `Total`, but nothing retries and nothing
tells anyone. On a live 96k-call backfill, that becomes a sync that is stuck with no visible
error: the exact silent gap this stage exists to prevent. Build:
- a bounded retry of the same window: N attempts with backoff, both configurable, defaults
  documented
- after N failures: keep the cursor, set the integration's sync status to **degraded** with
  a clear `last_sync_error` (no credentials, redacted), and emit one log line with window
  start/end, `Total` and the number of rows collected
- recovery: the next successful walk clears degraded automatically
- the same rule on the incremental path and the `RangeFetcher` path. State both.
- **Tests:** mismatch then success on retry, so the cursor advances; persistent mismatch,
  so status goes to degraded and the cursor is held; recovery, so degraded clears.
- Show where "degraded" is visible to the owner in the UI (Settings → Connected
  Platforms). If it is not visible, make it visible there. This is the only UI change allowed.

### F2 — Cover the "update just before `window_end`" case. 🟡
The corrected K1 test only models an update that stamps `updated_at ≥ window_end`. Clock skew
between Calyx and Retreaver, or a slow walk, can stamp an update just **before** `window_end`.
In that case the row reorders to the top **inside** the frozen window and is skipped in this
walk. It should then be collected by the next window's 15-minute overlap.
- Add a test: a row updated mid-walk with a new `updated_at` = `window_end − 30s`. Assert it
  is delivered across consecutive windows.
- State the guarantee in the report: if the walk takes less than the overlap and clock skew
  is below it, no row is lost. Also state the longest possible walk: `MAX_PAGES × 1/rate`
  (200 pages at 2/sec ≈ 100 s). If that could ever exceed the overlap, say what protects it.

### F3 — Monitor the C3 assumption. 🟡
Count every mapped Retreaver call that breaks the rule: `converted=true` with `revenue` null,
or `converted=false` with non-null `revenue`. Also count `converted=true` with revenue
exactly 0. For each violation, write one log line (call UUID prefix only) and keep a
counter. Add a unit test for each case. If R2 sees any violation, the owner revisits the
migration.

### F4 — Explain three behaviour changes nobody asked for. 🔴
The report mentions them but does not trace them. For each one, give the commit, the reason,
and every consumer affected:
1. **`GET /pings` now returns one row per call by default (`?raw=true` for the event
   stream).** Was this introduced in R1 or already in `convert-cancel-screen` (`c4e3c5d`)?
   If R1: which spec item authorised it? List every consumer: dashboard KPIs, Coverage,
   CSV export, the Stage 3 list and filter, the live feed, the scripts. Show the dashboard
   totals before and after for the same range.
2. **`EventType ← Retreaver status`** (`finished`, `forwarded`, …). List every place that
   reads `EventType` / `event_type` (filters, badges, the ingestion path, analytics) and
   prove that the new values break none of them. If any consumer expects a fixed set of
   event types, say so.
3. **`HasConvertedPresent`.** Where is it stored? The report says "Migrations: none". If
   it is only in the Kafka message or only in memory, say how the API knows
   `conversion_reported` (the report says it comes from `platform-capabilities.ts`). Then
   say whether `HasConvertedPresent` is used anywhere at all.

If any of the three is scope creep with no owner decision behind it, **say so plainly** and
propose: keep, or revert.

### F5 — Gates must not get weaker. 🟡
`verify-presentation.ps1` now requests `show_demo=true` so it passes on a fixture-only
database. That makes the gate pass on demo data alone. Change it: when the database holds real
(non-demo) rows, assert on real rows **without** `show_demo`, as before. Only when there are
none, fall back to `show_demo=true` and print a clear `DEMO-ONLY` note in the gate output.
Show both paths.
Also add a small `README` note on the macOS `pwsh-preview` binary name and the Makefile
alias.

### F6 — Test-mode mock data should be realistic. 🟢
The Test-mode Retreaver generator creates rows with revenue ($11.25) but `converted=false`,
so 57 mock calls show "Review". Make the mock follow the real rule: revenue present only when
`converted=true`. Otherwise every demo shows false alarms.

### F7 — The report must be one truth. 🟡
The R1 section was appended over time and now contradicts itself:
- §4 says C3 is BLOCKED; §8 says RESOLVED
- §5 and §12 say `pwsh` is not installed; §13 says it passes
- §6 says "Gate R1 is not met; C4–D not started"; the scoreboard says done

Rewrite the R1 section so every item has one current state. Keep the history only as short
"superseded" notes. This report feeds the final dossier.

### F8 — Commit and push. 🟡
The report gives **no commit hash**, and R0 was never committed, which the spec requires.
Commit R0 + R1 + this fix on `stage4/retreaver`, push the branch, and give the hash. The
owner reviews the diff on GitHub.

---

## Gate R1 (re-check)

- F1–F8 done, with the tests listed passing.
- C8 run and reported, if the owner sent a UUID.
- All three PowerShell gates pass, including the F5 real-data path, if real rows exist.
- Ringba: no regression (Rule 8).
- Update the report's R1 section (F7), commit, push the branch only, give the hash, and
  **STOP**. Wait for `GO R2`.
