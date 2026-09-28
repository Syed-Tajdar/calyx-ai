# Calyx — S2b: S2 Regressions + Remaining Security

**Scope:** fix what S2 broke, then finish S2's open items. **No UI redesign** (that is the
UI-foundation slice). **No intelligence services, no S3, no Phase 2B. No push.**

**Deliverable:** `docs/s2b-report.md`

## ⛔ RULE
- Parts are independent. If one blocks, report and continue.
- **Proven live on rebuilt images, and in a real browser where it concerns the web app.**
  An HTTP 200 is not proof a page works.
- Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`.
- No commit whose only purpose is recording a commit hash.
- **Permitted state changes:** deleting the synthetic `csp-live-%` rows (R1.2, after the
  count is reported); installing Go on the host (P4). Nothing else.

---
---

# PART R — Regressions introduced by S2 (fix first)

## R1 — 🔴 Test data leaking into real KPIs

The operator's Ping Feed shows a row `csp-live-0d7b82a8-47…  TX  $12.50`. It comes from the
new Playwright gate (`check-dashboard-browser.mjs`) added in the S2 CSP fix. The test-data
exclusion helper only covers `apisync-`, `demo-`, `smoke-` — so **every gate run adds a fake
call and $12.50 to the dashboard.** This is exactly the contamination cleaned before S1.

**R1.1** — Make the browser gate generate `smoke-` prefixed call ids. Find **every** script
that sends pings (all `scripts/*`) and confirm each uses a `smoke-` prefix. Report the list.

**R1.2** — Report the count and total revenue of `csp-live-%` rows in ClickHouse `pings`,
and any other non-real prefix you find (anything that is not a real platform id and not
`apisync-` / `demo-` / `smoke-`). Then delete **only** `external_call_id LIKE 'csp-live-%'`
rows. Report before/after counts and default KPI totals before/after.

**R1.3 — Stop it recurring.** Add a check (unit or gate) that fails if any script under
`scripts/` generates a call id without the `smoke-` prefix. A new test prefix must not be
able to slip past the exclusion helper again.

## R2 — 🔴 Typography broken on every page

Every page renders in the browser's default **serif** font. S2 removed `next/font/google`
Inter ("Docker build flake") and reported "GeistSans only" — but Geist is not rendering
either. The design system is **Geist for headings + Inter for body.**

- Restore **both**, loaded from **local font files committed to the repo**
  (`next/font/local`) — no network fetch at build time, so the Docker flake cannot recur.
  Confirm both fonts' licenses allow bundling (both are OFL — state it).
- Find why the page fell back to serif (e.g. a CSS variable left undefined) and state it.
- **Proof in the browser:** computed `font-family` of `body` and of an `h1` on dashboard,
  pings, settings, call detail. Screenshots.
- **Gate:** extend the browser check to **fail** if the body's computed font is a generic
  serif / not the expected family.

## R3 — Signed-in user's name/email missing

The top-right shows "Account" instead of the user's email. Cause: the web app used to read
it from the token; the token is now correctly hidden in an httpOnly cookie. Load it from
`/auth/me` instead. **Proof:** screenshot showing the email.

## R4 — Static assets must stay cacheable

Middleware now sets `Cache-Control: no-store` so HTML pages carry a fresh nonce. Confirm the
middleware **matcher excludes** `/_next/static`, images and fonts. **Proof:** response
headers for one JS chunk, one CSS file and one font file — long-lived cache, not `no-store`.
If they were `no-store`, that is a finding: fix and re-prove.

---
---

# PART P — Remaining S2 items

## P1 — WebSocket origin check
`/live` accepts the session cookie. Prove the socket.io gateway **rejects a handshake from an
origin not in `WEB_ORIGINS`**. If it does not, add the check and prove it.

## P2 — Full production-mode dry run (api + web together)
S2 only ran **web** in production mode. Run **api and web** with `APP_ENV=production` and
freshly generated non-placeholder secrets (local override file, **never committed**). Over
`http://localhost` prove: login sets the `Secure` session cookie and it is accepted; the full
browser gate passes (login → KPIs → pages → live event → zero CSP / console errors →
correct fonts). Restore development mode afterwards and confirm the gates still pass there.

## P3 — Next.js advisory: facts before any upgrade (report only — do NOT upgrade)
For every remaining `npm audit` critical/high in `web`: advisory ID, affected versions, the
**fixed-in version on each release line (14 / 15 / 16)**, and **whether the vulnerable
feature is used by Calyx** (with file references). Then the smallest upgrade that clears it
and what it would break (React 19, APIs in use).

## P4 — Go 1.25
- Move every v1 Go module (`go.mod`) and Docker builder to the latest stable **Go 1.25.x**
  (check go.dev — do not guess the number).
- Bump `pgx`, `golang.org/x/text`, `ch-go` to fixed versions. Rebuild.
- Re-run `govulncheck ./...` per v1 module. Target: no findings; list any remaining with why.
- **Host:** Go is currently installed in **two places** (`C:\Program Files\Go` and a user
  path), both 1.24.13. Install 1.25.x and make sure **only one** Go is on `PATH` afterwards
  — a stale copy earlier in `PATH` would silently shadow the new one. Report `where go` and
  `go version` from a fresh shell.

---

## Gates, commit, scan
- All gates exit 0 in development mode: `verify-system` (including the browser check with the
  new font assertion), `verify-ingestion-slice`, `verify-presentation`.
- After the gates run, confirm **zero** non-real, non-excluded rows were added (the R1 fix
  works).
- Commit locally. Secret scan of every unpushed commit. **Do not push.**

## Output — `docs/s2b-report.md`
1. Scoreboard R1–R4, P1–P4: PASS / FAIL.
2. Per item: cause → change → live proof (screenshots for R2, R3, P2).
3. P3 advisory table and upgrade proposal.
4. `where go` / `go version` from a fresh shell.
5. Honest limitations; found but not fixed.

STOP. Do not push. Do not start the UI-foundation slice.

## Out of scope
UI redesign, colours, table styling, hiding intelligence UI (UI-foundation slice) · data
issues seen in the UI — Coverage vs Dashboard totals, duplicate rows in Ping Feed, ping chart
bucketing, Publishers card (UI-foundation slice) · intelligence services · S3 · Phase 2B.
