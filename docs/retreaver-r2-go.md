# Calyx — Stage 4: owner's review of R1 re-check + GO R2 (with pre-flight)

**Applies to:** `stage4/retreaver`, `docs/retreaver-stage4-spec.md` PHASE R2
**Status:** R1 re-check **accepted**. **`GO R2`** — but only after the pre-flight below is
complete and reported. Where this file and the spec disagree, this file wins.

---

## Accepted from R1 ✅

- F1 retry + degraded + migration `000028` + the Settings pill. F2 clock-skew test and the
  stated guarantee (walk + skew < overlap; 100 s worst walk vs a 900 s overlap). F3 C3
  monitor. F5 gate with a real-data path and a loud `DEMO-ONLY` note. F6 realistic mock.
  F7 one-truth report.
- F4 tracing is honest and complete:
  - The `/pings` collapse came from `c4e3c5d`. It is handed to the Stage 1–3 audit.
  - `EventType ← status` was pre-existing, and nothing constrains its values.
  - `HasConvertedPresent` is inert and documented as such. Keep it.

---

## Owner decisions (confirmed)

The report attributes two decisions to the owner. They are confirmed here so they rest on
this file, not on memory:
1. **Keep** the `reportsConversionStateSql()` guard on the `converted` / `unconverted`
   filters, not only on `should_convert` (§6.4).
2. **Keep** the new `degraded` status and migration `000028`, instead of reusing `error`.

---

## Pre-flight — complete before any Live switch

**P1 — Commit hash + push.** §9 says "Commit hash in §12", but §12 has no hash. F8 is not
done until the hash is given and `stage4/retreaver` is visible on GitHub. Give the hash, and
the output of `git log origin/stage4/retreaver -1 --oneline`.

**P2 — `degraded` follows the Ringba rule for `error`. 🔴**
Ringba has no `degraded` status. When a Ringba sync fails:
- `sync_status` becomes `error` and the cursor is held (`syncer.go:434` on `main`).
- The next tick picks the integration up again, because `list_api_sync_integrations()`
  (migration `000021`) filters only on `connection_mode = 'api_sync'`,
  `is_active = TRUE` and credentials present. There is **no `sync_status` filter**.
- The next successful sync sets it back to `ok` with no operator action.

`degraded` must behave **exactly the same as `error` for scheduling**:
- It is retried on every tick, sweep and deep sweep.
- It is cleared to `ok` by the next successful walk.
- The only difference is the meaning: `degraded` = the data came back incomplete (short
  window); `error` = the API or network failed.

On `stage4/retreaver`, confirm with file:line:
1. `list_api_sync_integrations()` and any other function or query that selects integrations
   for the tick, the sweeps or the deep sweep still have **no `sync_status` filter**.
   Include migration `000028`, if it touches any of them.
2. The syncer (Go) never skips an integration because of its `sync_status`. Search every
   read of the status.
3. A test: an integration with `sync_status = 'degraded'` is selected on the next tick, and
   a successful walk sets it to `ok`.

If anything excludes `degraded` (or `error`), fix it to match the Ringba rule and add the
test. Otherwise the first shortfall would stop the sync forever.

**P3 — The API and UI accept `degraded` at runtime.** `tsc` passing is not enough. Check
every runtime validator on `sync_status` in the API (Zod schemas, DTOs, enums) and in the web
(type guards, `switch` defaults). Show that a row with `degraded` loads in Settings without
an error. You may set one **Test** integration to `degraded` by SQL for this check, then set
it back.

**P4 — C8, finished properly.** The owner compares the three calls from §8.1 (`d8858bae…`,
`97818024…`, `3e0d7343…`) in the **Retreaver web UI** itself: `status`, `created_at` and
`revenue`. That is Retreaver's UI, not Calyx's, so it does not need R2. 🙋 Wait for the
owner's "match" or "mismatch". Alternatively the owner sends one **full** call UUID.
- Note: §8.1 says Retreaver "caps `per_page` at 25", while R0.3 Q4 proved the max is 100
  (anything above falls back to 25). Correct whichever statement is wrong.

**P5 — Rule 10.** 🙋 The owner confirms that no other environment (his laptop, the staging
server) has a Live Retreaver integration for company `…5270`.

**P6 — Backfill range.** 🙋 The owner approves the range. The recommendation is the last
**90 days** first, then the full year (`Total` ≈ 96,747) as a second pass. That way the
reconciliation, sweeps and degraded behaviour are proven on a smaller window before the full
load.

---

## R2 additions (on top of the spec's PHASE R2)

- **Reconciliation per UTC day** for the approved range: API count vs ClickHouse distinct
  calls, plus field differences. Use the `Total` header per day as the API denominator.
  Target: 0 missing, 0 extra, 0 field differences.
- **Duration check:** for 10 real calls, compare `DurationSeconds` and `CallLengthSeconds` in
  Calyx with the durations the Retreaver UI shows. 🙋 The owner reads the UI values. This
  settles whether the `dialed_call_duration` mapping is right (C4, `[INFERRED]`).
- **C3 monitor counters** after the backfill: all three violation counts. Any non-zero count
  goes to the owner.
- **`uniqExact(event_type)`** per platform after the backfill, with the list of Retreaver
  `status` values seen.
- **Degraded, live:** report whether any window went degraded during the backfill. If none
  did, say so. Do not fake one.
- **Dashboard check:** Dashboard, Pings (Retreaver filter) and one call detail, with
  screenshots. The Retreaver totals for the range must match ClickHouse. Check "should have
  converted" for Retreaver: every flagged row must really be `revenue > 0 AND !converted`.
- **Sweep proof** as the spec describes, or `[UNKNOWN]` live with a pointer to the unit test.
- **Load:** API requests used, any 429s, backfill duration, and CPU/memory on this machine
  during the run.
- **Server deploy note:** keep it current. `000028` runs before the new `platform-sync`.

## Gate R2
Report in the PHASE R2 section, commit on `stage4/retreaver`, push the branch only, give the
hash, and **STOP**. Wait for the owner before R3.
