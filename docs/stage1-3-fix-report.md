# Calyx — Stage 1–3 fix report

**Branch:** `stage4/retreaver` (fixes sit on top of `b5f3dc4`)
**Input:** `docs/stage1-3-fix-spec.md` (owner response to `docs/stage1-3-verify-report.md` on `verify/stage1-3` @ `70b46ff`)
**Date:** 2026-10-03
**Part E:** CAN'T VERIFY (live). The $1 Convert and Cancel were not approved, and they were not sent.

Labels: **[PROVEN]** a test, command, or browser read from this pass. **[INFERRED]** follows from that evidence. **[UNKNOWN]** not observed.

---

## 1. Scoreboard

| Item | Status | Proof |
|---|---|---|
| A1 $25 cap removed | DONE | Server accepts 26, 1000, and 250. Server rejects 0, -1, and NaN. UI confirm allows an amount above 25. Confirm dialog shows platform, Convert or Cancel, the call id, and the amount. `9da02f243f4ef9bfebdf7fff7a3c7ef023cd2cfa` [PROVEN] |
| A2 One conversion state | DONE | Latest Retreaver rows: `converted` 1017, `converted_zero` 119, `not_converted` 2516 (sum 3652). Call `cf878cec-b6db-4e59-9138-414350e9c1b8` is `converted` on the list and on call detail (revenue 11). Call `6ddd3c3f-bb39-4666-89b5-e2d75bbefcdd` is `converted_zero` and is absent from both the Converted filter and the Not converted filter. Write-back `isConverted` matches `converted` (`TestIsConvertedMatchesApiConvertedState`). `4549b4118f8c5917365ea808b8133ff1ab72d9a6` [PROVEN] |
| A3 Audit every write-back rejection | DONE | `reject()` writes outcome `rejected` before the lock. Tests cover invalid amount, integration missing, tenant disabled, globally disabled, not live, already in progress (409), and lock error. `1f0b31bf7b96586f97dd630bccff04ead5d2dcc3` [PROVEN] |
| A4 Hide Convert/Cancel from non-admins | DONE | `canSeeWritebackActions` is admin-only. A local user `viewer-stage13@calyx.test` (role `viewer`) opened the same call detail and the page had no Convert or Cancel control. `/auth/me` returned `role=viewer`. Jest: non-admin 403 on convert, void, pause, and resume. `2160790fe27d29186c6e85212759393789f71f22` [PROVEN] |
| A5 Settle window 20s | DONE | `SettleWindow` default 20s. `WRITEBACK_SETTLE_SECONDS` parsed in platform-sync. Documented in `.env.example`, `docs/deploy-checklist.md`, and compose. `1fbdccead090f9fee2772f29082e4cb814d0bf4d` [PROVEN] |
| A6 Admin 403 and CSRF tests | DONE | Cookie session with an origin and no CSRF token returns 403 `csrf required` on convert, void, pause, and resume. Jest `writeback.routes.spec.ts` passed inside the system gate (27 tests). `042832c978990dc52a1685fa4a4fac7eddcb5a74` [PROVEN] |
| B1 Chart follows the period | DONE | Hardcoded 7-day timeline and “Last 7 days” badge are gone. Badge is the selected period. Browser reads: 7d chart sum 438 = KPI 438 (7 points); 30d sum 3064 = KPI 3064; all-time sum 3652 = KPI 3652 (49 points) and revenue $33,143.85. `beb04785372c760d2f66188191f8bf86823ace99` [PROVEN] |
| B2 v1 platforms only | DONE | Dashboard, Pings, and Coverage platform options are All, Ringba, Retreaver. Settings connect options are Ringba and Retreaver. Connection radios are API key, Test, and Live. The Webhook connection type is not on the form. TrackDrive and CallGrid code remains. `2ce5bcdeac3d1b2ef2796e48ea9215d67b57566f` [PROVEN] |
| B3 Login copy | DONE | Both blurbs read “Pay-per-call analytics for Ringba and Retreaver — every call, revenue and conversion in one place.” The login page text does not contain TCPA or Ask AI. `228890b41d0bd9afe8c83ffdfeeb57c2efd622b6` [PROVEN] |
| B4 No mock intelligence on call detail | DONE | Journey steps on `cf878cec-…` are Ping and Enrich. TCPA, Call Quality, Score, Compliance, and Ad attribution are absent. `bc9d6b237f161a205df4f45931d11fccc98b1732` [PROVEN] |
| B5 Duration labels | DONE | Call detail shows Talk time and Total duration. “Connected length” is absent. `754a78abee4b0005e75576c0b3fc29146a187ec7` [PROVEN] |
| B6 Profit dash when missing | DONE | `optionalMoney` maps an empty value to null, and the panel renders `—` for null. This call’s stored profit is 0, and the panel shows `$0.00`. `3984c5466cfb7d5476ea20cf6906b5c852137166` [PROVEN] for the real-zero row and the mapper. A row with a missing profit was not opened in the browser [UNKNOWN] on screen. |
| B7 Coverage gap on the header | DONE | 30-day header: “3,052 calls across 53 regions · 12 without a US/CA state”. 3052 + 12 = 3064 dashboard pings. `f90b5986be881b305550af23ae70fa909cf1e7fa` [PROVEN] |
| B8 Pause / resume | DONE | Migration `000029_integration_pause` applied (`schema_migrations` version 29). Pause sets `is_active=false` and leaves `sync_mode` unchanged. The row stays in the list as Paused. Delete is a separate confirm and sets `deleted_at`. Settings shows Retreaver “My test” as Paused, with Resume and Delete, and without a Live toggle. “My test” remains `sync_mode=mock`, `is_active=false`. `5922819841a10357ab1c0309b45e592528629fab` [PROVEN] |
| C1 Sweep log platform name | DONE | The window-fetch log line uses the platform name plus a `platform` field. `c7f2dd5b33572551ede9cb4e5754b9d23686414e` [PROVEN] |
| C2 Certification header | DONE | `docs/platform-certification.md` header is Stage 4 R2, 2026-10-02. Closing line: v1 is Ringba and Retreaver; TrackDrive and CallGrid are out of v1 scope. `9460b62f1368ff23ccb18274f94fc7cbd518e403` [PROVEN] |
| C3 govulncheck every Go module | DONE | Reported below. No upgrade. |
| C4 Next.js npm audit | DONE | Reported below. No upgrade. |
| D Gates | DONE | All three passed. See §3. |
| D Console | DONE | See §3. |
| E Live $1 Convert + Cancel | NOT DONE | CAN'T VERIFY (live). Not approved. |

