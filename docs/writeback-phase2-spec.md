# Calyx — Write-Back Phase 2: Call List (Status + Filter)

**Goal:** the admin can find convert candidates among hundreds of calls without opening
each one. A Status column and a server-side filter on the calls list.

**Not in this phase:** call-detail changes (Phase 3), convert/void UI (Phase 4), any
write to Ringba, Retreaver sync, bulk actions.

Phase 2 is split: **2A** (data) → STOP → operator approves the duration threshold → **2B**
(status, filter, UI).

---

## ⛔ RULE

Do exactly the part you are told to do. Do not start the next part. Do not refactor beyond
scope. If a step is ambiguous or reality differs from this document, STOP and report — do
not infer intent.

Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt Docker images.
A fix not in the deployed image is not a fix.

---

## Current state (start of Phase 2)

| Item | State |
|---|---|
| Ringba integration `ce6fd338-af48-47d8-850a-bd10b855e2f4` | `sync_mode=real`, `is_active=false`, `writeback_enabled=true`, `api_sync` |
| ClickHouse real data | 4100 Ringba calls (`RGB%`, `sync_path='api_sync'`); 0 Retreaver |
| Default KPIs | exclude `apisync-`, `demo-`, `smoke-` (shared helper) |
| `pings.has_converted` | `UInt8`, exists (migration `000012`) — **no down migration** |
| Existing rows' `has_converted` | all `0` — ingested before the mapping existed. `[PROVEN]` earlier slice |
| Gate | `verify-system` 15/15, 11/11; ingestion slice; presentation — all exit 0 |
| Repo | clean, pushed (`8f4c8f7`) |

## Proven Ringba facts this phase relies on

| Fact | Label |
|---|---|
| API ids: `hasConnected`, `wasBlocked`, `blockReason`, `wasConversionAdjusted`, `hasConverted`, `conversionAmount`, `connectedCallLengthInSeconds` | `[PROVEN]` columns dump |
| Converted = `hasConverted=true` **AND** `conversionAmount>0` | `[PROVEN]` |
| After void: `hasConverted` present **false**, `wasConversionAdjusted=true` | `[PROVEN]` Phase 1b |
| Blocked calls: `wasBlocked=true`, `blockReason` e.g. "The bid has expired", `hasConnected` absent | `[PROVEN]` Phase 0 (239/1716) |
| Connected-not-converted calls: `hasConnected=true`, no reason field of any kind | `[PROVEN]` Phase 0.5 (939/1716) |
| Ringba omits many false/empty values from records — but **not all** (`hasPreviouslyConnected=false` was present) | `[PROVEN]` — omission is per-field, not universal |

---
---

# PHASE 2A — Data layer

## 2A.1 — New ClickHouse columns

Next free migration number (`000013`). **Must include a `down`.**

| Column | Source | Notes |
|---|---|---|
| `has_connected` | `hasConnected` | see 2A.2 on null vs false |
| `was_blocked` | `wasBlocked` | see 2A.2 |
| `block_reason` | `blockReason` | string; empty when none |
| `was_conversion_adjusted` | `wasConversionAdjusted` | identifies voided/adjusted calls |

Why `was_conversion_adjusted` is included: a **voided** call is connected and not converted
— exactly the ⚠ Miss shape. Without this flag the list would present calls an admin
deliberately cancelled as fresh convert candidates. `[PROVEN]` void sets it `true`.

**Also in this step:** add the missing `down` for `000012_pings_has_converted`. It is on the
deploy checklist; tick it there once added.

## 2A.2 — 🔴 "False" must be distinguishable from "not provided"

This is the most important correctness point in Phase 2.

Two different situations produce an empty value:

| Situation | Meaning | Must store as |
|---|---|---|
| API-sync record, key absent | Ringba omitted a false value | **false** |
| Row from webhook path, or synced before this phase | Field was never requested | **not provided** |

If both become `0`, every old row and every webhook row looks like "not connected, not
blocked" — and status derivation will silently mislabel them.

**Requirements:**
- Rows written by the Phase 2 API-sync mapper: requested-but-absent boolean → `false`.
- Rows from any other path, and all pre-existing rows: **not provided** (e.g. `Nullable`
  columns where NULL = not provided, or an explicit schema-version marker — choose one and
  justify it).
- **Verify the absent→false rule, don't assume it.** Ringba's omission is per-field. On
  real records, show that when `hasConnected` is absent the call genuinely did not connect
  (e.g. blocked call, zero connected duration). Report the evidence. Same for `wasBlocked`.

**Do not change the type of `has_converted`** in this phase — it is already populated and
read by the deep view. Voided state is identified via `was_conversion_adjusted` + amount.

## 2A.3 — Mapper and content hash

- Add the four fields to the Ringba adapter's `valueColumns`, `RemoteCall`, the Kafka ping
  message, the consumer insert.
- Add them to the `pingdedup` canonical content. **This changes every call's hash** and
  triggers a one-time republish of the lookback window — same as B1.

## 2A.4 — Before enabling sync: volume

Report:
- Ringba calls in the lookback window (query the API count, not a guess)
- Expected republish volume
- Pages required at `size=1000`, and time at the certified rate (5/sec, 80/min per account)

## 2A.5 — Enable sync

- Set `ce6fd338` `is_active=true`. Leave `sync_mode=real`, `writeback_enabled=true`
  unchanged.
- **Retreaver stays off.** Do not activate any Retreaver integration.
- Do not touch any other integration row.

## 2A.6 — Reconciliation (the pagination-skip lesson)

Earlier work proved offset pagination can **silently omit calls** — 45 real calls were
missed by a lookback and only caught because a field was blank. Normal operation has no
such marker.

