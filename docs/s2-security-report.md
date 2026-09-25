# Calyx — S2 Security Baseline Report

**Spec:** `docs/s2-security-spec.md`  
**Date:** 2026-09-25  
**Constraint:** no push; operator reviews (incl. visual check after Next.js 14.2.35).  
**Evidence:** `docs/evidence/s2-security/`

---

## 1. Scoreboard

| Part | Result |
|---|---|
| S2.1 platform-sync internal auth | **PASS** |
| S2.2 Login rate limiting | **PASS** |
| S2.3 httpOnly session + CSRF + WS | **PASS** |
| S2.4 Security headers + CSP | **PASS** |
| S2.5 CORS allowlist | **PASS** |
| S2.6 Validation / error bodies | **PASS** |
| S2.7 Production secret boot refuse | **PASS** |
| S2.8 Non-root containers | **PASS** |
| S2.9 Dependency / toolchain vulns | **FAIL** |

**Gates (final):** `verify-system` = 0 · `verify-ingestion-slice` = 0 · `verify-presentation` = 0  
[PROVEN] — `docs/evidence/s2-security/gate_*_retry.txt`

---

## 2. Chosen numbers (with reasoning)

| Setting | Value | Why |
|---|---|---|
| Login fail / email | **5 / 15 min** | Blocks credential stuffing on a single account without long lockouts for typos. |
| Login fail / IP | **30 / 15 min** | Higher than email so shared NAT/office egress is less likely to lock everyone; still bounds spray. |
| JWT / session lifetime | **24h** | Matches existing Nest `signOptions.expiresIn: '24h'`. At the 24h ceiling in the spec — acceptable; propose **8–12h** if product wants tighter idle risk without daily re-login friction. |
| CSRF | **Origin allowlist (`WEB_ORIGINS`) + double-submit** (`calyx_csrf` cookie + `X-CSRF-Token` header) | Required once credentials are cookie-auto-sent. Bearer (`Authorization`) skips CSRF so gates/scripts keep working. |
| Script vs browser | Browser login returns **no** `access_token` (cookie only). Header `X-Calyx-Client: script` returns Bearer token for gates. | Keeps JWT out of JS-readable storage while preserving automation. |
| Existing sessions | **Re-login once** | localStorage JWTs are obsolete after this change. |

Code: `services/api/src/auth/auth-security.service.ts:7-16`, `auth.module.ts` (`expiresIn: '24h'`), `auth.controller.ts:60-65`.

---

## 3. Per-part before / after + live proof

### S2.1 — platform-sync `/sync` + `/test-auth` auth (F011) — PASS

**Before:** unauthenticated POST on `:8088`.  
**After:** same bearer as write-back / `/sync/window` via `writeback.RequireBearer`  
`services/platform-sync/cmd/server/main.go:136-148`.  
API callers send `WRITEBACK_INTERNAL_TOKEN`: `integrations.service.ts:203-212`, `:451-460`.

**Proof [PROVEN]** (`s21_auth.txt`, `s21_api_e2e.txt`):  
`sync_no=401 sync_bad=401 test_bad=401` · API Test connection / Sync now `test_ok=True sync=sync_triggered`.

---

### S2.2 — Login rate limiting (F007) — PASS

**After:** Redis counters per email + IP; `Retry-After` on 429; generic body (no email-existence leak).  
XFF trusted only when peer ∈ `TRUSTED_PROXIES` (`auth-security.service.ts:27-47`). Empty list = socket address only.

**Proof [PROVEN]** (`s22_rate.txt`, `s22_other_email.txt`):  
`rate_codes=401,401,401,401,401,429,429` · other email unaffected · success after clear = 200.

---

### S2.3 — httpOnly cookie session + CSRF (F009) — PASS