Retreaver R2 numbers after the fixes and after the gates: `GET /analytics/metrics` all-time **3,652** calls and revenue **33143.85**. Presentation gate: “pings total > 0 on real (non-demo) rows (3652)”. [PROVEN]

---

## 2. Re-verified audit items

| Audit row | Was | Now | Why |
|---|---|---|---|
| 2.3 Chart follows the period | FAIL | **PASS** | The chart and the badge use the selected period. Sums match the KPI for 7 days, 30 days, and all time. [PROVEN] |
| 2.5 v1 platform filter | FAIL | **PASS** | Filters and the connect form list Ringba and Retreaver. Webhook is not a connection type on that form. Existing webhook rows still show Mode = Webhook. [PROVEN] |
| 2.6 Mock intelligence hidden | PARTIAL | **PASS** | Call detail no longer shows Score, Compliance, TCPA risk, call quality, or Ad attribution. [PROVEN] |
| 2.7 Login copy | FAIL | **PASS** | Login copy is the v1 sentence above. [PROVEN] |
| 3.1 One status derivation | FAIL | **PASS** | The API stamps `conversion_state`. List, filters, call detail, and write-back `isConverted` use that rule (flag and revenue > 0 is `converted`). The 119 flag-with-$0 rows are `converted_zero`. [PROVEN] |
| 3.4 Buttons only for Ringba | PASS | **PASS** | This database has no Ringba call rows, so the Retreaver call shows Open in Retreaver and no Convert button. `WRITEBACK_PLATFORMS` is still ringba. [PROVEN] |
| 3.4 Admin only | PARTIAL | **PASS** | API 403 for a non-admin, and the component renders nothing unless `/auth/me` role is admin. Viewer screenshot has no Convert or Cancel control. [PROVEN] |
| 3.4 Amount, no cap, confirm | FAIL | **PASS** | No UI cap. Server still requires a finite amount > 0. The confirm dialog shows platform, direction, call id, and the amount. [PROVEN] |
| 3.5 `WRITEBACK_ENABLED` | PASS | **PASS** | Routes are registered when `WRITEBACK_INTERNAL_TOKEN` is set. `SetGlobalEnabled(false)` makes the service refuse and write an audit row (`ErrGloballyDisabled`). An empty token leaves the routes unregistered, so that request never reaches the service. [PROVEN] by code and the globally-disabled test. |
| 3.5 Audit row on every attempt | FAIL | **PASS** | Early rejections and the 409 lock rejection write an audit row. [PROVEN] |
| 3.5 Settle window / K10 | PARTIAL | **PASS** | Default is 20s and `WRITEBACK_SETTLE_SECONDS` overrides it. A live read-back was not timed. [PROVEN] for the setting. The ~10.4s observation was not re-timed [UNKNOWN]. |