After the first full sync completes, for the lookback window report:

| | Count |
|---|---|
| Ringba API distinct calls | |
| ClickHouse distinct Ringba calls | |
| Difference | |

If the difference is non-zero, identify which calls are missing and why. **Do not
hand-wave a gap** — explain it or report it as unexplained.

## 2A.7 — Field population

After sync, report for Ringba API-mode rows in the window:

| Field | true | false | not provided |
|---|---|---|---|
| `has_connected` | | | |
| `was_blocked` | | | |
| `was_conversion_adjusted` | | | |
| `has_converted` | | | |

And `block_reason` distinct values with counts.

Report rows **outside** the lookback window (older rows that were not republished): how
many exist, and confirm they show **not provided**, not false.

## 2A.8 — Duration distribution (threshold evidence)

`connectedCallLengthInSeconds` (stored as `duration_seconds`). Report distributions for:

| Group | Why |
|---|---|
| Converted calls (`hasConverted` true AND amount > 0) | what a real sale looks like — the strongest evidence |
| Connected, not converted, not blocked, not adjusted | the candidate pool |

For each: count, min, p10, p25, median, p75, max, and a histogram in buckets
(0–10s, 10–30s, 30–60s, 60–120s, 120–300s, 300s+).

**Propose a threshold with reasoning** — e.g. "the shortest converted call was Ns; p10 of
converted is Ms". Do **not** productize it. The operator approves the number before 2B.

## 2A.9 — Gate check under live sync

With sync running, real calls keep arriving. Check whether any gate script asserts that a
total does **not** change (Pre-Phase-2 added a "default view unchanged" line to
`verify-ingestion-slice`). If it is an assertion, live sync can make it flaky.

Run all three gates with sync live and report. If a script fails because live data moved,
report exactly which line — **do not change the script in 2A**; propose the fix.

## 🚦 GATE 2A — report and STOP

`docs/writeback-phase2a-report.md`:
- migration `000013` up/down applied; `000012` down added
- null-vs-false approach, justified, **with evidence** from real records
- volume before sync; republish count
- reconciliation table (API vs ClickHouse)
- field population table, incl. old rows showing not-provided
- duration distributions + **proposed** threshold
- gate results under live sync
- enrichment row count before/after republish (ReplacingMergeTree should converge)

**STOP. Do not start 2B.**

---
---

# PHASE 2B — Status, filter, list UI

*Only after the operator approves 2A and supplies the threshold.*

## 2B.1 — One status derivation, used everywhere

A single shared function (ClickHouse SQL fragment + typed mapping), used by the list, the
filter, and the counts — and reusable by Phase 3's call detail. **No copy-pasted
conditions.** (The earlier demo-row drift happened exactly because a condition was
duplicated across read paths.)

Precedence, first match wins:

| # | Status | Rule |
|---|---|---|
| 1 | **n/a** | platform is not Ringba (write-back is Ringba-only) |
| 2 | **Needs API mode** | Ringba row where the fields are *not provided* (webhook path, or pre-Phase-2 row) |
| 3 | **⊘ Blocked** | `was_blocked` true — show `block_reason` |
| 4 | **✓ Converted** | `has_converted` true AND revenue > 0 |
| 5 | **↺ Adjusted** | `was_conversion_adjusted` true AND not converted — voided/adjusted; **not** a candidate |
| 6 | **⚠ Miss** | `has_connected` true AND duration ≥ approved threshold |
| 7 | **–** | anything else (not connected, or connected below threshold) |

Rules 3–7 read latest values (`argMax` per `(platform, external_call_id)`), consistent with
existing aggregates.

Anti-drift test: fails if any list/filter/count path does not use the shared function.

## 2B.2 — API

- `GET /pings` accepts a `status` filter; filtering is **server-side** in ClickHouse.
- Pagination totals must reflect the filter.
- An endpoint (or field) returning **counts per status** for the current period/platform
  filters, so the UI can show them on the filter chips.
- Test-data exclusion still applies; `show_demo` behaviour unchanged.

## 2B.3 — UI

On the calls list:
- Filter: **All · Unconverted (⚠ Miss) · Converted · Blocked · Adjusted**, each with its
  count.
- A **Status** column on every row.
- "Needs API mode" shows explicit text — *"Requires API-mode integration"* — never a bare
  dash that reads like a bug.
- Blocked rows show the `block_reason` (tooltip or secondary text).
- Live-feed rows arriving over WebSocket must not display a status the server has not
  derived — show a neutral placeholder until the row is fetched, never a guessed ⚠ Miss.
- Styling follows the Teal Glass system (no new indigo).

## 2B.4 — Proof

- Status counts on real data, and they sum to the total.
- One real call of each status visible in the list (screenshot from rebuilt Docker web).
- Filter proven server-side (request/response showing the `status` param and a
  filtered total).
- A voided call (the Phase 1b call `…V3LIN01`, if in the synced window) shows
  **↺ Adjusted**, not ⚠ Miss.
- Anti-drift test passes; shown failing when a path bypasses the shared function.
- All three gates exit 0 (with the 2A.9 fix applied if one was approved).

## 🚦 GATE 2B — report and STOP

`docs/writeback-phase2b-report.md`. Do not start Phase 3.

---

## Out of scope — both parts

- Call-detail layout (Phase 3); convert/void buttons or dialogs (Phase 4)
- Any write to Ringba
- Retreaver sync or any Retreaver integration change
- Changing `has_converted`'s type
- Bulk actions
- Transcript / AI assessment
- Authenticating `/sync` and `/test-auth`