**Browser:** `calyx_session` httpOnly; `calyx_csrf` readable; `SameSite=Lax`; `Secure` only when `APP_ENV=production`; `COOKIE_DOMAIN` from config (empty locally).  
Web: no token in `localStorage`/`sessionStorage` (`web/src/lib/auth.ts`); fetches use `credentials: 'include'` (`api.ts:182`).  
WS `/live` accepts session cookie (`live.gateway.ts:45-48`).  
CSRF: global `CsrfGuard` (`auth.module.ts` APP_GUARD) — covers write-back `convert`/`void`.

**Proof [PROVEN]:**  
- Login body has csrf only, no JWT (`s23_login_body.txt`).  
- `calyx_session httpOnly=True` (`s23_cookies.txt`).  
- Cross-origin POST → 403 (`s23_csrf.txt`).  
- Live feed: WS smoke PASS under `verify-system` (cookie/script auth path).

---

### S2.4 — Security headers + CSP (F022) — PASS

**After:** `web/src/middleware.ts` — CSP enforced; `default-src 'self'`; `connect-src` self + API/WS from config; `frame-ancestors 'none'`; script nonces + `strict-dynamic` in production; **no `unsafe-eval` in production** (dev keeps it for React Refresh).  
`style-src 'unsafe-inline'` retained — Next/Tailwind inline styles; scripts use nonces instead.  
HSTS only when `APP_ENV=production`.

**Proof [PROVEN]:** headers on `/login` include CSP + nosniff + referrer + Permissions-Policy; development build has no HSTS (local http). Gate web routes all 200. Console CSP violation sweep not automated in CI — operator visual check after Next upgrade remaining.

---

### S2.5 — CORS allowlist (F023) — PASS

**Before:** `origin: true` (reflect any).  
**After:** `WEB_ORIGINS` allowlist + `credentials: true` (`main.ts:24-34`).

**Proof [PROVEN]** (`s25_cors.txt`): `allow=http://localhost:3001 deny=` (no ACAO for foreign origin).

---

### S2.6 — Validation errors / error bodies (F024) — PASS

**After:** `GlobalExceptionFilter` — `ZodError` → 400 + field messages; unexpected → 500 generic + `request_id` (`global-exception.filter.ts:26-61`).  
ISO date params: `common/date-params.ts` used by analytics/pings.

**Proof [PROVEN]:**  
- `bad_login=400` body `Validation failed` / `email` (`s26_bad_login_body.json`).  
- `from=garbage` → 400 (`s26_bad_from_body.json`).  
- Unexpected Error → 500 without stack: unit test PASS (`s26_filter_unit.txt`).

---

### S2.7 — Refuse production boot without real secrets (F010) — PASS

**After:** `APP_ENV` (`development` default). Production refuse in api (`env.ts:72-93`), platform-sync, consumer, ingestion.

**Proof [PROVEN]:**  
- api: refuses `JWT_SECRET, CREDENTIAL_ENCRYPTION_KEY, WRITEBACK_INTERNAL_TOKEN` (`s27_api_refuse.txt`).  
- platform-sync: `CREDENTIAL_ENCRYPTION_KEY ANI_ENCRYPTION_KEY WRITEBACK_INTERNAL_TOKEN DATABASE_URL` (`s27_ps_refuse.txt`).  
- consumer / ingestion refuse files present. Dev compose `APP_ENV=development` unchanged.

---

### S2.8 — Non-root containers (F027) — PASS

**After:** `platform-sync/Dockerfile` `USER nonroot` (uid 65532).

**Proof [PROVEN]** (`s28_users.txt`):

| Container | Runtime user |
|---|---|
| api | `uid=1000(node)` |
| web | `uid=1000(node)` |
| consumer | `uid=65534(nobody)` |
| ingestion | `uid=65534(nobody)` |
| platform-sync | `uid=65532(nonroot)` |

---

### S2.9 — Dependencies / toolchain (F025, F026, F054) — FAIL

