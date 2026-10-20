# v1 hotfix: out-of-scope routes return 404

Branch `v1/hide-intel-routes`, based on `origin/main` 99618e0.

Until this change, `V1_ONLY` only took the out-of-scope pages out of the menu. Anyone who typed `/scoring` or `/ask` still got the page, and the API still served the mock attribution, compliance, enrichment, provider, scoring and ask-ai endpoints. Now, with V1_ONLY on (the default when it is unset), the web returns a real HTTP 404 for those pages and the API returns 404 for those endpoints.

## What is closed

### Web (`web/`)

| Route | Feature key | v2 flag that re-opens it |
|---|---|---|
| `/scoring`, `/scoring/*` (incl. `/scoring/scorecards`) | `qa` | `NEXT_PUBLIC_FEATURE_QA=true` |
| `/ask`, `/ask/*` | `ask_ai` | `NEXT_PUBLIC_FEATURE_ASK_AI=true` |
| `/intelligence`, `/intelligence/*` | `intelligence` (hub) | none yet (only `NEXT_PUBLIC_V1_ONLY=false`) |
| `/compliance`, `/compliance/*` | `compliance` | none, ever |
| `/attribution`, `/attribution/*` | `attribution` | none, ever |

Matching works per path segment: `/asking` or `/scoringx` would not be caught.

How it works:
- `web/src/lib/v1-features.ts`: `NON_V1_ROUTE_FEATURES` maps each route prefix to its feature. `featureForRoute()`, `isFeatureEnabled()` and `isRouteVisible()` read the map. The navigation already called `isRouteVisible()`, so the menu and the 404 always agree.
- `web/src/middleware.ts` (the existing CSP middleware): when `isRouteVisible(pathname)` is false, it rewrites the request to a path that does not exist (`/__calyx_not_found`). Next.js then renders its not-found page with **HTTP 404**. The CSP nonce and the security headers are applied exactly as before. RSC / client-navigation requests also get 404.
- `web/src/app/(app)/{scoring,ask,intelligence,compliance,attribution}/layout.tsx`: these server layouts call `notFound()` as a backstop. A layout-level `notFound()` alone was **not enough**: the app shell streams first, so the 404 UI rendered but the status was still **200**. I checked this with curl before adding the middleware rewrite.
- The flags are `NEXT_PUBLIC_*`, so their values are baked in at build time (the same as `NEXT_PUBLIC_V1_ONLY`). Each flag is referenced literally so Next can inline it.

### API (`services/api`)

There is one global guard, `FeatureGateGuard`, plus a `@FeatureGate('<feature>')` decorator for a controller or a single handler. The code is in `services/api/src/common/feature-gate.ts`. The guard throws `NotFoundException('Cannot <METHOD> <path>')`, which has the same body shape as Nest's 404 for an unknown route.

| Controller / route | Feature key | Re-opened by |
|---|---|---|
| `AttributionController`: `GET /attribution/events`, `GET /attribution/status` | `attribution` | only `V1_ONLY=false` |
| `ComplianceController`: `GET /compliance/risk-scores`, `/violations`, `/exposure` | `compliance` | only `V1_ONLY=false` |
| `EnrichmentController`: `GET /enrichment/:aniHash` | `enrichment` | only `V1_ONLY=false` |
| `ProvidersController`: `GET /providers/catalog`, `GET/POST /providers/bindings`, `PATCH/DELETE /providers/bindings/:id`, `POST /providers/bindings/:id/test`, `POST /providers/resolve` | `providers` | only `V1_ONLY=false` for now (v2 will add a flag in `FEATURE_FLAG_ENV`) |
| `AskAiController`: `POST /ask`, `GET /ask/history` | `ask_ai` | `FEATURE_ASK_AI=true` |
| `ScoringController`: `GET /scoring/scores` | `qa` | `FEATURE_QA=true` |
| `ScorecardsController`: `GET/POST /scorecards`, `GET/PATCH/DELETE /scorecards/:id` | `qa` | `FEATURE_QA=true` |
| `ScoringController`: `GET /scoring/transcript` | `transcription` | `FEATURE_TRANSCRIPTION=true` |
| `ScoringController`: `POST /scoring/deepgram/callback` (mock Deepgram callback) | `transcription` | `FEATURE_TRANSCRIPTION=true` |