---

## 3. Gates and console

**Binary.** `which pwsh-preview` → `/usr/local/bin/pwsh-preview` → `/usr/local/microsoft/powershell/7-preview/pwsh`. `pwsh-preview --version` → **PowerShell 7.7.0-preview.5**. `which pwsh` → `/usr/local/bin/pwsh`, a symlink to `pwsh-preview` (same version). A user symlink `~/bin/pwsh` points at the same binary. The gates were started with **`/usr/local/bin/pwsh-preview`** (`pwsh-preview -ExecutionPolicy Bypass -File …`).

**Passing runs** (2026-10-02, after scoring, enrichment, providerconfig, compliance-intelligence, ask-ai, and ad-attribution were up):

| Gate | Result |
|---|---|
| `scripts/verify-system.ps1` | **SYSTEM VERIFY PASS** (exit 0). Docker 15/15, healthz 11/11, live status PASS, API 20/20, web routes 13/13, WebSocket PASS, smoke prefix PASS, browser CSP 21/21, unit tests PASS, web build PASS, pipeline 4/4. |
| `scripts/verify-ingestion-slice.ps1` | **INGESTION SLICE VERIFY PASS** (exit 0). Unique ping incremented analytics and coverage. Identical resend added zero rows. Changed revenue published a new row. `call_key` stored as ciphertext. Default view stayed 3652. |
| `scripts/verify-presentation.ps1` | **PRESENTATION VERIFY PASS** (exit 0). API checks 20/20, including `POST /ask`. |

An earlier `verify-system` on the same evening failed Docker 9/15 and healthz 5/11 because those six containers were stopped, and `POST /ask` returned 500. That run is not the result above. After the containers were started, presentation passed and the system gate was run again and passed.

**Console.** The system gate’s Playwright section: zero CSP violations and zero console/page errors on login (redirects to dashboard), dashboard, pings, settings, and call detail. In-app `window.__calyxErrors` was `[]` on Login, Dashboard, Pings, Coverage, Settings, and the call detail opened as admin. [PROVEN]

---

## 4. Server deploy note

Rebuild images **api**, **web**, and **platform-sync**.

Migrations, in order:

1. `000028` (already on this branch; apply on a server that is still behind it)
2. `000029_integration_pause` (new)

New env on platform-sync: **`WRITEBACK_SETTLE_SECONDS`**. Empty or invalid keeps the default **20**. Local compose sets `"20"`.

**`WRITEBACK_ENABLED` stays `false` on the server.** Committed compose uses `${WRITEBACK_ENABLED:-false}` on api and platform-sync. A git-ignored local `.env` may set `true` for this machine. Do not copy that value to the server.

---

