# Calyx — Stage 4: Retreaver (verify → paging fix → re-sync & re-cert → write-back research)

**Deliverable per phase:** `docs/retreaver-stage4-report.md` (one file, one section per phase,
appended as each phase finishes).

**Goal:** Retreaver reaches the same standard Ringba reached: no silently missed or stale
calls, full history reconciled against the Retreaver API with 0 differences, and a proven
path to convert / void from Calyx.

---

## Who is who

- **Operator = Aryan.** Runs this spec with Claude Code, runs commands, forwards each report.
- **Owner = Syed.** Reviews every report and gives each `GO`. He also holds the Retreaver
  account: credentials, the account confirmation, test calls in Retreaver.
- 🙋 = stop and ask the operator. When the question is for the owner (credentials, account
  confirmation, test calls, approvals), say so plainly so the operator can forward it.
  Do not guess and do not continue past a 🙋.

## Branch and review flow

- Before starting: `git pull` on `main`, then create and work on branch
  **`stage4/retreaver`**. Never commit Stage 4 work to `main` — the server pulls `main`,
  and only reviewed code may reach it.
- At the end of each phase: commit on the branch, push **the branch only**
  (`git push -u origin stage4/retreaver`), and give the operator the commit hash. The
  owner reviews the report and the diff on GitHub.
- Merging to `main` happens only when the owner says so, after the whole stage is approved.
- No commit whose only purpose is recording a commit hash.

---

## ⛔ Rules (apply to every phase)

1. **Phase by phase with a hard stop.** Finish one phase, write its report section, **STOP**.
   Do not start the next phase until the operator passes on the owner's `GO <phase>`
   (for example `GO R1`).
2. **Verify first, never assume.** Before changing anything, read the current code, migrations,
   docs and data. Do not trust earlier reports, this spec, or code comments. State where each
   fact came from (file:line, command output, API response).
3. Label every claim `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. An HTTP 200 is not proof; row
   counts and field-by-field comparisons are.
4. **Reuse, don't fork.** Ringba already solved windowed pagination, sweeps, hash-after-publish,
   reconciliation, and write-back safety (per-tenant flag, kill switch, idempotency, audit,
   read-back). Where that code is platform-generic, use it for Retreaver. Where it is
   Ringba-only, say so and propose the smallest change that makes it shared.
5. **Secrets:** never print, log or commit API keys, company IDs or caller numbers. Refer to
   the integration by its Calyx integration ID and name only.
   - **Credentials never go into the Claude Code chat.** The owner enters the Retreaver
     API key and company ID himself in Calyx **Settings**.
   - Live API probes read the credentials from that stored integration, decrypted inside
     the script's own process. They are never echoed and never written to a file.
   - If a probe cannot work this way, 🙋 stop and ask.
6. **Retreaver writes are forbidden** except in Phase R3, and there only on the call the
   owner names, one call at a time, after the owner approves (🙋). GET requests to the
   Retreaver API are allowed in every phase.
7. **Account check (known lesson):** Retreaver was once certified against the wrong account.
   Before any live test, confirm and report which Retreaver account the integration points
   to: the account name, and only the last 4 characters of the company ID. 🙋 The owner
   confirms it is the right one.
8. Ringba sync stays as it is. Don't touch Ringba code paths except to make shared code
   shared, and prove Ringba still passes after any shared change: gates plus one Ringba sync
   tick. **Do not** turn a Ringba integration Live just for this test. If none is live in
   this environment, prove it with Ringba unit tests and say so.
9. Secret-scan every commit before pushing the branch. Never push to `main`.
10. **One live sync per platform account.** Retreaver and Ringba enforce API limits per
    account. Before making a Retreaver integration Live here, 🙋 confirm with the owner that
    no other environment (the owner's laptop, the server, anything else) is live-syncing the
    same Retreaver account.
11. **Don't disturb a running stack.** If this environment is the staging server:
    - Do not stop, rebuild or restart services other than the ones a phase needs.
    - Never run `docker compose down -v`.
    - Never delete volumes.
    - Report every restart.

---

## Known context (verify, do not trust)

- Adapter: `services/platform-sync/internal/adapter/retreaver_adapter.go`. Pulls
  `GET https://api.retreaver.com/api/v2/calls.json` with `updated_at_start`,
  `sort_by=updated_at` (the adapter comment says Retreaver forces **desc**), `page` and
  `per_page=100`. The cursor stores `{updated_at_start, page}`. It has a 15-minute
  safety rewind and a 30-day default lookback.