Not touched, so v1 keeps working: auth, analytics/metrics, pings, coverage, reports, integrations (including the write-back convert/void and pause/resume routes), settings integrations, native-integration, the live websocket, healthz/readyz, and **alerts**.

Rules (`isFeatureOpen` in `feature-gate.ts`):
- `V1_ONLY=false` opens every feature (internal demo builds).
- Otherwise a feature is open only when its flag is exactly `'true'`. Values like `'1'` or `'TRUE'` do not count.
- `attribution`, `compliance`, `enrichment` and (for now) `providers` map to `null`: no flag can open them.
- `workflows` (`FEATURE_WORKFLOWS`) has a key and a flag, but no routes carry it yet.
- `FEATURE_ASK_AI_TRANSCRIPTS` is a sub-feature of Ask AI and owns no routes. It deliberately opens nothing: test `attribution, compliance and enrichment have no flag and no other flag opens them`.
- The guard reads the env on every request, so the default is decided at runtime. When `V1_ONLY` is unset, the routes are **closed**.

### Guard order (unauthenticated requests get 404, never 401/403)

Nest runs guards in this order: global guards, then controller `@UseGuards`, then handler `@UseGuards`. Global `APP_GUARD` providers are applied in the order Nest scans their modules. The root module is inserted and scanned first (checked in `@nestjs/core/scanner.js`: `scanForModules` / `applyApplicationProviders`).

`FeatureGateGuard` is registered as `APP_GUARD` in the **root `AppModule`'s own `providers`**. The existing global `CsrfGuard` is registered in `AuthModule`, which is imported. So the order for a gated route is:

1. `FeatureGateGuard`: throws 404 if the feature is closed.
2. `CsrfGuard` (global, AuthModule)
3. `AuthGuard('jwt')` (controller or handler)

A closed route therefore answers 404 to anonymous requests, to requests CSRF would reject, and to authenticated requests alike. The response does not reveal that the route exists. Do not move the provider into a feature module or call `app.useGlobalGuards()` in `main.ts`: that would append the gate *after* `CsrfGuard`. The tests cover this.

The websocket (`LiveModule`) is skipped: the guard only acts on `http` contexts.

## Tests

API (`services/api/src/common/feature-gate.spec.ts`, 122 tests). It boots a real Nest HTTP app on an ephemeral port with the real controllers, the real `JwtStrategy`, the real `CsrfGuard` as a global guard in an imported module, `FeatureGateGuard` in the root module, and stubbed services. Requests go through `fetch`.
- Every closed route returns 404 with `V1_ONLY` unset and with `V1_ONLY=true`, both authenticated and anonymous.
- `/analytics/metrics` (v1) still returns 200 (401 anonymous).
- The 404 body matches Nest's unknown-route shape.
- Guard order: a cookie-session POST with no Origin gets 404 when closed and 403 from CsrfGuard when open. An anonymous GET gets 404 when closed and 401 from AuthGuard when open.
- With `V1_ONLY=false`, every route is not 404 and below 500.
- `FEATURE_ASK_AI`, `FEATURE_TRANSCRIPTION` and `FEATURE_QA` each open only their own routes.
- Unit tests for `isFeatureOpen`, and a wiring check that `AppModule` registers the guard in its own providers.

Web (`web/src/lib/v1-features.test.ts`, 6 new tests): the route-to-feature mapping, per-segment prefix matching, all closed in v1, all open with v1 off, `ask_ai`/`qa` flags opening only their own routes, and exact `"true"` only.

Results:
- `services/api`: `npx jest`, **12 suites, 186 tests, all pass**. `npm run build` passes. (`npm run lint` cannot run: eslint is not installed in the package.)
- `web`: `node --experimental-strip-types --no-warnings --test src/lib/*.test.ts`, **30/30 pass**. `npx next build` passes. It was built three times (default, `NEXT_PUBLIC_FEATURE_ASK_AI=true`, `NEXT_PUBLIC_V1_ONLY=false`), and the final build is the default one.

