# Calyx — Pre-Phase-2: Gate Coverage + Test-Data Hygiene

**Scope:** make the verification gate cover the write-back service, and stop test data
from contaminating real numbers. **No Phase 2 work. No UI. No write-back code changes.
No API sync turned live.**

**Deliverable:** `docs/pre-phase2-report.md`

---

## ⛔ RULE

Do exactly the parts below, in order. **Part C ends in a STOP — no rows are deleted in
this file.** If anything is ambiguous or reality differs from this document, STOP and
report; do not infer intent. Do not start Phase 2.

Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt images.

---

## Why this exists

1. **The gate does not check the write-back service.** `verify-system.ps1` lists 10 app
   containers, health ports 8080–8087 + 3000, and Go unit tests for 8 services.
   `platform-sync` (API sync + convert/void) appears in **none** of them.
   `packages/pingdedup` is not tested either. A green gate therefore says nothing about
   the service that moves money. `[PROVEN]` from the script source.

2. **Every gate run writes fake calls that count as real.** `verify-ingestion-slice.ps1`,
   `ws-smoke-test.mjs`, `smoke-test.ps1` and `smoke-test-all-platforms.ps1` send webhook
   pings with random UUIDs. A random UUID is neither `apisync-` nor `demo-`, so the
   exclusion helper does not catch it and it lands in headline KPIs. `[PROVEN]` from the
   script source.

3. **Fixture rows (`demo-%`) are still counted in KPIs.** Decision taken: keep the rows,
   exclude them from counts.

---
---

# PART A — Gate covers platform-sync

In `scripts/verify-system.ps1`:

1. Add `platform-sync` to the Docker app-service list.
2. Add `8088` to the health-endpoint ports (`http://localhost:8088/healthz`).
3. Add `services\platform-sync` to the Go unit-test list.
4. Add `packages\pingdedup` to the Go unit-test list.

**This is coverage expansion only.** Do not remove, relax, or reorder any existing check.

Expected summary after the change:
```
Docker:   15 / 15   (was 14)
Healthz:  11 / 11   (was 10)
```

If the newly included tests **fail**, that is a real finding — report it, do not skip the
package or mark it optional to get green. The write-back tests
(`TestDoubleClickOneFire`, cross-tenant, auth, confirmed/pending/failed) must now run as
part of the gate.

**DoD A:** diff of the script; one full `verify-system.ps1` run exit 0 with the new counts
and platform-sync test output visible in the log.

---
---

# PART B — Stop new contamination

## B1 — One exclusion helper for all test-data families

Current helper (`apiSyncExclusionSql`, `services/api/src/common/demo-rows.ts`) excludes
only `apisync-%`.

Extend it to exclude **three** prefixes:

| Prefix | Source |
|---|---|
| `apisync-` | mock API-sync adapter |
| `demo-` | ClickHouse fixture `000006_demo_fixture` |
| `smoke-` | verify / smoke scripts (new — see B2) |

- Rename the helper to reflect what it now does (e.g. `testDataExclusionSql`). Keep the
  old names as deprecated aliases so nothing breaks.
- The existing `show_demo=true` toggle must include **all three** families when on.
- **Rows stay in the database.** Read-time filtering only.
- Extend the anti-drift test to assert all three prefixes.
- Update the naming-convention note in the helper file: *synthetic or script-generated rows
  must use `smoke-` (scripts) or `apisync-` (mock adapter). Never a bare UUID.*

**Must still work after B1** — these depend on fixture rows by ID, not by aggregate:
- `verify-presentation.ps1` (deep view / score / transcript for `demo-call-1`)
- `verify-system.ps1` web route `/calls/demo-call-1`

Prove both still pass. If excluding `demo-%` breaks either, STOP and report.

Report dashboard KPIs (total pings, revenue, exposure) **before and after** B1 so the drop
from removing fixture rows is visible and explained.

## B2 — Scripts generate prefixed IDs and assert against the inclusive view

For each script that sends webhook pings:
- `scripts/verify-ingestion-slice.ps1`
- `scripts/ws-smoke-test.mjs`
- `scripts/smoke-test.ps1`
- `scripts/smoke-test-all-platforms.ps1`

Change generated call IDs from a bare UUID to `smoke-<uuid>`.