- **K1 / audit F012 (P1, [PROVEN] in `docs/production-readiness-audit.md`):** paging with
  `page` / `per_page` over a window sorted by `updated_at` desc, while calls keep updating.
  When a call's `updated_at` changes mid-paging, it jumps to page 1 and every row after it
  shifts. Rows get **skipped** or duplicated. The same pattern silently skipped **431 Ringba
  calls** before S1A.
- Ringba fix (S1A, commits `3e22959`, `ca048d9`): 6-hour incremental window, day-windowed
  full-history backfill, hourly 7-day and daily 30-day sweeps, content hash stored only
  after a successful publish, and windowed reconciliation proof. See
  `docs/s1a-sync-window-report.md`, `docs/s1a-followup-report.md` and
  `services/platform-sync/internal/adapter/ringba_windowing.go`.
- Data: after the machine reset, local ClickHouse has **0 real Retreaver calls**
  (`docs/deploy-checklist.md`). Earlier certification reports:
  `docs/retreaver-final-certification-report.md`, `docs/phase2-r2-retreaver-reverify.md`,
  `docs/retreaver-api-d9-proof-report.md`.
- The docs mention `updated_at_end`, `created_at_start` and `created_at_end` as query
  parameters. **Whether the live API honours `updated_at_end` is `[UNKNOWN]`.** The window
  design depends on it.
- Write-back: Ringba convert/void is built (Phase 1/1b, Phase 2 UI). Retreaver convert/void
  endpoints are **completely `[UNKNOWN]`**.

---

# PHASE R0 — Verify what exists (read-only + live GETs)

No code changes. Report and STOP.

**R0.0 Environment.** Report:
- Where this runs: staging server `calyx.ridgerisemedia.com` or a local dev machine. OS.
- Repo path, branch, HEAD commit, and `git status --short` line count. If files show as
  modified only because of line endings, prove it with `git diff --ignore-cr-at-eol --stat`.
- `docker compose ps`: which services run and on which images.
- Whether the verify gates can run here. `scripts/verify-*.ps1` are PowerShell scripts;
  on Linux they need `pwsh`. If they cannot run, list the equivalent checks you will use
  instead.
- Go and Node versions.

**R0.1 Code inventory.** For Retreaver, list with file:line:
- the adapter: cursor, window, paging, lookback, the fields mapped by `mapRetreaverCall`, and
  anything not mapped that the API returns (e.g. converted flag, duration, buyer/target names)
- the syncer: does Retreaver get the incremental window, hourly and daily sweeps, backfill,
  and hash-after-publish that Ringba got in S1A, or are those Ringba-only? Answer per
  feature.
- dedup / content hash: which fields go into the Retreaver hash
- throttling / rate-limit handling for Retreaver (limits used, where they come from)
- write-back: the `writeback` package, migrations `000024`/`000025`, and the API routes.
  Which parts are platform-generic and which are Ringba-specific? Does anything stop a
  Retreaver integration from reaching the convert/void path today?
- UI: what the call list, call detail and Settings show for Retreaver today, and whether the
  Stage 3 convert/void screens appear for a Retreaver call. They must not show a working
  button while the backend can't convert Retreaver.

**R0.2 Data state.** In this environment's Postgres: Retreaver integrations (ID, name,
mode, data source, active, last sync, cursor). In this environment's ClickHouse: Retreaver
row count, by prefix (real / `apisync-` / `demo-` / `smoke-`).

**R0.3 Live API facts (GET only).** 🙋 Ask for the owner to save the real Retreaver
integration in Settings with **Data source = Test**, so the credentials are stored but no
live sync runs. Confirm the account (Rule 7). Then prove the following with real responses,
using credentials only as Rule 5 describes (keys redacted):
- Is `updated_at_end` honoured? Request a narrow window and show that every returned row is
  inside it.