**Done:**
- **web:** Next.js **14.2.35** (patched within v14), PostCSS 8.5.28, socket.io-parser 4.2.7, `d3-color` override ^3.1.0 (clears prior high).  
- **api:** multer override ^2.2.1, socket.io-parser ^4.2.7 — **no high/critical** remaining (moderate only).  
- **Go:** `go.mod` → 1.24; Docker builders → **`golang:1.24.13-alpine`**; `go-redis` → **v9.7.3**.  
- Web production build PASS; gate routes render.

**Blocked / remaining (honest):**

1. **Next.js critical advisories remain on 14.2.35.** `npm audit fix --force` wants **next@16.3.6** (major). Spec: do not silently rewrite on major — **reported, not applied.** Operator must plan Next 15/16 migration + visual QA.  
2. **govulncheck** on v1 Go modules still reports findings. Latest **1.24.13** stdlib vulns are fixed only in **Go ≥1.25.x**. Clearing `pgx@v5.9.2`, `golang.org/x/text@v0.39.0`, `ch-go@v0.65` also requires **go ≥1.25**. Not inferred into a Go 1.25 jump in this slice.  
3. **api** still has moderate Nest/body-parser/qs/file-type issues; fixing cleanly wants Nest 12 (major).

#### npm audit tables

| Surface | Before (audit F025/F026) | After (`npm audit --omit=dev`) |
|---|---|---|
| **web** | 1 critical + 4 high (Next, PostCSS, socket.io-parser) | **1 critical** (Next 9.5–15.5.23 range) — high PostCSS/socket.io-parser/d3-color cleared |
| **api** | 3 high (multer, socket.io-parser, …) | **0 high/critical** — 8 vulns (1 low, 7 moderate) |

#### govulncheck (v1 modules, runner `golang:1.24.13` + govulncheck v1.1.4)

| Module | After summary |
|---|---|
| platform-sync | 18 findings — mostly stdlib needing Go ≥1.25.x; plus `pgx@v5.7.2`, `x/text@v0.21.0` |
| consumer | 13 findings — stdlib ≥1.25 + `ClickHouse/ch-go@v0.61.5` |
| ingestion | 15 findings — same pattern as platform-sync |

Redis GO-2025-3540 cleared via v9.7.3.

#### Host Go (operator action)

Current host: **`go1.22.12 windows/amd64`** (`host_go_version.txt`).  

**Install:** [`go1.24.13.windows-amd64.msi`](https://go.dev/dl/go1.24.13.windows-amd64.msi) to match v1 Docker builders.  

To clear remaining stdlib/module advisories later: move modules + images to **Go ≥1.25.13** (or current stable on go.dev) — out of this S2 pass without a deliberate language-line bump.

---

## 4. Deploy checklist

Already under **Infrastructure** in `docs/deploy-checklist.md`:

- Webhook ingestion not publicly exposed in v1  
- `COOKIE_DOMAIN`, `WEB_ORIGINS`, `APP_ENV=production`, trusted-proxy for client IPs  
- HTTPS + Secure cookie + HSTS in production  

---

## 5. Honest limitations / found but not fixed

- **S2.9 FAIL:** Next.js major (16) required for zero critical; Go ≥1.25 + dep bumps required for zero govulncheck noise on current DB.  
- **CSP style `unsafe-inline`:** kept for Next/Tailwind; scripts use nonces in production.  
- **Browser console CSP violation matrix:** not captured as automated evidence; operator visual check after Next upgrade.  
- **Injected inline script block:** middleware design blocks non-nonce scripts in prod; not separately screenshot-proven in this run.  
- **Some non-gate scripts** still omit `X-Calyx-Client: script` (e.g. older prove-*.ps1) — gates adapted; product not weakened.  
- **Ingestion gate** once flaked on default-view KPI race; re-run exited 0.  
- Out of scope per spec: intelligence, webhook body limits (F020/F021/F059), CI, alerts, S4 UI, Retreaver, Phase 2B.

---

## 6. Commit / secret scan

Local commit: *(filled after commit)*  

Secret scan of unpushed commits: *(filled after scan)*  

**Not pushed.** STOP. Do not start S3.
