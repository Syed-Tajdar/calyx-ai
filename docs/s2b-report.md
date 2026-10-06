# Calyx — S2b Report

**Spec:** `docs/s2b-spec.md`
**Date:** 2026-09-30
**Environment:** local Docker stack, rebuilt images, macOS host
**Constraint honoured:** no push, no commit.

> **Scope note.** The S2b code largely landed earlier (commit `781a66a`, "land S2b hardening") but
> this report was never written. This closes that gap: every item below was **re-verified against a
> running stack today**, not taken from the earlier work. Items that could not be verified are
> marked `[UNKNOWN]` rather than assumed.

---

## 1. Scoreboard

| Part | Result |
|---|---|
| R1 Test data leaking into real KPIs | **PASS** — plus a **new leak found and fixed** (see R1.4) |
| R2 Typography broken | **PASS** |
| R3 Signed-in user's name/email missing | **PASS** |
| R4 Static assets must stay cacheable | **PASS** |
| P1 WebSocket origin check | **PASS** |
| P2 Full production-mode dry run (api + web) | **NOT RUN** `[UNKNOWN]` |
| P3 Next.js advisory facts (report only) | **PASS** — see §4 |
| P4 Go 1.25 | **PASS** |

**Gates:** `check-smoke-call-id-prefix` = 0 · `check-ws-origin` = 0 · API Jest **15/15** ·
`platform-sync` adapter tests **ok** · API image build = 0 · web image build = 0.
`verify-system` / `verify-ingestion-slice` / `verify-presentation` **not run** — see §5.

---

## 2. R1 — test data leaking into real KPIs

### R1.1 / R1.2 — the original `csp-live-` contamination

Current local ClickHouse, grouped by id prefix:

```
┌─prefix─┬─rows─┬─distinct_calls─┬─revenue─┐
│ demo-  │   53 │             53 │  802.75 │
└────────┴──────┴────────────────┴─────────┘
```

**Zero `csp-live-` rows and zero `proof-` rows** `[PROVEN]`. The browser gate now seeds
`smoke-csp-` (`check-dashboard-browser.mjs:324`) and `demo-seed.ps1` uses `smoke-demo-`.

Note: this is a database created fresh today, so absence here does **not** prove the production
database is clean. Re-run the prefix census against production before go-live.

### R1.3 — the guard

`scripts/check-smoke-call-id-prefix.mjs` exists and **is** wired into the gate
(`verify-system.ps1:137`).

### R1.4 — NEW FINDING: a second leak the guard could not see `[PROVEN]`

**Cause.** `services/platform-sync/internal/adapter/mock.go:84` emitted call ids as
`proof-{platform}-{id}` when `MOCK_DEDUP_PROOF` is on. `proof-` is **not** in
`TEST_DATA_ID_PREFIXES` (`apisync-` / `demo-` / `smoke-` only), so those rows would have counted
as **real revenue** in every default KPI — the identical bug class R1 was created to eliminate.

The R1.3 guard could never have caught it: it walked `scripts/` only, and matched only
`.mjs|.js|.ps1|.ts|.sh` — never `.go`, never `services/`.

**Change.**
1. `mock.go` now emits **`smoke-proof-…`**, so the *existing* exclusion covers it — the prefix list
   does not have to grow every time someone invents a name.
2. The guard now scans `services/` as well as `scripts/`, includes `.go`, and understands Go
   `ExternalCallID:` literals. `_test.go` is skipped (in-memory ids never reach ClickHouse).
3. `scripts/proof-content-hash-sync-c.ps1` and `scripts/prove-content-hash-ch.ps1` updated to the
   new prefix (ClickHouse `LIKE` patterns and the Redis dedup key scan).

**Proof `[PROVEN]`** — regression test, both directions:

```
# with the old prefix deliberately restored:
SMOKE PREFIX CHECK FAIL:
  services/platform-sync/internal/adapter/mock.go: ExternalCallID "proof-%s-%s"
      — must start with smoke-/demo-/apisync-
EXIT=1

# with the fix in place:
SMOKE PREFIX CHECK PASS (321 scripts scanned)
EXIT=0
```

Scan coverage went from ~34 files to **321**. `go build ./...` OK; adapter tests `ok`.