## 5. Found, not fixed

- Dashboard Total Pings and the ping chart do not take the platform dropdown. `GET /analytics/metrics` and `GET /analytics/ping-series` accept `from`, `to`, and `show_demo`. Platform mix does accept `platform`.
- Call detail still renders Transcript and Audience Enrichment. B4 hid Score, Compliance, TCPA, call quality, and ad attribution. With the scoring container running, the transcript panel showed scripted sample lines on a Retreaver call.
- `npm audit` and the uncalled govulncheck IDs were reported and left unchanged (C3, C4).
- A row that was turned off only by `is_active=false` (no `deleted_at`) now shows as Paused. On this database that row is “My test”, which is the intended paused mock integration. It was not resumed.
- If `WRITEBACK_INTERNAL_TOKEN` is empty, convert and void routes are not mounted, so a rejection audit row is not written for that request. The audit row exists once the request reaches the service.

### C3 — govulncheck

`/Users/macbookpro/go/bin/govulncheck` **v1.8.0**, Go **1.27.1**, DB updated **2026-10-01**. Every module exit 0. “Your code is affected by 0 vulnerabilities.”

| Module | Called vulns | Extra, not called |
|---|---|---|
| ingestion, providerconfig, platform-sync, pingdedup | none | none reported |
| consumer, scoring, enrichment, compliance-intelligence, ad-attribution | 0 | 1 in an imported package, 1 in a required module |
| ask-ai | 0 | 0 imported, 2 in required modules |

Verbose on consumer and ask-ai names the same two IDs: **GO-2026-5841** (`klauspost/compress` v1.18.0, fixed in v1.18.7) and **GO-2026-5506** (`go.opentelemetry.io/otel` v1.37.0, fixed in v1.41.0). Neither is called. Not upgraded.

### C4 — Next.js `npm audit` (report only)

`npm audit --omit=dev` in `web`, exit 1. Metadata: info 0, low 0, moderate 0, high 0, **critical 1**, total 1. Package `next`, range `9.5.0 - 15.5.23`. Installed Next is **14.2.35** (`next build` printed that version). `next.config.mjs` has no `images`, `remotePatterns`, or `rewrites`. Web `src` has no `next/image` import and no `"use server"`. The app is served from Linux Docker. Middleware (`web/src/middleware.ts`) sets a CSP nonce. It is not an i18n middleware.

The single `next` finding rolls up these advisories. No upgrade was applied. Management decides.

| Advisory | Severity | Feature in this app |
|---|---|---|
| GHSA-p293-qw3h-jr36 | critical | Windows-hosted RCE. This web image is Linux. |
| GHSA-2xp9-vwfh-vxw4 | critical | Image Optimization AVIF RCE. No `images` config and no `next/image`. |
| GHSA-h25m-26qc-wcjf | high | RSC request deserialization. No server actions in `web/src`. |
| GHSA-q4gf-8mx6-v5v3 | high | Server Components DoS. |
| GHSA-8h8q-6873-q5fj | high | Server Components DoS. |
| GHSA-c4j6-fc7j-m34r | high | WebSocket-upgrade SSRF. Next rewrites are not configured. Live feed is the API socket. |
| GHSA-36qx-fr4f-26g5 | high | Pages Router i18n middleware bypass. No i18n config. Middleware is the CSP nonce. |
| GHSA-m99w-x7hq-7vfj | high | Server Actions DoS. No `"use server"`. |
| GHSA-89xv-2m56-2m9x | high | Server Actions SSRF on custom servers. No `"use server"`. |
| GHSA-p9j2-gv94-2wf4 | high | SSRF in rewrites. No rewrites. |
| GHSA-9g9p-9gw9-jx7f | moderate | Image Optimizer `remotePatterns` DoS. Not configured. |
| GHSA-ggv3-7p47-pfv8 | moderate | Request smuggling in rewrites. No rewrites. |
| GHSA-3x4c-7xq6-9pq8 | moderate | `next/image` disk cache. No `next/image`. |
| GHSA-ffhc-5mcf-pf4q | moderate | XSS in App Router apps that use CSP nonces. **This app does use CSP nonces.** |
| GHSA-gx5p-jg67-6x7h | moderate | XSS in `beforeInteractive` scripts with untrusted input. |
| GHSA-h64f-5h5j-jqjh | moderate | Image Optimization API DoS. Image optimization is not configured. |
| GHSA-wfc6-r584-vfw7 | moderate | RSC response cache poisoning. |
| GHSA-68g3-v927-f742 | moderate | Cache confusion for requests with bodies. |
| GHSA-4633-3j49-mh5q | moderate | Cache confusion for invalid UTF-8 bodies. |
| GHSA-4c39-4ccg-62r3 | moderate | Unbounded Server Action payload on the Edge runtime. |
| GHSA-955p-x3mx-jcvp | moderate | Disclosure of internal Server Function endpoints. |
| GHSA-3g8h-86w9-wvmq | low | Middleware redirect cache poisoning. |
| GHSA-vfv6-92ff-j949 | low | RSC cache-busting collisions. |

