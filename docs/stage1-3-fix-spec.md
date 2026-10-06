# Calyx — Stage 1–3 fixes (from the audit) + merge preparation

**Input:** `docs/stage1-3-verify-report.md` (`verify/stage1-3` @ `70b46ff`)
**Output:** `docs/stage1-3-fix-report.md`
**Operator:** Talha · **Owner:** Syed
Where this file and the audit or earlier specs disagree, this file wins.

---

## Branch decision (owner)

Stage 4 (`stage4/retreaver`) is built **on top of** `c4e3c5d` (Stages 1–3), and this
machine's database is already at migration **28** (Stage 4). Fixing on
`convert-cancel-screen` cannot even start the stack (the audit hit
`no migration found for version 28`).

So:
- Make all fixes **on `stage4/retreaver`**, in separate commits per item, using the prefix
  `fix(stage1-3): …`.
- Stages 1–4 then merge to `main` **together, once**, after the owner reviews this report.
  The audit of 1–3 and the reviews of Stage 4 R0–R2 are the review record.
- **Do not merge or push to `main`.** Push the branch only.
- R3 does not start until this is accepted.

---

## ⛔ Rules

1. Verify before changing. Every fix ends with proof: a test, a command and its output, or
   a browser screenshot. Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`.
2. Only the items below. If you find something else, record it under "Found, not fixed".
3. Never print or commit secrets. No Retreaver or Ringba sync switched to Live. No live
   Convert/Cancel, except Part E, and only with the owner's approval 🙋.
4. Gates: `pwsh` was installed in Stage 4 R1 as **`pwsh-preview`**, not `pwsh`. Run the gates
   with that binary. All three must pass before the report: `verify-system`,
   `verify-ingestion-slice`, `verify-presentation`.
5. Stop and ask at every 🙋.

---

## Part A — Money path 🔴

**A1 — Remove the $25 cap (owner decision: no cap).**
- Remove `WRITEBACK_MAX_AMOUNT = 25` (`web/src/lib/api.ts:586`), the input `max`, and the
  "disabled above 25" logic (`ConversionActions.tsx:86-88`). Remove the comment that claims
  the server enforces a cap.
- The server stays as it is: amount must be finite and > 0, with no upper limit.
- The safety net is the confirmation popup. It must show the call ID, the platform, the
  direction (Convert / Cancel) and the amount written large, e.g. "Convert this call for
  **$57.00**?". The admin confirms there.
- Tests: the server accepts 26 and 1000. The server rejects 0, a negative amount, NaN and
  a string. The UI allows confirming more than 25.

**A2 — One conversion-state definition, everywhere (owner rule: flag AND amount > 0).**
Today the list filter and call detail use `has_converted` alone, while write-back uses
`hasConverted && revenue > 0` (`convert.go:103-105`). Define the state **once, in the API**.
Stamp it on every ping row, the way `conversion_reported` is stamped today. The web reads
the stamp and holds no logic of its own:

| State | Rule | Shown as |
|---|---|---|
| `converted` | reports conversion AND flag true AND revenue > 0 | **Converted** |
| `converted_zero` | reports conversion AND flag true AND revenue = 0 | **Converted · $0** (muted) |
| `not_converted` | reports conversion AND flag false | **Not converted** |
| `not_reported` | platform does not report conversion state | **n/a** |

- The filters **Converted** / **Not converted** / **Should have converted** and the call
  detail "Converted" field all use this one value.
- "Should have converted" stays: `not_converted` AND (revenue > 0 OR duration ≥ required).
- Proof: counts per state on the current database (Retreaver: the 119 flag-with-$0 rows
  must land in `converted_zero`), and the same call showing the same state in the list,
  the filter and the call detail. Ringba's write-back read-back must agree with
  `converted`.

**A3 — An audit row for every write-back attempt (K7).**
Write an audit row for **every** request that reaches the write-back service, with the
outcome and the reason. That includes the 409 already-in-progress lock rejection
(`convert.go:167-171`) and the rejections before the lock: invalid amount, integration not
found, write-back disabled (tenant or global), not live. Tests for each outcome.

**A4 — Hide Convert/Cancel from non-admins in the UI.**
The API already returns 403. The component must also read the user's role (from
`/auth/me`) and render nothing for non-admins. Add a component test, and a screenshot with
a non-admin user (create a local test user; do not change a real account).

**A5 — Settle window (K10).** Make `SettleWindow` configurable (`WRITEBACK_SETTLE_SECONDS`)
with a default of **20 s** (12 s was tight against the observed ~10.4 s). Document it.

**A6 — Tests the audit found missing:** Jest tests for the admin-only 403 and for CSRF
(cookie session without a token → 403) on the convert and cancel routes, plus the tests in
A1–A4.

---

## Part B — v1 scope and honesty in the UI

**B1 — Ping chart follows the period.** Remove the hardcoded `buildFilledPingTimeline(…, 7)`
and the literal "Last 7 days" badge (`dashboard/page.tsx:121-123`, `PingChart.tsx:31`).
- 7 days → daily buckets
- 30 days → daily buckets
- All time → weekly or monthly buckets, whichever keeps under ~60 points
- The badge says the selected period.

Proof: one screenshot per period, and the chart total matches the KPI ping count for that
period.

**B2 — v1 platforms only.** Behind the existing `v1-features.ts` flag:
- Platform filters (Dashboard, Pings, Coverage) show **All, Ringba, Retreaver** only.
- Settings "Connect a platform" shows **Ringba, Retreaver** only.
- The **Webhook** connection type is hidden (v1 is API key only).

Code for the other platforms stays; nothing is deleted.

**B3 — Login page copy.** Remove the TCPA-compliance-intelligence and Ask AI claims
(`login/page.tsx:62,73`). Write accurate v1 copy, e.g. "Pay-per-call analytics for Ringba
and Retreaver — every call, revenue and conversion in one place." Keep it short.

**B4 — Call detail: no mock intelligence.** Behind the same v1 flag, hide the journey steps
Score and Compliance, the TCPA risk number, the call quality score, and the Ad attribution
"Mock · Fired" block. A mock number must never look like real data.

**B5 — Duration labels (approved in Stage 4 R2).** Call detail: "Connected length" →
**"Talk time"**, "Total call length" → **"Total duration"**.

**B6 — Profit (gross).** Show `—` when `profit_gross` is missing, instead of `$0.00`
(`CallDeepViewPanel.tsx:184-185`). A real 0 still shows `$0.00`.

**B7 — Coverage vs Dashboard (12-call gap).** Keep the numbers. Make the gap visible: the
Coverage header shows e.g. "3,052 calls across 53 regions · 12 without a US/CA state". Then
the two totals reconcile on screen.

**B8 — Settings: pause/resume instead of soft-delete (Stage 4 open issue).**
- Add **Pause / Resume** for an integration. It sets `is_active` false/true, and a paused
  integration **stays visible** in Settings, marked "Paused".
- Delete stays a separate action, with its own confirmation.
- Add an API route for it: admin-only, CSRF-protected, and audited like other settings
  changes.

This is what Rule 10 (one live sync per account) depends on.

---

## Part C — Small cleanups

- **C1** The sweep log line says `ringba window fetch completed` for any platform. Use the
  real platform name.
- **C2** `docs/platform-certification.md`:
  - Update the header to Stage 4 R2, 2026-10-02.
  - Replace "TrackDrive Phase 3 may start" with: v1 = Ringba + Retreaver; TrackDrive /
    CallGrid out of v1 scope.
- **C3** `govulncheck` on **every** v1 Go module (the audit scanned only platform-sync).
  Report the results.
- **C4** Next.js `npm audit` critical: **report only, no upgrade.** List each advisory and
  whether Calyx uses the affected feature. This decision goes to management.

---

## Part D — Re-verify (on the fixed branch)

- All three PowerShell gates pass (Rule 4).
- Console/CSP: open Login, Dashboard, Pings, Coverage, Settings and one call detail. Capture
  the browser console for each and show **zero errors** (audit item 2.10 was CAN'T VERIFY).
- Re-run the audit checks for 2.3, 2.5, 2.6, 2.7, 3.1, 3.4 and 3.5 (audit row) and give
  each a verdict.
- Ringba no regression (unit tests); Retreaver R2 numbers unchanged (Dashboard all-time
  Retreaver revenue and call count, same as before the fixes).
- Server deploy note: which images to rebuild (api, web, platform-sync), migrations
  (`000028` plus anything new, in order), and new env vars (`WRITEBACK_SETTLE_SECONDS`).
  Also note that `WRITEBACK_ENABLED` stays `false` on the server.

## Part E — Live money-path check 🙋 (only after A–D pass)

Only with the owner's explicit approval, and only on the Ringba sandbox call the owner
names (campaign `CALYX-SANDBOX-DO-NOT-USE`, internal publisher and buyer, $0 terms):
1. One Convert at **$1**. Show the read-back and the audit row.
2. One Cancel. Show the read-back and the audit row.
3. Show the UI state before and after.

If the owner does not approve, write CAN'T VERIFY (live) and stop.

---

## Output — `docs/stage1-3-fix-report.md`

1. Scoreboard: each item → DONE / NOT DONE, with proof.
2. Re-verified audit items with their new verdicts.
3. Gates and console output.
4. Server deploy note.
5. Found, not fixed.
6. Merge recommendation for `stage4/retreaver` → `main`.

Commit per item on `stage4/retreaver`, push the branch only, give the hashes, and **STOP**.
Do not merge. Do not start R3.