**Severity in practice:** latent, not active — `MOCK_DEDUP_PROOF` is `"0"` in compose and commented
out in `.env.example`. It would have fired the first time anyone ran the content-hash proof scripts.

---

## 3. R2 / R3 / R4 / P1 / P4 — verification

### R2 — typography `[PROVEN]`

**Cause (as recorded at `web/src/app/globals.css:14`):** `--font-geist` / `--font-inter` were
referenced but never defined, which made the whole `font-family` declaration invalid, so browsers
fell back to their default serif.

**State:** both fonts are committed locally as `.woff2` under `web/src/fonts/{geist,inter}/` with
their licences (OFL), loaded through `next/font/local` in `layout.tsx` (`variable: '--font-geist'`
/ `'--font-inter'`), mapped in `tailwind.config.js` to `font-heading` / `font-body` / `font-sans`.
No network fetch at build time, so the Docker flake cannot recur.

**Browser proof:** login page and dashboard render in the correct sans faces, not serif. The gate
already asserts computed fonts on three pages (`check-dashboard-browser.mjs:198-217, 255, 269`) and
fails on a generic-serif fallback.

### R3 — signed-in user's email `[PROVEN]`

Top-right shows **`admin@calyx.ai`**, not "Account" — observed in the browser after login.

### R4 — static assets cacheable `[PROVEN]`

| Resource | `Cache-Control` |
|---|---|
| `/_next/static/css/…css` | `public, max-age=31536000, immutable` |
| `/_next/static/chunks/webpack-….js` | `public, max-age=31536000, immutable` |
| `/_next/static/media/….woff2` | `public, max-age=31536000, immutable` |
| HTML page `/pings` | `no-store, no-cache, must-revalidate, private` |

Correct on both sides: assets cache for a year, HTML does not, so each page load gets a fresh CSP
nonce (observed: `script-src 'self' 'nonce-…'`).

### P1 — WebSocket origin check `[PROVEN]`

```
PASS: disconnected Origin=http://evil.example:3999 reason=io client disconnect
PASS: allowed Origin=http://localhost:3101 connected
P1 WebSocket origin check PASS   (exit 0)
```

Implemented at `live.gateway.ts:46-56` — cookie-authenticated sockets must present an Origin in
`WEB_ORIGINS`; script clients with no Origin must present `auth.token`.

**Correction worth recording.** An initial probe of
`GET /socket.io/?EIO=4&transport=polling` returned HTTP 200 with a session id for
`evil.example.com`, which looked like a bypass. It is not: that is the engine.io **transport**
handshake, which precedes namespace connection. The origin check runs on connection to the `/live`
**namespace**. Anyone re-testing this should use `scripts/check-ws-origin.mjs`, not curl.

### P4 — Go 1.25 `[PROVEN]`

All ten modules declare `go 1.25.0`; every service Dockerfile builds `FROM golang:1.25.14-alpine`.

*Not verified:* `govulncheck` per module, and `where go` / `go version` from a fresh shell — the
spec's host instructions are Windows-specific and this verification ran on macOS (host Go 1.27.1).

---

## 4. P3 — Next.js advisory: facts before any upgrade

`npm audit --omit=dev --package-lock-only` on `web` (current: **next ^14.2.35**):

**1 critical, 0 high.** All of it is `next` itself — six advisories:

| Advisory | Issue | Vulnerable | Used by Calyx? |
|---|---|---|---|
| GHSA-9g9p-9gw9-jx7f | DoS via Image Optimizer `remotePatterns` | `>=10.0.0 <15.5.10` | **No** — no `images` config in `next.config.mjs`; `next/image` never imported |
| GHSA-3x4c-7xq6-9pq8 | Unbounded `next/image` disk cache | `>=10.0.0 <15.5.14` | **No** — same |
| GHSA-ggv3-7p47-pfv8 | HTTP request smuggling in **rewrites** | `>=9.5.0 <15.5.13` | **No** — no `rewrites` in `next.config.mjs` |
| GHSA-h25m-26qc-wcjf | RSC request deserialization DoS | `>=13.0.0 <15.0.8` | **Unlikely** — no `'use server'` / server actions anywhere |
| GHSA-q4gf-8mx6-v5v3 | DoS with Server Components | `>=13.0.0 <15.5.15` | **Minimal surface** — see below |
| GHSA-8h8q-6873-q5fj | DoS with Server Components | `>=13.0.0 <15.5.16` | **Minimal surface** — see below |

