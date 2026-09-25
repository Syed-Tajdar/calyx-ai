# S2 CSP Fix Report

**Date:** 2026-09-25  
**Scope:** Content-Security-Policy broke Next.js hydration (login + dashboard). No push. Do not start S3.  
**Status:** FIXED — Playwright gate fails on broken build, passes after fix; all three system gates exit 0.

---

## 1. Root cause (with evidence)

### What the browser showed

On `/login`, Chromium reported ~5×:

> Executing inline script violates the Content Security Policy directive `script-src 'self' 'nonce-…' 'unsafe-eval' 'unsafe-inline'`. Note that `'unsafe-inline'` is ignored if either a hash or nonce value is present in the source list.

React never hydrated: the login form did not submit, and authenticated pages stayed on loading skeletons. This was **not** an IDE-browser limitation.

### Which scripts were blocked

The five blocked scripts are **Next.js App Router flight / bootstrap inlines** — they push into `self.__next_f` (RSC payload), with **no `nonce` attribute**:

| Evidence | Result |
|----------|--------|
| `docs/evidence/s2-csp-fix/login_body_before.html` | `total_scripts=15`, **inline=5**, **with_nonce=0** |
| Inline bodies | `(self.__next_f=self.__next_f\|\|[]).push([0])…` and subsequent `self.__next_f.push([1,"…"])` flight chunks |

External `/_next/static/chunks/*.js` tags were also un-nonced on the cached document.

### Why the middleware nonce never reached them

Two independent failures:

1. **CSP was set only on the response.** Next.js App Router reads the nonce from the **request** `Content-Security-Policy` header (and stamps it onto framework scripts during SSR). Setting CSP only on `NextResponse` left every script without `nonce="…"`. With a nonce present in `script-src`, the browser **ignores** `'unsafe-inline'`, so the five flight inlines are blocked.

2. **The HTML document was statically cached.** Before fix, `/login` headers showed:

```text
x-nextjs-cache: HIT
Cache-Control: s-maxage=31536000, stale-while-revalidate
```

(`docs/evidence/s2-csp-fix/login_headers_before.txt`)

A cache HIT / 304 cannot carry the **current** request’s nonce. Middleware minted a fresh nonce into the CSP header while the body still had zero (or stale) nonces → guaranteed mismatch.

Root layout was eligible for static prerender (no `force-dynamic`), which is how `/login` ended up as a long-lived cached document.

### Fail-before gate (broken build)

`scripts/check-dashboard-browser.mjs` against the broken image:

```text
=== 5/14 checks passed ===
FAIL login redirects to /dashboard
FAIL zero CSP violations: 25: Executing inline script violates…
FAIL zero console/page errors: pageErrors=4 … Minified React error #423
```

Evidence: `docs/evidence/s2-csp-fix/browser_fail_before.txt`, screenshots under `docs/evidence/s2-csp-fix/before/`.

---

## 2. Fix

### `web/src/middleware.ts`

- Generate per-request nonce.
- Set **both** `Content-Security-Policy` and `x-nonce` on **request** headers passed into `NextResponse.next({ request })` so Next SSR stamps scripts.
- Mirror CSP on the response.
- `Cache-Control: no-store, no-cache, must-revalidate, private` (+ `Pragma: no-cache`) so HTML with a one-shot nonce is never cached / 304’d.
- **No `'unsafe-inline'` for scripts** (dev or prod).
- Production (`APP_ENV=production`): `script-src 'self' 'nonce-…' 'strict-dynamic'` only.
- Development: same + `'unsafe-eval'` for React Refresh / webpack HMR only.

### `web/src/app/layout.tsx`

- `export const dynamic = 'force-dynamic'` so every page is rendered per request with the request CSP nonce (static prerender has no per-request nonce).
- Dropped `next/font/google` Inter (Docker build flake); GeistSans only.

### After-fix headers / HTML

| Check | Before | After |
|-------|--------|-------|
| Cache | `x-nextjs-cache: HIT`, `s-maxage=31536000` | `cache-control: no-store, no-cache, must-revalidate, private` |
| Inline scripts | 5, **0 nonced** | 5, **all nonced** (`with_nonce=15` total) |
| Prod CSP | n/a (broken) | `script-src 'self' 'nonce-…' 'strict-dynamic'` — **no** `unsafe-inline`, **no** `unsafe-eval`; HSTS set |

Evidence: `login_headers_after.txt`, `login_body_after.html`, `prod_csp_headers.txt`.

---

## 3. Playwright proof (real Chromium)

Gate: `scripts/check-dashboard-browser.mjs` (system Chrome, headless).

For each of **APP_ENV=development** (compose web) and **APP_ENV=production** (one-shot `deploy-web` with `APP_ENV=production`, then compose restored):

1. Type credentials into `/login` and submit → `/dashboard`
2. Dashboard shows real KPI data (Total Revenue + metric cards), not skeletons
3. Open Pings, Settings, call detail — content renders on each
4. Seed a Ringba ping; **live feed receives matching `ping_received`**
5. Zero CSP violations and zero console/page errors

| Mode | Result | Evidence |
|------|--------|----------|
| development | **14/14 PASS**, exit 0 | `browser_pass_dev.txt`, `after_dev/*.png`, `after_dev/browser_console.json` |
| production | **14/14 PASS**, exit 0 | `browser_pass_prod.txt`, `after_prod/*.png`, `after_prod/browser_console.json` |

Sample live events: `csp-live-f7c50fde-…` (dev), `csp-live-04d33ce0-…` (prod).

---

## 4. Gate wiring (catch this class of break)

- Extended `scripts/check-dashboard-browser.mjs`: full login→KPI→pages→live-event path; fails on any CSP violation, console/page error, skeleton gate, or empty page.
- Wired into `scripts/verify-system.ps1` as **Browser CSP + hydration (Playwright)** with `hardFail` on non-zero exit.

**Fail → pass:**

| Build | Gate result |
|-------|-------------|
| Broken (response-only CSP + cached HTML) | exit **1**, 5/14 — `browser_fail_before.txt` |
| Fixed | exit **0**, 14/14 — `browser_pass_dev.txt` / `browser_pass_prod.txt` |

---

## 5. Gates / commit / scan

| Gate | Exit | Evidence |
|------|------|----------|
| `scripts/verify-system.ps1` | **0** (`SYSTEM VERIFY PASS`, `BrowserCSP: PASS`) | `docs/evidence/s2-csp-fix/verify_system.txt` |
| `scripts/verify-ingestion-slice.ps1` | **0** | `docs/evidence/s2-csp-fix/verify_ingestion.txt` |
| `scripts/verify-presentation.ps1` | **0** | `docs/evidence/s2-csp-fix/verify_presentation.txt` |

Commit: `7cbc5e5` (local only — **do not push**). Secret scan of the staged/unpushed commit: **CLEAN** (no JWT / AKIA / ghp / sk_live / private-key shapes; known dev placeholders only).

---

## 6. STOP

S2 CSP blocker closed. **Do not push. Do not start S3.**
