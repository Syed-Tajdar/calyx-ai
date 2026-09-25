# Calyx — S2: Security Baseline (v1)

**Scope:** close the security findings on surfaces v1 actually exposes. **No intelligence
services, no CI, no S4 UI scope, no Retreaver, no Phase 2B. No push.**

**Deliverable:** `docs/s2-security-report.md`

---

## ⛔ RULE

- Parts S2.1–S2.9 are **independent**. If one blocks, report it and continue.
- **Proven live on rebuilt images.** Reading code is not proof.
- Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`.
- **Local development must keep working** (http on `localhost`, dev placeholders). Hardening
  applies in production mode; dev mode stays usable.
- **The gates must keep working.** Scripts log in and call the API; if an auth change breaks
  them, adapt the scripts — never weaken the product to satisfy a script.

## Operator decisions (recorded)

| Decision | Value |
|---|---|
| Webhook ingestion in v1 | **Not publicly exposed.** v1 connects platforms by API key only. The `ingestion` service keeps running internally (the gates' smoke tests use it) but is never published to the internet. |
| Domain | Not purchased yet. Web and API will share **one parent domain** (e.g. `app.<domain>` / `api.<domain>`). Everything domain-specific must be **configuration**, not code. |

---
---

## S2.1 — platform-sync internal routes require auth (F011)

`POST /sync` and `POST /test-auth` are unauthenticated on `:8088`.
- Require the same internal bearer token as the write-back and `/sync/window` routes.
- Find **every caller** (API "Sync now", "Test connection", scripts) and make them send it.
- **Proof:** no token → 401; wrong token → 401; API "Sync now" and "Test connection" still
  work end-to-end.

## S2.2 — Login rate limiting (F007)

- Limit per **email** and per **IP**, **Redis-backed** (works across replicas). Choose and
  justify the numbers (e.g. 5 failed / 15 min per email, higher per IP).
- Exceeded → **429** with `Retry-After`. The response must **not** reveal whether the email
  exists.
- Client IP: trust `X-Forwarded-For` **only** from a configured trusted proxy — otherwise
  anyone can fake their IP and bypass the limit.
- **Proof:** N+1 failed logins → 429; a correct login after the window → success; a
  different email is unaffected.

## S2.3 — Session token in an httpOnly cookie (F009)

Today the JWT lives in `localStorage` / `sessionStorage` — any script on the page can read it.

**Browser:**
- Login sets the session in an **`httpOnly`** cookie. `Secure` in production (https);
  allowed without `Secure` on local http.
- `SameSite=Lax`. Cookie `Domain` from config (`COOKIE_DOMAIN`), empty in local dev.
- The web app **never** stores the token in any JS-readable storage. Remove all
  `localStorage` / `sessionStorage` token handling. Fetches use `credentials: 'include'`.
- Logout clears the cookie server-side.
- **WebSocket `/live`** authenticates via the cookie. Prove the live feed still works.

**CSRF — required, because the browser now sends credentials automatically:**
- State-changing requests (POST / PATCH / DELETE) require a CSRF defence: an `Origin` check
  against the allowlist **plus** a double-submit CSRF token (or equivalent — justify).
- **The write-back `convert` / `void` routes move money — they must be covered.** Prove a
  cross-origin POST to them is rejected.

**Non-browser clients (scripts, gates):**
- Keep `Authorization: Bearer` working for non-browser clients so the gates run. Explain how
  a browser session and a script token are kept apart (e.g. the web never receives the token
  in a readable form). Report the token lifetime; if longer than 24h, propose a shorter one.

**Existing sessions:** users re-login once after this change — acceptable; state it.

**Proof:** after login, DevTools/`document.cookie` cannot read the session; `localStorage` /
`sessionStorage` hold no token; dashboard, pings, settings, call detail and the live feed work;
cross-origin POST to write-back → rejected.

## S2.4 — Security headers + CSP (F022)

On the web app:
- **CSP enforced** (not report-only at the end). `default-src 'self'`; `connect-src` =
  self + API origin + its WebSocket origin (from config); `frame-ancestors 'none'`.
  **No `unsafe-eval` in production.** If Next.js needs inline scripts, use **nonces**
  (middleware) rather than `unsafe-inline` for scripts — justify any exception.
- `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`,
  `Permissions-Policy` (deny unused), **HSTS in production only**.

**Proof:** response headers captured; every page listed in the gate renders with **no CSP
violations** in the browser console; an injected inline script is blocked.

## S2.5 — CORS allowlist (F023)

`enableCors({ origin: true })` reflects any origin. Replace with an explicit allowlist from
config (`WEB_ORIGINS`), `credentials: true`.
**Proof:** allowed origin → CORS headers present; any other origin → none.

## S2.6 — Validation errors and error bodies (F024)

- A global exception filter: `ZodError` → **400** with field messages; unexpected errors →
  **500** with a generic body and a request id — **no stack traces or internals** to the
  client. Details stay in server logs.
- **Date params** (`from`, `to`) validated as ISO-8601 → 400 on bad input (S0 V8 found
  them as bare strings).
**Proof:** invalid login body → 400; `from=garbage` → 400; forced unexpected error → 500 with
no internals.

## S2.7 — Refuse to boot in production without real secrets (F010)

Introduce `APP_ENV` (`development` default / `production`). In **production** mode, `api`,
`platform-sync`, `consumer` and `ingestion` **refuse to start** if any required secret is
missing or equals a known dev placeholder: JWT secret, `CREDENTIAL_ENCRYPTION_KEY`,
`ANI_ENCRYPTION_KEY`, `WRITEBACK_INTERNAL_TOKEN`, database passwords — the full list per
service. Development mode unchanged.
**Proof:** `APP_ENV=production` with dev placeholders → each service exits with a clear
message naming the variable; with real values → boots.

## S2.8 — Containers do not run as root (F027)

`services/platform-sync/Dockerfile` has no `USER`. Add a non-root user. Check every v1 image
(`api`, `web`, `consumer`, `ingestion`, `platform-sync`) and report each one's runtime user.
**Proof:** `docker exec <c> id` for each v1 container.

## S2.9 — Dependency and toolchain vulnerabilities (F025, F026, F054)

- **web:** upgrade Next.js and affected packages (PostCSS, socket.io-parser) to patched
  releases. If a **major** version is required, report what breaks before doing it — do not
  silently rewrite the app.
- **api:** patch multer / platform-express and socket.io-parser.
- **Go:** move every Go module and Docker base image to a patched Go release. **The host Go
  (1.22.12) must be upgraded by the operator** — report the exact version and the Windows
  `.msi` file name to install.
- Re-run `npm audit --omit=dev` and `govulncheck ./...` for every v1 module. **Target: no
  high/critical.** List any that remain, with why.
**Proof:** before/after audit tables; web production build passes; every web route in the gate
renders.

---

## Deploy checklist — add under **Infrastructure**

```
- [ ] Webhook ingestion (`ingestion`, :8080) is NOT exposed publicly in v1 — internal network
      only. v1 connects platforms by API key.
- [ ] Set `COOKIE_DOMAIN` to the parent domain, `WEB_ORIGINS` to the web origin(s),
      `APP_ENV=production`, and the trusted-proxy setting for client IPs.
- [ ] HTTPS everywhere — session cookie is `Secure` in production; HSTS enabled.
```

## Gates, commit, scan

- All three gates exit 0: `verify-system` (15/15, 11/11, LiveStatus, WebSocket),
  `verify-ingestion-slice`, `verify-presentation`.
- Commit locally. Secret scan of every unpushed commit. **Do not push.**

## Output — `docs/s2-security-report.md`
1. Scoreboard S2.1–S2.9: PASS / FAIL.
2. Per part: before/after with `file:line`, then live proof.
3. Chosen numbers (rate limits, token lifetime, CSRF method) with reasoning.
4. Audit tables before/after (npm, govulncheck).
5. The host Go version the operator must install.
6. Honest limitations; found but not fixed.

STOP. Do not push. Do not start S3.

## Out of scope
Intelligence services · webhook replay protection and body limits (F020, F021, F059 —
webhook not exposed in v1) · CI (F045) · alerts (F035) · S4 UI scope · Retreaver · Phase 2B.