- Is `created_at_start` / `created_at_end` honoured?
- Which sort orders actually work (`sort_by=created_at`, `order=asc`)?
- The maximum `per_page` the API accepts.
- The time zone of the `*_start` / `*_end` parameters and of the timestamps in responses
  (UTC or account time zone). Day-by-day reconciliation in R2 depends on this.
- The response shape for paging: total count, `next` link, headers?
- Rate limits: response headers, a documented limit, or the response when throttled. Cite
  the source.
- Is there a single-call endpoint (e.g. `/api/v2/calls/<uuid>.json`)? Show one real
  response.
- Which fields exist for conversion state and duration: revenue, payout, converted flag,
  `connected`, duration?

**R0.4 K1 reproduction.** Show K1 is real with the current code. A deterministic unit test
with a fake API is enough: rows whose `updated_at` changes between page fetches, and the
current adapter skipping at least one of them. Do not fix it yet.

**R0.5 Proposal.** Using the R0.3 facts, propose the R1 design in half a page. Which window
field and size, how paging works inside a frozen window, sweeps, and backfill. What is
shared with Ringba and what is new.

**Gate R0 — STOP.** Report sections R0.0–R0.5. Commit the report on the branch and push the
branch. Wait for `GO R1`.

---

# PHASE R1 — Paging fix (K1) + sync parity with Ringba

Build what the owner approved in R0.5. Minimum requirements:

- **Frozen windows.** Each sync pages inside a window whose end is fixed when the window
  starts (`[start, end)`, end ≤ now). A row that updates during paging moves out of the
  window; the next window catches it, so it is not lost. If `updated_at_end` is not honoured,
  use the approach approved in R0.5 and prove it has no gaps.
- **Cursor** stores the window, not only `updated_at_start` + page. Resume after a crash
  continues the same window. Old cursors migrate cleanly.
- **Overlap** between consecutive windows. Duplicates are fine because the content hash
  suppresses them. Gaps are not.
- **Sweeps and backfill** like Ringba: an hourly re-check of the last 7 days, a daily re-check
  of the last 30 days (configurable), and a day-windowed full-history backfill. Share Ringba's
  code where possible.
- **Hash after publish:** Retreaver's content hash is stored only after a successful Kafka
  publish.
- **Throttle** at the limits proven in R0.3, per Retreaver account.
- Every new or changed setting is documented with its default.
- If R0.1 found a convert/void button that shows for Retreaver calls but cannot work,
  disable or hide it for Retreaver in this phase. Nothing else in the UI changes.

**Proof:**
- The R0.4 K1 test now passes. Add tests for: a row updated mid-window, a crash
  mid-window and resume, overlap without duplicates in ClickHouse, and an empty window.
- Unit tests for the shared code still pass. Ringba: no regression, proven as Rule 8
  describes.
- All gates exit 0, or the R0.0 equivalent checks pass.
- Content hash vs. sweeps: the dedup TTL (`DEDUP_TTL_HOURS`, currently 240h ≈ 10 days) is
  shorter than the 30-day sweep. Show what happens when the sweep re-reads an unchanged
  call older than the TTL. Is it republished? Does ClickHouse keep exactly one row? Prove
  that the dashboard counts it once.

**Gate R1 — STOP.** Commit on the branch, push the branch. Report and wait for `GO R2`.

---

# PHASE R2 — Re-sync and re-certify (live)

🙋 **Owner + operator step:**
1. Confirm Rule 10: no other environment is live-syncing this Retreaver account.
2. The owner switches the integration saved in R0.3 from Test to **Live** (API key) in
   Settings of this environment himself.
3. Confirm the account (Rule 7).

Ringba stays as it is. The R1 code must be running here first: build and restart only
`platform-sync` (plus anything else R1 changed) and report it.

- **Backfill** full history (or the range the owner approves 🙋). Report duration, API
  calls used, and any throttling.
- **Reconciliation, day by day.** For every day in the range, compare the Retreaver API with
  ClickHouse: count of unique calls, and per call revenue, payout, status, campaign,
  target / buyer, caller state, `created_at` and `updated_at`. Target: **0 missing, 0
  extra, 0 field differences.** List any difference by call UUID prefix and explain it.