---

## 6. Merge recommendation

`stage4/retreaver` is the branch that should merge to `main`, once, after the owner accepts this report. It already contains Stages 1–3 plus Stage 4 R2, and these fixes are commits on that same branch. The audit (`verify/stage1-3` @ `70b46ff`) and the Stage 4 R0–R2 reviews stay the review record.

This pass does not merge. R3 does not start. Part E stays CAN'T VERIFY (live) until the owner names the sandbox call and approves the $1 Convert and the Cancel.

---

## Commits

Prefix `fix(stage1-3)`, oldest first:

- `9da02f243f4ef9bfebdf7fff7a3c7ef023cd2cfa` remove the $25 convert cap
- `4549b4118f8c5917365ea808b8133ff1ab72d9a6` stamp one conversion state from the API
- `1f0b31bf7b96586f97dd630bccff04ead5d2dcc3` audit every write-back rejection
- `2160790fe27d29186c6e85212759393789f71f22` hide Convert and Cancel from non-admins
- `1fbdccead090f9fee2772f29082e4cb814d0bf4d` settle window 20 seconds
- `042832c978990dc52a1685fa4a4fac7eddcb5a74` test admin-only and CSRF on write-back routes
- `beb04785372c760d2f66188191f8bf86823ace99` ping chart follows the selected period
- `2ce5bcdeac3d1b2ef2796e48ea9215d67b57566f` v1 filters and connect are Ringba and Retreaver
- `228890b41d0bd9afe8c83ffdfeeb57c2efd622b6` login copy
- `bc9d6b237f161a205df4f45931d11fccc98b1732` hide mock scores and compliance on call detail
- `754a78abee4b0005e75576c0b3fc29146a187ec7` duration labels
- `3984c5466cfb7d5476ea20cf6906b5c852137166` dash when gross profit was not reported
- `f90b5986be881b305550af23ae70fa909cf1e7fa` coverage gap header
- `5922819841a10357ab1c0309b45e592528629fab` pause and resume
- `c7f2dd5b33572551ede9cb4e5754b9d23686414e` sweep log platform name
- `9460b62f1368ff23ccb18274f94fc7cbd518e403` certification v1 scope

---

# Round 2

**Input:** `docs/stage1-3-fix2-spec.md` (owner review of the first fix round).
**Date:** 2026-10-03
**Branch:** `stage4/retreaver`
**Gates binary:** `/usr/local/bin/pwsh-preview` (PowerShell 7.7.0-preview.5)

Part E stays **CAN'T VERIFY (live)**. It is not a merge blocker. No live Convert or Cancel was sent. `WRITEBACK_ENABLED` stays `false` on the server. Local compose was put back to `true` after the R2-1 proof (`printenv` on `api` and `platform-sync` both print `true`).

## Scoreboard

