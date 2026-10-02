# Calyx — Stages 1–3 Verification (read-only audit before merge to `main`)

**Deliverable:** `docs/stage1-3-verify-report.md`
**Under review:** branch `convert-cancel-screen` @ `c4e3c5d`, compared against `main` @
`ba9c825`. Per the Stage 4 R0 report, `c4e3c5d`'s parent is `ba9c825`. Verify that, and
whether Stages 1–3 are one commit or several.

**Why this exists:** Stages 1–3 were built and reported "complete", but nothing has been
reviewed against its requirements yet. `stage4/retreaver` is built on top of this branch, so
nothing merges to `main` (which the server pulls) until this audit is clean.

---

## Who is who

- **Operator** (runs this spec with Claude Code) — Talha, who also runs Stage 4.
- **Owner** — Syed. Reviews the report, decides on every finding, and gives any approval
  marked 🙋.
- 🙋 = stop and ask the operator. If the question is for the owner, say so plainly.

## When and where (same machine as Stage 4)

This audit runs on the **same Mac** as Stage 4 (`stage4/retreaver`). The two must not overlap:

1. Start only **after Gate R2 of Stage 4 is reported** (the Retreaver live backfill and
   reconciliation are finished). Do not start it in the middle of R2.
2. Before switching, set the Retreaver integration's Data source back to **Test**. The
   `c4e3c5d` code still has the old Retreaver adapter with the K1 bug, and it must not
   live-sync. Report this step.
3. Check out `convert-cancel-screen` (`c4e3c5d`), then rebuild and restart the containers
   from it, so the running app matches the code under audit. Record which images were
   rebuilt. No `down -v`, no deleted volumes.
4. The Stage 4 data (real Retreaver rows) stays in the database. Account for it wherever
   counts are compared. Report Retreaver separately, and note that it came from Stage 4.
5. When the audit is committed and pushed, check out `stage4/retreaver` again and rebuild
   its images. Stage 4 continues with R3. Retreaver only goes back to Live if R3 needs it,
   and only with the owner's approval.

## ⛔ Rules

1. **Audit only. No fixes.** Do not change product code, config, migrations or data to make
   something pass. Record findings; fixes come later in a separate spec. The only commit is
   the report itself (plus screenshots / evidence files).
2. **Verify from the code and the running app, never from earlier reports.** Earlier reports
   are context only. Every claim cites file:line, a command and its output, or a screenshot.