**Server-component surface is one file.** Every route under `web/src/app/(app)/` and `(auth)/` is
`'use client'`. The only server component is `web/src/app/page.tsx`, which is a three-line
`redirect('/login')`. Layouts still render server-side, so the surface is not zero — but there is
no data-fetching or user-input-driven RSC path.

**Smallest upgrade that clears all six:** **`next@15.5.16`**. The highest lower-bound across the
six ranges is `<15.5.16`; everything below that on the 15 line is still vulnerable to at least one.

**There is no fix on the 14 line.** All six ranges span the whole of 14.x. Staying on 14.2.35 means
staying formally vulnerable to all six regardless of patch level.

**What 14 → 15 would cost (assessment, not a recommendation):**
- React 19 is the expected pairing for Next 15 App Router; the app is on React 18.3. This is the
  main risk and needs its own test pass.
- 15 changed caching defaults (`fetch`, route handlers, client router) from cached-by-default to
  uncached-by-default. Calyx fetches client-side through `web/src/lib/api.ts`, so exposure is low.
- Async request APIs (`cookies()`, `headers()`, `params`) became promises. Calyx uses `middleware.ts`
  and `useParams()`; a compile pass will surface any breaks.
- Storybook 8, Tailwind 3, `react-simple-maps`, `recharts` would all need a compatibility check.

**Mitigation available today without upgrading:** all six are denial-of-service, not RCE or data
disclosure. The deploy checklist already requires the web app to sit behind nginx with rate
limiting and not be directly exposed. That reduces, but does not remove, the exposure.

**Decision needed from management (Decision 5):** accept a 14.x with no available fix, or fund a
15.5.16 upgrade with a React 19 test pass.

---

## 5. Honest limitations

1. **P2 — full production-mode dry run: NOT RUN `[UNKNOWN]`.** Running `api` + `web` with
   `APP_ENV=production` and freshly generated secrets was not performed. Note the interaction worth
   knowing: compose **hardcodes** `APP_ENV: "development"` for `ingestion`, `consumer`, `api` and
   `platform-sync` — only `web` reads it from `.env`. So P2 requires a compose override file, not an
   env change. And `env.ts:71-93` refuses production boot while any secret is still a placeholder,
   so real secrets must be generated in the same step.

2. **`verify-system`, `verify-ingestion-slice`, `verify-presentation` not run.** All three are
   PowerShell, and `pwsh` is not installed on this macOS host. The Node-based gates
   (`check-smoke-call-id-prefix.mjs`, `check-ws-origin.mjs`) were run and pass. This is a real
   coverage gap on this machine, not a code problem.

3. **`govulncheck` not re-run** per module (P4 asked for it).

4. **Prefix census is local only.** The production database at `calyx.ridgerisemedia.com` was not
   inspected; `csp-live-` / `proof-` rows may exist there.

5. **The new guard is still pattern-based.** It now covers Go `ExternalCallID:` literals and the
   script patterns, but a sufficiently indirect construction (id assembled from variables across
   several lines) would still pass. It raises the bar; it does not make the class impossible.

---

## 6. Found but not fixed

- **`.github/workflows/go.yml` cannot pass** — runs `go build ./...` from the repo root, but there
  is no root `go.mod` (ten separate modules), and it pins **Go 1.20** against the repo's 1.25.
  This is audit F045 / K3, still open. Belongs to step 5 (Operations).
- **Publishers card shows invented data regardless of the demo toggle.** With demo rows hidden the
  dashboard says *"No pings yet — connect Ringba to start receiving data"* while the Publishers card
  below shows MediaCo / LeadStream / DialPro / CallStream with percentages and revenue, hardcoded in
  `web/src/lib/demo-publishers.ts`. Same credibility class as C1–C3. Belongs to step 2 (UI clean-up).
- **Coverage page has no period filter** — `coverage/page.tsx:38` sends only `show_demo`, and the
  page never states that it is showing all time, while the dashboard filters by period. This is
  enterprise-roadmap **Gap C**, still open. Belongs to step 2.

---

## STOP

No push. No commit. Working tree holds the changes for operator review.