| Item | Status | Proof |
|---|---|---|
| R2-1 Kill switch with the token set | DONE | File:line below. Live convert and void with the flag off returned 403 `writeback: write-back is disabled`, wrote `rejected` audit rows, and made no Ringba call. `69cd0b9f55e2acbdacacb8d28d12aaaff27cec51` [PROVEN] |
| R2-2 Pause, delete, and the server list | DONE for code. Server rows are a decision. | Rolled-back SQL: four PASS notices. Read-only query is in the deploy note. No `deleted_at` was written on an existing row. `852693959eeb832f2f555c57ee83c3e84dec1d09` [PROVEN] |
| R2-3 Platform filter on KPIs, chart, coverage | DONE | Window of four `smoke-r2plat-` rows: All = Ringba + Retreaver. ClickHouse matches. Rows deleted. Default view stayed 3,652 / $33,143.85. `6d8ecb65f4b77fdea3fd8d05adf537e5d0583cb8` [PROVEN] |
| R2-4 Hide transcript and enrichment | DONE | Call `cf878cec-b6db-4e59-9138-414350e9c1b8` shows Ping and Enrich. Transcript and Audience Enrichment are absent. `322d1fed6cfe4c8a7aa5193a299b1b3edb2b91c8` [PROVEN] |
| R2-5 Minute budget 75, test clock | DONE | Ten runs under load, admit max 80 against a limiter built at 80. Production default is 75. `6e5309b3deb3d386a4102264927b0e162bfc3781` [PROVEN] |
| R2-6 Gates in v1 and full mode | DONE | All three exit 0 in both modes. See below. `67710e1d9b8250736f9e375633a06cc3fa25d611` [PROVEN] |
| R2-7 Converted · $0 chip | DONE | 1017 + 119 + 2516 = 3652. The chip lists 119 calls. `0de8ea2e3bd4ed23b01c4b826d1660c8bb92747e` and `2b57a4d99b2180611797c3d4c731a98376961ab1` [PROVEN] |

## R2-1 — where the flag is read

The API mounts convert and void whenever the process is up. It does not read `WRITEBACK_ENABLED`.

- `services/api/src/integrations/integrations.controller.ts:105` `@Post(':id/writeback/convert')`
- `services/api/src/integrations/integrations.controller.ts:126` `@Post(':id/writeback/void')`
- `services/api/src/integrations/integrations.service.ts:267-271` `callWriteback` forwards when `WRITEBACK_INTERNAL_TOKEN` is set

platform-sync reads the flag and applies it before any Ringba call:

- `services/platform-sync/internal/config/config.go:68` `WRITEBACK_ENABLED` is true only for `true` or `1`
- `services/platform-sync/cmd/server/main.go:153` `wb.SetGlobalEnabled(cfg.WritebackEnabled)` when the token is non-empty
- `services/platform-sync/internal/writeback/convert.go:177` `run` calls `reject` with `ErrGloballyDisabled` first
- `convert.go:154-162` a failed audit insert is returned with the refusal, so a swallowed insert cannot look like success

Migration `000030_writeback_audit_rejected` allows outcome `rejected`. Schema is version **31**, dirty false.

Live proof, flag false and token set, admin cookie session, CSRF, fake call id `r2-killswitch-do-not-fire`, real integration id: convert and void both HTTP 403 with `writeback: write-back is disabled`. `writeback_audit` has one `convert`/`rejected` row and one `void`/`rejected` row with that message. platform-sync logs for that window contain no `api.ringba.com` call. `TestAuditRowForEveryEarlyRejection` covers `SetGlobalEnabled(false)` for convert and void and asserts zero Ringba fires. [PROVEN]

## R2-2 — sweeps, resume, and the server list

`scripts/verify-pause-scheduling.sql` runs in a transaction and rolls back. This pass printed:

- PASS paused (`is_active=false`) is not in `list_api_sync_integrations()`
- PASS deleted (`deleted_at` set) is not scheduled even if `is_active` is true
- PASS resume matches 0 rows when `deleted_at` is set
- PASS pause and resume audit rows record who, when, integration, action

Then `ROLLBACK`. [PROVEN]