3. Label every claim `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. An HTTP 200 is not proof a page
   works. Web items need browser proof.
4. Every requirement ends with exactly one verdict:
   - **PASS**: requirement met, with proof
   - **FAIL**: requirement not met, with proof
   - **PARTIAL**: say exactly what is missing
   - **NOT BUILT**: the requirement was never implemented
   - **CAN'T VERIFY**: say why
5. **Money safety first.** Stage 3 moves real money in Ringba. Every write-back finding gets
   a severity:
   - 🔴 can move money wrongly, or cross tenants
   - 🟡 wrong or missing safety signal
   - 🟢 cosmetic
6. **No live writes to Ringba or Retreaver** unless the owner approves it at the 🙋 in
   Part 3.8, on the sandbox call the owner names.
7. Never print, log or commit a secret: tokens, keys, passwords, caller numbers.
8. Work on branch **`verify/stage1-3`**, created from `convert-cancel-screen`. Push that
   branch only. Never push to or merge into `main` or `convert-cancel-screen`.
9. If something blocks one item, record it and continue with the next.

---

## Part 0 — Environment and scope

- Machine, OS, repo path, branch, HEAD. Run `git status --short` and give the line count. If
  files show as changed only because of line endings, prove it with
  `git diff --ignore-cr-at-eol --stat`.
- `git log --oneline ba9c825..c4e3c5d`, and `git diff --stat ba9c825..c4e3c5d`.
- Group every changed file by stage (1 / 2 / 3 / other). List any file that belongs to no
  stage: that is unreviewed scope, and it is a finding.
- List every spec and report in `docs/` that belongs to Stages 1–3, and which ones are
  missing.
- Confirm the running containers were built from `c4e3c5d` (image build time vs commit
  time, or a rebuild).
- Toolchain: `pwsh`, Go, Node, Docker versions. Say whether the three PowerShell gates can
  run here. They are the acceptance standard. If `pwsh` is missing, 🙋 ask for it to be
  installed.

---

## Part 1 — Stage 1: S2b (S2 regressions + remaining security)

Requirements: `docs/s2b-spec.md` (R1–R4, P1–P4), and `docs/s2b-closeout-spec.md` if it was
used.

| # | Requirement | Proof needed |
|---|---|---|
| 1.1 | Every script that creates call IDs uses a `smoke-` prefix (or `apisync-` / `demo-` where that is its documented purpose). `scripts/check-smoke-call-id-prefix.mjs` runs inside a gate and fails on a bad prefix | Script list with prefixes. Gate file:line. One failing run with a deliberately bad temp line, then reverted |
| 1.2 | No `csp-live-%` rows or other non-real, non-excluded rows in ClickHouse `pings` | Count by prefix |
| 1.3 | Fonts: Geist (headings) + Inter (body) from local files; no serif fallback; the browser gate fails on a serif body | Computed `font-family` of `body` and one `h1` on Dashboard, Pings, Settings and a call detail. The gate's assertion, file:line |
| 1.4 | Top bar shows the signed-in user's email via `/auth/me` | Screenshot + network entry |
| 1.5 | `_next/static` JS/CSS and fonts are cacheable; HTML is `no-store` | Response headers for one of each |
| 1.6 | WebSocket rejects an `Origin` not in `WEB_ORIGINS` | Output of `scripts/check-ws-origin.mjs`. Also state what happens with **no** `Origin` header |
| 1.7 | Production mode (api + web + platform-sync) refuses placeholder secrets and works end-to-end with real ones | Evidence, or NOT BUILT |
| 1.8 | Next.js `npm audit` critical/high: report exists with fixed-in versions and whether Calyx uses the vulnerable feature | Report location, or NOT BUILT |
| 1.9 | Go 1.25 in every `go.mod` and Docker builder; `govulncheck` clean per v1 module | Output |

---

## Part 2 — Stage 2: UI clean-up

There may be no single spec for Stage 2. Check the branch against each requirement below
**and** against any Stage 2 spec or report found in Part 0. Screenshots for every web item.

| # | Requirement | Proof needed |
|---|---|---|
| 2.1 | Dashboard KPI totals match ClickHouse for the same range, and match the Coverage map total | Numbers side by side, with the queries used |
| 2.2 | Ping Feed has no duplicate rows for the same call | Query + screenshot |
| 2.3 | The ping chart follows the selected period (7d / 30d / All). Before the fix it always said "Last 7 days" | Screenshot per period |
| 2.4 | The mock "Publishers" card (MediaCo, LeadStream, DialPro, CallStream) is gone or shows real data | Screenshot + code reference |
| 2.5 | v1 scope in the UI: TrackDrive, CallGrid and Call Loom are hidden or disabled in the platform filter and in Settings → connect. The Webhook connection option is hidden for v1 | Screenshots |
| 2.6 | Mock intelligence (Scoring, Intelligence, Compliance exposure, Ask AI, Ad attribution cards, TCPA) is hidden, or clearly marked as not live. No mock number appears as if it were real | Screenshots of the dashboard and sidebar |
| 2.7 | Login page copy no longer promises TCPA compliance intelligence or Ask AI as live features | Screenshot |
| 2.8 | Design is consistent across Dashboard, Pings, Settings and call detail (fonts, spacing, colours); no broken layout at 1280px and 1920px | Screenshots |
| 2.9 | The "Show demo" toggle: default off; demo rows never counted in real KPIs | Screenshot + query |
| 2.10 | No console errors or CSP errors on any page | Console screenshot per page |

For any item Stage 2 did not touch, write NOT BUILT. Do not guess what was intended.

---

## Part 3 — Stage 3: Convert / Cancel (write-back UI) 🔴 money path

Requirements: `docs/writeback-phase2-spec.md` (Phase 2A data layer, Phase 2B list +
status + filter), and the Phase 3 (call detail) / Phase 4 (Convert/Cancel buttons) specs or
reports, if present. Owner decisions that must hold:
- The Calyx user is the Ringba **admin** (account owner).
- Convert and Cancel are **manual only**. Calyx executes the admin's decision; it never
  converts or voids on its own.
- The amount is **entered by hand**, with **no cap**, and must be confirmed in a popup
  before anything is sent.
- The conversion state read-back is `hasConverted` **AND** `conversionAmount > 0`.
  `hasConverted` stays `true` after a void.

### 3.1 Status derivation (one definition)
- One status derivation is used by the list, the filter, the call detail and the API. Show
  every place it is computed. More than one independent definition is a finding.
- "Should have converted" filter: the exact predicate, with file:line, in both the API
  (`pings.filters.ts`) and the web (`shouldHaveConverted`). They must match.
- Known issue from Stage 4 R0: platforms that never report conversion state (Retreaver
  today, TrackDrive, CallGrid) are flagged falsely. Confirm it and count the affected rows.
  It is being fixed in Stage 4 R1, so only record it here.

### 3.2 Call list
- Status column and filter work: screenshot, plus counts per status matching a direct
  ClickHouse query.
- Pagination: no duplicates or gaps when paging through a filtered list.

### 3.3 Call detail
- Every Ringba field shown matches ClickHouse for three calls. Pick them yourself, with
  different statuses.
- The empty state tells "not provided" apart from a real 0 / false.

### 3.4 Convert / Cancel controls
- Buttons only for platforms in `WRITEBACK_PLATFORMS` (Ringba). Never for others.
- Visible only to a user with the admin role. Prove it from both sides: the UI hides them,
  **and** the API rejects the call from a non-admin (test with a non-admin user or a unit
  test).
- Convert: the amount is required, must be a number > 0, has no cap. A confirmation popup
  shows the call, the amount and the direction before anything is sent. Cancel has its own
  confirmation.
- Double-click and double-submit are guarded: the button is disabled while in flight **and**
  the idempotency key is enforced server-side. Prove it with a test.
- The UI shows pending / confirmed / failed after the action, based on the read-back. Show
  where `hasConverted AND amount > 0` is applied.

### 3.5 Server-side guards (API + platform-sync)
List each guard with file:line, and prove each one with a test or a request:
- `WRITEBACK_ENABLED` kill switch: off means every write is rejected before any platform
  call
- per-tenant `writeback_enabled` flag
- CSRF on the convert/cancel endpoints: a request without a valid token is rejected
- **Tenant isolation:** a user of tenant A cannot act on a call of tenant B, **and** a
  write always uses the calling tenant's own stored credentials. Show the query that loads
  the credentials and its tenant filter. 🔴 if unclear.
- Amount validation server-side, not only in the UI
- The audit row is written for every attempt, including rejections and 409 in-progress
  (known gap K7: say whether it was fixed)
- Settle / read-back window: its value, and whether K10 (12s vs ~10.4s observed) was
  addressed

### 3.6 Auto-actions
Prove there is **no** code path that converts or voids without a human click: no job,
sweep, rule or retry that sends a write on its own. Search for every caller of the
convert/void functions.

### 3.7 Tests
List the write-back unit and integration tests and run them. Name every safety requirement
above that has no test.

### 3.8 Live money-path check (optional) 🙋
Only if the owner approves, and only on the sandbox call the owner names (the sandbox
campaign with internal publisher and buyer, $0 terms):
- one Convert at $1, then its read-back
- one Cancel, then its read-back
- the audit rows written

If not approved, write CAN'T VERIFY (live) and rely on 3.4–3.7.

---

## Part 4 — Cross-cutting

- All three PowerShell gates exit 0 on `c4e3c5d`: `verify-system`,
  `verify-ingestion-slice`, `verify-presentation`.
- Ringba sync still works on this branch. Run unit tests; do **not** switch a Ringba
  integration to Live just for this.
- Secret scan of every commit in `ba9c825..c4e3c5d`.
- Server deploy note: exactly what the server needs when this branch merges. List the images
  to rebuild, new `.env` variables, migrations, and one-off commands.

---

## Output — `docs/stage1-3-verify-report.md`

1. Part 0 facts, and the file-to-stage grouping
2. One scoreboard per stage: requirement → verdict → severity (Stage 3)
3. Findings, most severe first, each with proof
4. The list of requirements with no test
5. Server deploy note
6. **Merge recommendation:** ready to merge / merge after listed fixes / not ready, with the
   reason
7. Open questions for the owner
8. Honest limitations

Commit the report and evidence on `verify/stage1-3`, push that branch only, give the
commit hash, and **STOP**.

## Out of scope

Fixing anything · Stage 4 / Retreaver · intelligence services · server or nginx changes ·
pushing or merging to `main` or `convert-cancel-screen`.