- **Live change test (D9 style).** One real call changes after the first sync (revenue 0 →
  non-zero, or a status change). Show BEFORE in ClickHouse, AFTER in the API, and the
  updated row in ClickHouse after the next window or sweep. Use a call the owner points
  to 🙋. Do not make one up.
- **Sweep proof:** a row whose `updated_at` falls inside the last 7 days but outside the
  incremental window is picked up by the hourly sweep. Show the tick log. If no such row
  occurs naturally during the test, say so (`[UNKNOWN]` live) and point to the R1 test that
  covers it.
- **UI check:** Dashboard, Pings (Retreaver filter) and call detail show the Retreaver calls
  with correct numbers. Screenshots. Dashboard totals match the ClickHouse totals for the
  same range.
- Certification summary for `docs/platform-certification.md` in the same style as Ringba's.
  Write it into the report. Update the doc only after the owner approves 🙋.

**Gate R2 — STOP.** Commit the report on the branch, push the branch. Wait for `GO R3`.

---

# PHASE R3 — Convert / void write-back: research and sandbox proof

**Research only. No product code, no UI, no migrations in this phase.**

**R3.1 Documentation research.** From Retreaver's official API docs (cite URLs), find every
endpoint or mechanism that can:
- mark a call converted, set or adjust revenue or payout (convert)
- reverse or cancel a conversion, or set revenue or payout back to 0 (void)
- read back the conversion state of a single call

For each one: method, path, auth, parameters, whether it is absolute or relative, whether it
can be reversed, and which role or permission it needs. Also check whether Retreaver's
conversion model is a postback or pixel fired per call rather than an API call. Label each
finding `[PROVEN-docs]` or `[UNKNOWN]`.

**R3.2 Sandbox plan.** Write a plan in the style of `docs/ringba-writeback-sandbox-report.md`
before touching anything:
- 🙋 The owner creates a disposable test call on an internally owned Retreaver campaign
  (no external buyer or publisher, $0 terms) and gives the call UUID.
- If a step needs a Retreaver role or permission the account doesn't have, stop and report
  it. Do not look for a workaround.
- One step per endpoint: request, expected result, read-back, and how to undo it.
- Amounts no larger than $1.

**STOP after R3.2.** Commit the plan in the report, push the branch. The owner approves
the sandbox plan 🙋 before R3.3.

**R3.3 Sandbox execution** (only after approval, and only on the call the owner named). Execute the plan
step by step, with read-back after every write. Prove:
- convert works
- void works and can be reversed or not
- whether the endpoint is absolute or relative
- which read-back signal is reliable. Remember the Ringba lesson: `hasConverted` stayed true
  after a void, so the state had to be checked together with the amount.

**R3.4 Integration design (proposal only).** How Retreaver plugs into the existing write-back
path:
- per-tenant credentials
- the same flag, kill switch, idempotency and audit
- any differences from Ringba, and what the UI needs to change

This becomes the next spec (Stage 4b implementation).

**Gate R3 — STOP.** Commit the report on the branch, push the branch. Wait for the owner.

---

## Server deploy note (every phase that changes code)

In the report, list exactly what must be done on the server once the owner approves the
merge to `main`:
- which images to rebuild (e.g. `platform-sync`)
- any new `.env` variable, with its default
- any migration
- any one-off command

If nothing, write "nothing". Remember that Ringba (and later Retreaver) live sync must run
from **one place only** — laptop or server, not both. The shared per-account API limit is
why.

## Report format — `docs/retreaver-stage4-report.md`

Each phase section contains:
1. Scoreboard (item → PASS / FAIL / BLOCKED)
2. Findings with sources and labels
3. Changes (file list, commit hash)
4. Proof (tests, commands, reconciliation tables, screenshots)
5. Server deploy note
6. Open questions for the owner
7. Honest limitations

## Out of scope

Ringba behaviour changes · UI redesign (except the R1 item: hiding or disabling a Retreaver
convert button that cannot work) · intelligence services · webhooks ·
TrackDrive / CallGrid · pushing or merging to `main`.