The list query is `deleted_at IS NULL`, so a deleted row is not on the Settings screen and Resume is not offered. The resume `UPDATE` also requires `deleted_at IS NULL`, so a direct call changes 0 rows and the API returns 404. [PROVEN] by the SQL. The UI path is the same filter. [INFERRED]

**Server decision, still open.** Do not guess which old `is_active=false` rows are deletes. Run this on the server **before** `000029`. It returns tenant, platform, name, and `updated_at` only:

```sql
SELECT t.name AS tenant, i.platform, i.name, i.updated_at
FROM integrations i
JOIN tenants t ON t.id = i.tenant_id
WHERE NOT i.is_active
ORDER BY i.updated_at;
```

On this machine the same query returns one row: Demo Tenant, retreaver, “My test”, `2026-10-02 16:33:05.125366+00`. That row is the paused mock. It stays `sync_mode=mock` and `is_active=false`. No existing row was given a `deleted_at`. The owner decides which server rows get `deleted_at`.

## R2-3 — platform filter

`GET /analytics/metrics` and `GET /analytics/ping-series` take `platform` and apply `AND platform = {platform:String}` inside the deduped call query. The dashboard passes `filters.platform`. Coverage already passed `platform` through to the heatmap; that path was left as it was and checked below.

Proof used four `smoke-r2plat-` rows on 2020-01-15 (two Ringba, revenue 10 and 5.50; two Retreaver, revenue 3 and 0), queried with `show_demo=true` and that day as the period, then deleted. `mutations_sync` left zero of those ids.

| Surface | All | Ringba | Retreaver |
|---|---|---|---|
| `/analytics/metrics` calls / revenue | 4 / 18.50 | 2 / 15.50 | 2 / 3.00 |
| ping-series sum | 4 | 2 | 2 |
| `/coverage/heatmap` total | 4 (TX 2, CA 2) | 2 (TX) | 2 (CA) |

All equals Ringba plus Retreaver. ClickHouse `argMax` over those ids: ringba 2 / 15.5, retreaver 2 / 3. [PROVEN]

Default view with the smoke rows still present: 3,652 calls and revenue 33,143.85. Ringba with the smoke prefix excluded: 0. After delete, the same default numbers. [PROVEN]

## R2-4 — transcript and enrichment

`TranscriptPanel` and `EnrichmentPanel` render only when `SHOW_MOCK_PANELS` is on (`NEXT_PUBLIC_V1_ONLY !== 'false'`). The rebuilt web image leaves that unset, so the flag is off. Call `cf878cec-b6db-4e59-9138-414350e9c1b8`: page text has Ping and Enrich, and has neither “Transcript” nor “Audience Enrichment”. The journey has no Score step. Screenshot `/tmp/calyx-r2-call-detail.png`. [PROVEN]

## R2-5 — 81 versus 80

The limiter records the admit time under the lock and returns that time. It appends a hit only when the count inside the window is still under the cap, so it cannot store an 81st admit. The failing run stamped `time.Now()` **after** `Wait` returned. Under load that later stamp pulled one already-expired limiter hit into the test’s inclusive 60s window, and the test reported 81. That is a test clock, not an extra Ringba call. [PROVEN] by the admit-versus-observed log in `ratelimit_test.go` and by the limiter refusing to append at the cap (`ratelimit.go`).

The test still builds the limiter at 80 so the cap itself is what gets measured. Ten runs with a CPU busy-loop, each `admit_max=80` and `observed_after_wait_max=80`. The max rolling admit count across the ten runs is **80**.

Real Ringba traffic uses `RingbaRatePerMin()` (`ratelimit.go:125`), env `RINGBA_RATE_PER_MIN`, default **75**. `ForAccount` builds the limiter with that value.

## R2-6 and the merge-gate runs

`-V1` skips the six intelligence containers, their health ports, and `POST /ask`. `INTELLIGENCE_SCALE=0` selects the same mode. Full mode is unchanged.

v1, intelligence containers stopped:

| Gate | Result |
|---|---|
| `verify-system.ps1 -V1` | **SYSTEM VERIFY PASS** exit 0. Docker 9/9, healthz 5/5, live status PASS, API PASS (v1), web routes 13/13, WebSocket PASS, smoke prefix PASS, browser CSP 21/21, unit tests PASS, web build PASS, pipeline 4/4. |
| `verify-ingestion-slice.ps1` | **INGESTION SLICE VERIFY PASS** exit 0. Default view stayed 3652. |
| `verify-presentation.ps1 -V1` | **PRESENTATION VERIFY PASS** exit 0. API checks 19/19. Log line: `SKIP POST /ask (v1 mode)`. |

`GET /scoring/scores`, `/compliance/exposure`, `/compliance/risk-scores`, and `/attribution/status` returned 200 with those containers stopped, so v1 mode left them in the gate. [PROVEN]

Full mode, those containers running:

| Gate | Result |
|---|---|
| `verify-system.ps1` | **SYSTEM VERIFY PASS** exit 0. Docker 15/15, healthz 11/11, API PASS (20/20), browser CSP 21/21, pipeline 4/4, unit tests PASS, web build PASS. |
| `verify-ingestion-slice.ps1` | **INGESTION SLICE VERIFY PASS** exit 0. Default view stayed 3652. |
| `verify-presentation.ps1` | **PRESENTATION VERIFY PASS** exit 0. API checks 20/20, including `POST /ask`. |

After both passes, `GET /analytics/metrics` and `?platform=retreaver` are **3,652** calls and revenue **33143.85**. [PROVEN]

## R2-7 — where the $0 conversions are

A separate chip, **Converted · $0** (`converted_zero`). Folding those calls into Converted would count a flag with amount 0 as converted, which is the opposite of the A2 rule (flag and amount > 0).

Latest Retreaver rows, `distinct_total`:

| Filter | Calls | Sample state |
|---|---|---|
| (none) | 3652 | |
| converted | 1017 | `converted` |
| converted_zero | 119 | `converted_zero` |
| unconverted | 2516 | `not_converted` |

1017 + 119 + 2516 = 3652. [PROVEN]

The call list was 500ing on these filters: ClickHouse treats `argMax(has_converted, received_at) AS has_converted` in the SELECT as nested inside the same `argMax` in the HAVING (code 184). `SETTINGS prefer_column_name_to_alias = 1` on the collapsed list and export queries stops that. After the API rebuild, the Ping Feed with All time and the chip selected shows **119 calls**, revenue $0.00, status “Converted · $0”. Screenshot `/tmp/calyx-r2-converted-zero.png`. [PROVEN]

## Server deploy note

1. Run the R2-2 query **before** `000029`. The owner marks which rows are old deletes. This pass does not set `deleted_at` for them.
2. Apply Postgres in order: `000028` → `000029` → `000030` → `000031`.
3. Rebuild `api`, `web`, and `platform-sync`.
4. Set `WRITEBACK_SETTLE_SECONDS` (default 20 when unset) and `RINGBA_RATE_PER_MIN` (default **75**).
5. `WRITEBACK_ENABLED` stays `false` on the server. Committed compose defaults it with `${WRITEBACK_ENABLED:-false}` on api and platform-sync. This machine’s git-ignored `.env` sets `true`. `INTELLIGENCE_SCALE` is not a compose setting. `NEXT_PUBLIC_V1_ONLY` is unset in compose, and the web treats that as v1.

The same notes are in `docs/deploy-checklist.md`.

## Secret scan

`git log -p b5f3dc4..HEAD` was scanned for live secret-key prefixes, cloud access-key ids, and private-key blocks. The only hits are the existing local `WRITEBACK_INTERNAL_TOKEN` placeholder in `.env.example` and `deploy/docker-compose.yml`, which the checklist already says to replace before production. No new credential was added. `docs/evidence/s2-csp-fix/` was not committed.

## Merge recommendation

Round 2 is complete on `stage4/retreaver`. The owner says merge. This session pushes the branch only. It does not merge to `main`. R3 does not start. Part E remains the gate for turning write-back on, later, on the Ringba sandbox call, from one machine, after the owner approves it.