## Proof

All files are in `docs/evidence/v1-hide-intel-routes/`.

- `web-curl-default.txt`: `next start` on the default build. `/scoring`, `/scoring/scorecards`, `/intelligence`, `/ask`, `/compliance` and `/attribution` return **404**. `/dashboard`, `/login`, `/pings`, `/reports`, `/coverage`, `/settings` and `/calls/abc` return **200**.
- `web-curl-feature-ask-ai.txt`: only `/ask` returns 200; the other gated routes stay 404.
- `web-curl-v1-only-false.txt`: everything returns 200.
- `web-scoring-404.jpg`, `web-scoring-scorecards-404.jpg`, `web-ask-404.jpg`: Next's 404 page.
- `web-dashboard-redirects-to-login-200.jpg`: `/dashboard` returns 200 and, with no backend or session, redirects on the client to `/login`, which renders.
- `api-curl.txt`: curl against the compiled `dist/` controllers, with the same wiring as the spec (real controllers, guards and JWT; stubbed services; no Postgres/Redis/Kafka). It has three runs: `V1_ONLY` unset (all gated routes 404, `/analytics/metrics` 200), `V1_ONLY=false` (all 200/201, anonymous 401), and `FEATURE_ASK_AI=true` (only `/ask` and `/ask/history` open).

## Deploy

- Rebuild **api** and **web** images.
- Env: none needed. `V1_ONLY` (api) and `NEXT_PUBLIC_V1_ONLY` (web build arg) are not set in `deploy/docker-compose.yml` or `web/Dockerfile`, so both default to **on**. Do **not** set `V1_ONLY=false` in production.
- v2 flags default to off. Before they can be used, the web Dockerfile and compose need to pass `NEXT_PUBLIC_FEATURE_*` build args, and the api needs `FEATURE_*` env. Neither is wired yet.
- nginx: no change needed. The exact-match API locations (`/scoring/scores`, `/compliance/*`, `/attribution/*`, `/ask/history`, POST `/ask`, and so on) still proxy to the api, which now returns 404.
- The web 404 response carries Next's `x-middleware-rewrite: /__calyx_not_found` header. It does not reveal anything about the hidden page.

## Open questions

1. **Alerts API** (`/alerts/rules` GET/POST/DELETE): the Alerts tab is hidden in v1 and the alert engine is mock, but the owner did not list it, so it is left open. If it should close, add `@FeatureGate('workflows')` (or a new `alerts` key) to `AlertsController`; this is a one-line change.
2. **`/intelligence` hub**: it has no v2 flag (key `intelligence`, web only). Should it re-open when any of `ask_ai`/`qa`/`transcription` is on, or get its own flag?
3. **Providers in v2**: when the Providers status tab comes back, should it open the whole `ProvidersController`, or only `GET /providers/catalog` and `GET /providers/bindings` (handler-level `@FeatureGate`)? Today the whole controller is one key.
4. Web `/scoring` is keyed `qa`. If v2 ships transcription without QA, the transcript UI will need its own page or key.

## Owner decisions (2026-10-20)

| # | Decision | Done |
|---|---|---|
| a | Close `AlertsController` under `@FeatureGate('workflows')`; Phase 2D re-opens it | ✅ `services/api/src/alerts/alerts.controller.ts`; `GET /alerts/rules` and `DELETE /alerts/rules/:id` added to `feature-gate.spec.ts` (404 closed, open with `FEATURE_WORKFLOWS=true` or `V1_ONLY=false`). API jest 196/196 |
| b | The `/intelligence` hub stays closed in v2; the sidebar links each feature directly | No flag re-opens `/intelligence` |
| c | `ProvidersController` stays closed for good. The v2 Providers status tab gets a **new read-only endpoint** (configured yes/no, usage, $ estimate), because keys live in the server env, not in bindings | `providers` keeps no flag |
| d | The transcript UI goes on the **call detail page** behind `FEATURE_TRANSCRIPTION`, not on `/scoring` | Noted for 2B; web `/scoring` stays keyed to `qa` |