Then every read the script uses to **find or count** those pings must pass
`show_demo=true` (`/pings`, `/analytics/metrics`, `/coverage/heatmap`) — otherwise the
script's own rows are hidden and its assertions fail.

**This is not weakening a check.** The assertion is unchanged ("a new ping increments the
total by at least 1"); it is simply evaluated against the view that includes test data.
To prove the check still has teeth, report for `verify-ingestion-slice.ps1`:
```
show_demo=true   total before → after   (must increase by ≥ 1)
default view     total before → after   (must NOT change — smoke row excluded)
```
Both lines are required. The second is the proof that smoke rows no longer reach KPIs.

**Check first, don't assume:** confirm the ingestion connectors accept a non-UUID call ID
for every platform the scripts use (Ringba `CallID`, Retreaver `call_uuid`, TrackDrive,
CallGrid). If any connector rejects or mangles `smoke-…`, STOP and report which one.

## B3 — `verify-api-sync.ps1` (roadmap H)

Same mechanism, same file class:
- Assert growth using `show_demo=true` (mock adapter rows are `apisync-`).
- At the end, **deactivate** (not delete) the integration the script creates, so runs stop
  leaving active mock integrations behind.

Run it and report exit code. It is still **not** added to the gate in this file.

## B4 — Stop the running mock generators

Report every `integrations` row with `connection_mode='api_sync'` AND `is_active=true` AND
`sync_mode='mock'` — these are producing `apisync-` rows on a schedule.

Deactivate them (`is_active=false`, not delete). **Do not touch:**
- `ce6fd338-…` Ringba Live (`is_active=false`, `sync_mode=real`, `writeback_enabled=true`)
- webhook dev rows (`Ringba Dev Pixel`, `* Dev Postback`) — the smoke scripts need them

Report the table before/after, and the `apisync-%` row count sampled twice a few minutes
apart to show growth stopped.

**DoD B:** helper diff + anti-drift covering 3 prefixes; KPI before/after; four scripts
using `smoke-` with the two-line proof; connector acceptance confirmed; `verify-api-sync`
exit code + teardown; mock generators stopped with counts.

---
---

# PART C — Inventory of existing contamination (NO DELETION)

Real Retreaver calls pulled by API sync **also have UUID call IDs**. A deletion keyed on
"looks like a UUID" would destroy real data. So this part only counts and classifies.

Report, from ClickHouse `pings`, grouped by family:

| Family | How identified | Distinct calls | Rows | Revenue counted in KPIs |
|---|---|---|---|---|
| `apisync-%` | prefix | | | (already excluded) |
| `demo-%` | prefix | | | (excluded after B1) |
| Old smoke rows | **see below** | | | |
| Real Ringba | `RGB%` via api_sync | | | |
| Real Retreaver | api_sync | | | |
| Unclassified | anything left | | | |

**Identifying old smoke rows — provenance, not shape.** Determine what distinguishes rows
that came from the smoke scripts from real calls. Candidates to test, not assume:
- `sync_path = 'webhook'` (column exists from migration `000011`; rows older than that
  may have `''`)
- ingestion via the dev webhook integrations / dev tokens
- anything else stored that records origin

For each candidate, state whether it can produce a **false positive on a real call**. If no
field cleanly separates them, say so — that is an acceptable answer.

Then write, **but do not run**, the proposed deletion statements for each family you would
remove, with the exact row count each would affect.

## 🚦 STOP

Write `docs/pre-phase2-report.md` and stop. **No `DELETE` / `ALTER TABLE … DELETE` is
executed in this file.** The operator reviews Part C and approves deletions separately.

**DoD C:** family table with counts; provenance analysis with false-positive risk per
candidate; proposed statements with affected counts; explicit confirmation that nothing
was deleted.

---

## Final verification (before the STOP)

Run and report raw output + exit codes:
```
scripts/verify-system.ps1            → exit 0, Docker 15/15, Healthz 11/11
scripts/verify-ingestion-slice.ps1   → exit 0, with the two-line proof
scripts/verify-presentation.ps1      → exit 0 (demo-call-1 still works)
```

## Out of scope

- Phase 2 (list / status / filter) — do not start
- Turning API sync live; changing `ce6fd338` in any way
- Any write-back code, settle window, lock, or audit change
- Executing any deletion
- Adding `verify-api-sync.ps1` to the gate
- Authenticating `/sync` and `/test-auth` (reported earlier, still out of scope)
