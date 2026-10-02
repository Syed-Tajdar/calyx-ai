# Stages 1–3 verification

**Spec:** `docs/stage1-3-verify-spec.md` (copied from `~/Downloads/stage1-3-verify-spec-1.md`)
**Branch under review:** `convert-cancel-screen` @ `c4e3c5da9a173e72d9e7d5c23cf69535d4b0c6ea`
**Compared with:** `main` @ `ba9c825b32af123c3d3add4e9b8c0fa92cfcf3d1`
**Audit branch:** `verify/stage1-3` (this commit)
**Date:** 2026-10-02
**Mode:** audit only. No product code was changed.

Claims are `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. One verdict per requirement.

---

## Part 0 — Environment and scope

| | |
|---|---|
| Machine | Same Mac as Stage 4. macOS 15.7.4, x86_64. Repo `/Users/macbookpro/Desktop/Calyx/calyx-ai` |
| HEAD | `c4e3c5d` on `verify/stage1-3`, created from `convert-cancel-screen` |
| Parent | `c4e3c5d^` is `ba9c825`. `ba9c825` is an ancestor. `[PROVEN]` |
| Commits in range | **One.** `c4e3c5d feat: add conversion state handling and UI updates` (2026-09-30 03:34 +0500). Stages 1–3 are not separate commits. |
| `git status --short` | 8 lines, all untracked (this spec, this report's evidence, and unrelated `docs/evidence/s2-csp-fix/*`). No line-ending-only modifications. |
| Diff size | 28 files, +2031 / −93 |

### Retreaver before the switch

`My test` (`53200af1-…`) was already `is_active=false`, `sync_mode=mock`. Re-checked after the `c4e3c5d` platform-sync container started. Still mock and inactive. No Live Retreaver sync on the old adapter. `[PROVEN]`

### Containers

`docker compose build api platform-sync web` from `c4e3c5d`, then `up`. **Migrate failed** and the app containers did not start on that command:

```
error: no migration found for version 28: read down for version 28 .: file does not exist
```

This machine's Postgres is at schema version **28** (Stage 4). This branch's migrations stop at **27**, so migrate tries to step down and has no down file. Volumes were not removed. The three rebuilt images were then started with `docker compose up -d --no-deps`.

| Image | Created | What was checked inside |
|---|---|---|
| `deploy-platform-sync` | 2026-10-02T19:13:24Z | Rebuilt after checkout. `[PROVEN]` |
| `deploy-api` | 2026-09-29T22:16:08Z | Cache hit. `dist` contains `should_convert`. `[PROVEN]` the feature is in the running image. The image timestamp is ~18 minutes before the git commit. |
| `deploy-web` | 2026-09-29T22:14:30Z | Cache hit. Built JS contains `Confirm convert`. `[PROVEN]` |

API `http://127.0.0.1:3100/healthz` 200. Web `http://127.0.0.1:3101/login` 200.

### Toolchain

| Tool | Result |
|---|---|
| Docker | 28.5.1 |
| Go | `go1.27.1 darwin/amd64` (host). Modules say `go 1.25.0`. Builders say `golang:1.25.14-alpine`. |
| Node | v22.17.0 |
| `pwsh` | **Missing.** 🙋 The three PowerShell gates were not run. |

### File grouping (`ba9c825..c4e3c5d`)

**Stage 1 (S2b)**

- `scripts/check-smoke-call-id-prefix.mjs`
- `scripts/proof-content-hash-sync-c.ps1`
- `scripts/prove-content-hash-ch.ps1`
- `services/platform-sync/internal/adapter/mock.go` (`proof-` id changed to `smoke-proof-`)
- `docs/s2b-report.md`

**Stage 2 (UI)**

- `web/src/app/(app)/dashboard/page.tsx`
- `web/src/app/(app)/pings/page.tsx`
- `web/src/app/(app)/coverage/page.tsx`
- `web/src/lib/v1-features.ts`
- `web/src/lib/app-navigation.ts`
- `web/src/lib/date-filter.tsx`
- `web/src/lib/ping-timeline.ts`
- `services/api/src/coverage/coverage.controller.ts`
- `services/api/src/coverage/coverage.service.ts`

**Stage 3 (convert / cancel)**

- `web/src/components/native/ConversionActions.tsx`
- `web/src/components/ui/ConfirmDialog.tsx`
- `web/src/components/ui/FilterBar.tsx`
- `web/src/components/dashboard/PingsTable.tsx`
- `web/src/app/(app)/calls/[callId]/page.tsx`
- `web/src/lib/api.ts`
- `services/api/src/pings/pings.controller.ts`
- `services/api/src/pings/pings.filters.ts`
- `services/api/src/pings/pings.service.ts`
- `services/api/src/integrations/integrations.service.ts`
- `docs/convert-cancel-screens-plan.md`
- `docs/convert-cancel-screens-report.md`
- `.calyx-ai/plans/convert_cancel_screens.plan.md`

**No stage (unreviewed scope on a branch that is supposed to be Stages 1–3)**

- `docs/retreaver-stage4-spec.md` — Stage 4 spec committed here. Finding.

### Specs and reports for Stages 1–3

| Doc | Present |
|---|---|
| `docs/s2b-spec.md` | Yes (already on `main`) |
| `docs/s2b-closeout-spec.md` | **Missing** |
| `docs/s2b-report.md` | Yes, added in `c4e3c5d` |
| `docs/writeback-phase2-spec.md` | Yes (already on `main`) |
| `docs/writeback-phase2b-report.md` | **Missing** (the phase 2 spec names it as the phase output) |
| Phase 3 / Phase 4 spec files | **Missing.** The work that landed is `docs/convert-cancel-screens-plan.md` and `docs/convert-cancel-screens-report.md` |

---

## Part 1 — Stage 1 scoreboard

| # | Verdict | Proof |
|---|---|---|
| 1.1 Smoke prefix | **PASS** | `node scripts/check-smoke-call-id-prefix.mjs` → `SMOKE PREFIX CHECK PASS (321 scripts scanned)`. A temp `scripts/__audit_bad_prefix_probe.mjs` with `` callId = `csp-live-…` `` exited 1, then the file was deleted and the check passed again. Gate call is `scripts/verify-system.ps1:136-140`. The PowerShell process itself was not executed (`pwsh` missing). |
| 1.2 No `csp-live-` rows | **PASS** | ClickHouse `pings` for tenant `11111111-…`: `csp-live` 0, `proof-` 0, `apisync` 136 rows / 134 ids, `demo` 53, `smoke` 39, other (real ids) 3691 rows / 3652 ids. `[PROVEN]` |
| 1.3 Fonts | **PASS** | Local Geist + Inter in `web/src/app/layout.tsx`. Gate assertion `scripts/check-dashboard-browser.mjs:198-217` fails a serif body and requires `/inter/i` and `/geist/i`. Computed on Dashboard and call detail: body `__interBody_…`, h1 `__geistHeading_…`. Not a generic serif. Pings and Settings share that layout. |
| 1.4 Top bar email | **PASS** | Dashboard button `AD admin@calyx.ai`. Screenshot `evidence/stage1-3/dashboard-30d-kpis.png`. The label is the signed-in account control; this audit did not capture the `/auth/me` network row. `[PROVEN]` the email is on screen. `[INFERRED]` it came from `/auth/me`. |
| 1.5 Cache headers | **PASS** | `GET /login` → `cache-control: no-store, no-cache, must-revalidate, private` (`web/src/middleware.ts:59`). `/_next/static/chunks/webpack-….js` and one `woff2` → `public, max-age=31536000, immutable`. |
| 1.6 WebSocket origin | **PASS** | `node scripts/check-ws-origin.mjs`: bad origin `http://evil.example:3999` did not stay connected; `http://localhost:3101` connected. **No `Origin` header:** a script client with `auth.token` connected. That matches `live.gateway.ts:48-49` (missing Origin is allowed when a token is present). |
| 1.7 Production secret refusal | **PARTIAL** | API `assertProductionSecrets` (`services/api/src/config/env.ts:72-95`) and platform-sync `refuseProductionPlaceholders` (`internal/config/config.go:92-112`) throw when `APP_ENV=production` and the known dev secrets are still set. Ingestion and consumer have the same helper. **Not booted** in production with real secrets, so the "works end to end" half is not shown. Web has no equivalent secret gate (`APP_ENV` there only switches CSP). |
| 1.8 Next.js audit | **PARTIAL** | `npm audit --omit=dev` on `web` this run: `next` `^14.2.35`, severity **critical**, range `9.5.0 - 15.5.23`. `docs/s2b-report.md` already has a fixed-in table for several GHSA ids and says `images` / `rewrites` are unused (`next.config.mjs` has neither). This audit did not rebuild that table for every advisory the current audit printed (RSC, middleware, server actions, and others). |
| 1.9 Go 1.25 / govulncheck | **PARTIAL** | Every `go.mod` under `services/` and `packages/pingdedup` is `go 1.25.0`. Every Go Dockerfile builder is `golang:1.25.14-alpine`. Host Go is 1.27.1, one binary on `PATH`. `govulncheck` v1.8.0 (DB 2026-10-01) on `services/platform-sync`: **No vulnerabilities found**, exit 0. The other Go modules were not scanned in this pass. |

---

## Part 2 — Stage 2 scoreboard

Stage 4 Retreaver rows are in this database (3,652 real calls, all `platform=retreaver`). They are included in the totals below and called out.

| # | Verdict | Proof |
|---|---|---|
| 2.1 KPI vs ClickHouse vs Coverage | **PARTIAL** | Default period is 30 days (`date-filter.tsx:37`). Dashboard showed **$28,906.34** and **3,064** pings. ClickHouse latest-row query for event time `>= 2026-09-02 19:00:00` (local midnight minus 30 days, PKT): **3064** calls, revenue **28906.34**. All time dashboard **$33,143.85** / **3,652**, same as all-time ClickHouse nondup revenue. Coverage headline for that 30-day view: **3,052** pings (`3,052 pings across 53 regions`). ClickHouse with `state != ''` in the same window is 3052. The 12-call gap is blank `state`. Dashboard matches ClickHouse. Coverage does not match the dashboard ping total. |
| 2.2 No duplicate ping rows | **PASS** | Collapsed groups with more than one row: **0** (3,875 groups). The visible Ping Feed page had 25 ids, 25 unique. List query groups by `platform, external_call_id` before `LIMIT` (`pings.service.ts:169-178`). Screenshot `evidence/stage1-3/pings.png`. |
| 2.3 Chart follows the period | **FAIL** | `buildFilledPingTimeline(chartPings, 7)` is hardcoded (`dashboard/page.tsx:121-123`). The badge is the literal `Last 7 days` (`PingChart.tsx:31`). With 30 days selected the chart axis was 09-27…10-03 and the badge still said Last 7 days, while KPIs were the 30-day totals. 7 days: KPIs **$4,738.32** / **438**, badge still Last 7 days. All time: **$33,143.85** / **3,652**, badge still Last 7 days. |
| 2.4 Publishers card | **PASS** | `SHOW_MOCK_PANELS` is false when `NEXT_PUBLIC_V1_ONLY` is not `false` (`v1-features.ts:16,42`). Compose does not set that variable. Dashboard body text had no `MediaCo`. The Publishers block is behind that flag (`dashboard/page.tsx:242-258`). |
| 2.5 v1 platform filter | **FAIL** | Dashboard and Pings combobox options: All, Ringba, Retreaver, **TrackDrive, CallGrid, Call Loom**. Settings "Connect a platform" lists Ringba, Retreaver, TrackDrive, CallGrid, and the connection type **Webhook — postback URL in platform** is shown (`platform-connection.ts:6-20`, `AddIntegrationForm.tsx`). Screenshot `evidence/stage1-3/settings.png`. |
| 2.6 Mock intelligence hidden | **PARTIAL** | Sidebar at 1280px: Dashboard, Pings, Reports, Coverage, Settings. No Scoring, Ask, Compliance, or Attribution links. Dashboard had no TCPA / Ask / Publishers cards. **Call detail still shows them:** journey steps Score and Compliance, TCPA risk **15**, Ad attribution **Mock · Fired**, call quality **73.5**. Screenshot `evidence/stage1-3/call-detail.png`. |
| 2.7 Login copy | **FAIL** | `web/src/app/(auth)/login/page.tsx:62` and `:73` still say TCPA compliance intelligence and Ask AI. The accessibility tree on `http://localhost:3101/login` showed that sentence before the existing session painted the dashboard. |
| 2.8 Layout at 1280 and 1920 | **PASS** | `documentElement.clientWidth` was 1280, then 1920. Sidebar and page content sat side by side. No overlapping controls in the captured region. The screenshot frame does not show the full 1920 canvas (the right side is past the image). |
| 2.9 Show demo default off | **PASS** | `showDemo` starts `false` (`date-filter.tsx:39`). The checkbox was unchecked on Dashboard, Pings, and Coverage. KPI queries use `apiSyncExclusionSql`, which drops `apisync-` / `demo-` / `smoke-` unless show-demo is on (`demo-rows.ts:31-38`). The 3,064 / $28,906.34 figure matches the query that excludes those prefixes. |
| 2.10 Console / CSP | **CAN'T VERIFY** | No console-panel capture per page. No error toast was on the pages opened. That is not a console log. |

---

## Part 3 — Stage 3 scoreboard

Severity is only for findings that can affect the money path. 3.8 was not run.

| # | Verdict | Severity | Proof |
|---|---|---|---|
| 3.1 One status derivation | **FAIL** | 🟡 | List filter: `pings.filters.ts:68-81` (`converted` = `has_converted = 1` only). Badge: `shouldHaveConverted` in `web/src/lib/api.ts:49-55` plus `PingsTable.tsx:35-46`. Those two match each other. Call detail shows `Converted: Yes/No` from `has_converted` alone (`CallDeepViewPanel.tsx`). Write-back read-back is a third rule: `hasConverted && revenue > 0` (`writeback/convert.go:103-105`). The owner rule (flag AND amount > 0) is the write-back rule, not the list rule. |
| 3.1 Should-convert predicate | **PASS** (the two copies match) | | API: unconverted AND (revenue > 0 OR duration ≥ required). Web `shouldHaveConverted` is the same predicate. |
| 3.1 False flag on platforms that do not report conversion | **PASS** as a count, code still broad | | Predicate has no platform allowlist, so any such row would be flagged. Current latest rows with `has_converted=0` AND `revenue>0`: **0**. Converted with revenue 0: **119** (these are not the should-convert set). All real calls in this DB are Retreaver, written by Stage 4, which does store `has_converted`. TrackDrive and CallGrid have no call rows here. |
| 3.2 Status column and filter | **PARTIAL** | | Filter buttons exist: All, Converted, Not converted, Should have converted. This audit did not click each filter and compare a count to ClickHouse. |
| 3.2 Pagination | **PARTIAL** | | SQL applies `LIMIT` after the collapse, so a collapsed page should not repeat a call. The first page had no repeated ids. A second page was not fetched. |
| 3.3 Call detail vs ClickHouse | **PARTIAL** | 🟢 | One call opened: `cf878cec-b6db-4e59-9138-414350e9c1b8`. Screen: revenue **$11.00**, converted **Yes**, state PA, platform retreaver. ClickHouse latest: `has_converted=1`, revenue **11**, `event_type=finished`. Two other ids were selected and not opened: `a9bae284-…` (converted 0, revenue 0) and `6ddd3c3f-…` (converted 1, revenue 0). Profit (gross) renders **$0.00** whenever the path is `api_sync`, including when `profit_gross` is missing (`CallDeepViewPanel.tsx:184-185` uses `?? 0`). That does not separate "not provided" from a real 0. Recording and Buyer render as `—`. |
| 3.4 Buttons only for Ringba | **PASS** | | `WRITEBACK_PLATFORMS = ['ringba']` (`ConversionActions.tsx:34`). The Retreaver call's buttons were `AD admin@calyx.ai` and `Open in Retreaver`. No Convert button. |
| 3.4 Admin only | **PARTIAL** | 🟡 | API: `integrations.controller.ts:144-146` throws `Write-back is admin-only`. A locally signed `role=viewer` token got **403** `Write-back is admin-only` on `POST …/writeback/convert` with a fake call id. No Ringba call was made. **UI does not check role.** Both database users are `admin`, so a non-admin screen was not available. |
| 3.4 Amount, no cap, confirm | **FAIL** | 🟡 | Amount must be a finite number `> 0` in the UI and in `ValidateAmount` (`convert.go:82-86`). Confirmation dialog shows the call context and the amount (`ConversionActions.tsx:183-211`). **There is a cap.** `WRITEBACK_MAX_AMOUNT = 25` (`web/src/lib/api.ts:586`). The input sets `max={25}` and disables confirm above 25. The comment says the server enforces that cap. `ValidateAmount` does **not** cap at 25. The owner decision is no cap. |
| 3.4 Double submit | **PASS** | | Button sets `busy` and the confirm control is disabled while `busy` (`ConversionActions.tsx:59,188`). Server lock: `lock.Acquire` then `ErrAlreadyProcessing` (`convert.go:163-171`). `TestDoubleClickOneFire` passed. This is an in-flight lock, not a client idempotency-key header. |
| 3.4 Pending / confirmed / failed | **PASS** (code) | | Toast uses `result.outcome` (`ConversionActions.tsx:109-117`). Settled means `isConverted`: flag AND revenue > 0 (`convert.go:103-105`, `pollConvert` 296-301). Not exercised with a live write. |
| 3.5 `WRITEBACK_ENABLED` | **PASS** | | Routes are registered only when the env is `true`/`1` (`integrations.controller.ts:73-86`, `platform-sync/cmd/server/main.go:151-162`). Compose sets it `true` on api and platform-sync, which is why the 403s above came from a real route rather than a 404. |
| 3.5 Per-tenant `writeback_enabled` | **PASS** | | `convert.go:144-146` returns `ErrTenantDisabled`. `TestTenantDisabled` is in `convert_test.go`. |
| 3.5 CSRF | **PASS** | | Cookie session, `Origin: http://localhost:3101`, no CSRF header: **403** `csrf required`. Cookie session with no Origin: **403** `origin required`. Bearer requests skip CSRF (`auth-security.service.ts:144-146`). The probe used a fake id and did not reach Ringba. |
| 3.5 Tenant isolation | **PASS** | | `get_api_sync_integration_any` filters `id = p_id AND tenant_id = p_tenant_id` (`db/postgres/migrations/000025_writeback_enabled.up.sql:26-33`). A second check rejects a mismatched `item.TenantID` (`convert.go:141-143`). Credentials are decrypted from that row only. |
| 3.5 Amount server-side | **PASS** | | Zod `.positive()` (`integrations.controller.ts:47-53`) and `ValidateAmount`. |
| 3.5 Audit row on every attempt | **FAIL** | 🟡 | Audits are written for not-found, already converted, fire failure, confirmed, and pending. **Not written** for the in-flight lock rejection (`convert.go:167-171` returns `ErrAlreadyProcessing` with no `writeAudit`). Also skipped for invalid amount, missing integration, write-back disabled, and not-live, which fail before the lock. K7 (audit on 409) is **not fixed**. |
| 3.5 Settle window / K10 | **PARTIAL** | 🟢 | `SettleWindow = 12 * time.Second` (`convert.go:15-16`). Pending message says "not settled within 12s". This audit did not time a live read-back, so the 10.4s observation was not re-checked. The constant was not changed. |
| 3.6 No auto convert/void | **PASS** | | Callers of `Service.Convert` / `Service.Void` are `cmd/server/main.go` (the HTTP handler) and `convert_test.go`. Sweeps call the sync window, not `FireInboundConversion`. `[PROVEN]` by search. |
| 3.7 Tests | **PASS** for the Go package that exists | | `go test ./internal/writeback/ -count=1` **ok** (1.855s). `go test ./internal/adapter/ -count=1` **ok** (82.227s). No API Jest test covers the admin 403 or CSRF on these routes. |
| 3.8 Live money path | **CAN'T VERIFY** | | Not approved. No Convert and no Cancel were sent to Ringba or Retreaver. 🙋 |

### Safety requirements with no test

- Nest admin-only 403 (proven here with a signed viewer token, not by a committed test)
- CSRF on the convert/cancel routes
- UI cap of $25 vs server `ValidateAmount` (no test that the server rejects 26, because the server does not)
- Audit row on the 409 in-flight rejection (the behaviour is the opposite)
- Call-detail status using the same predicate as the list
- Non-admin UI hide

---

## Part 4

| Item | Result |
|---|---|
| PowerShell gates | **CAN'T VERIFY.** `pwsh` is not installed. `verify-system.ps1`, `verify-ingestion-slice`, `verify-presentation` were not run. 🙋 |
| Ringba unit tests | **PASS.** `go test ./internal/adapter/` and `./internal/writeback/` exited 0. No Ringba integration was switched to Live. |
| Secret scan `ba9c825..c4e3c5d` | **0 hits** for `api_key=<long value>`, `AKIA…`, or a private-key block on added lines. |
| Server deploy | Rebuild **api**, **web**, and **platform-sync** (mock id prefix). This commit adds **no** migration and does **not** change `deploy/docker-compose.yml`. Do not point migrate at a database already on version 28 without a down file for 28; this branch only has 1–27. `WRITEBACK_ENABLED` is already `true` in the compose file on this branch. |

---

## Findings, most severe first

1. **🟡 UI will not send a conversion above $25, and the server does not share that cap.** Owner decision is no cap. `WRITEBACK_MAX_AMOUNT = 25` disables confirm. `ValidateAmount` only rejects `<= 0`. A caller who bypasses the UI can send a larger amount. `web/src/lib/api.ts:586`, `ConversionActions.tsx:86-88`, `convert.go:82-86`.
2. **🟡 "Converted" on the list and call detail is `has_converted` alone.** Write-back treats a call as converted only when the flag is true **and** revenue > 0. A void that leaves the flag true and the amount 0 (the Ringba shape) still shows as Converted in the list. 119 real rows are converted with revenue 0 right now.
3. **🟡 409 in-flight rejection writes no audit row.** `convert.go:167-171`.
4. **🟡 Convert / Cancel are not hidden for a non-admin.** The API rejects `role !== 'admin'`. The component never reads `role`.
5. **Ping chart ignores 30 days and All time.** Hardcoded 7-day series and a hardcoded "Last 7 days" badge. Dashboard KPIs do follow the period.
6. **v1 scope is not applied to the platform filter or Settings.** TrackDrive, CallGrid, and Webhook are still offered. Call Loom is in the filter.
7. **Login still advertises TCPA compliance intelligence and Ask AI as the product.**
8. **Call detail still shows mock score, TCPA risk, and ad attribution** after the dashboard hid those cards. TCPA risk rendered as **15**. Attribution is labelled Mock.
9. **Coverage total is 12 below the dashboard** for the same 30-day window (3,052 vs 3,064) because blank state is dropped.
10. **`docs/retreaver-stage4-spec.md` is in `c4e3c5d`.** It is not a Stage 1–3 change.
11. **Profit (gross) prints `$0.00` for API-sync calls when the value is missing.**
12. **Migrate cannot start this branch's compose project on a database that is already at version 28.**

---

## Merge recommendation

**Not ready.**

The branch is one commit on top of `main`, and the dashboard totals for the default period match ClickHouse. It is not ready for `main` (which the server pulls) because the ping chart does not follow the selected period, TrackDrive / CallGrid / Webhook are still in the v1 UI, the login page still promises TCPA and Ask AI, and the convert path disagrees with the owner's "no cap" and "flag AND amount > 0" rules. The PowerShell gates were not run. Live Convert/Cancel was not run.

---

## Open questions 🙋

1. Install `pwsh` so `verify-system`, `verify-ingestion-slice`, and `verify-presentation` can be run?
2. Approve one live Convert at $1 and one Cancel, and name the sandbox call? Not done in this audit.
3. The $25 UI cap contradicts "no cap". Confirm the cap should be removed rather than enforced on the server.

---

## Honest limitations

- `pwsh` is missing, so the acceptance gates did not run.
- API and web images were Docker cache hits from 2026-09-29 22:14–22:16Z, about 18 minutes before `c4e3c5d` was committed. The running bundles contain `should_convert` and `Confirm convert`, so the features under review are in the containers. A byte-for-byte "image built after the commit" proof exists only for platform-sync.
- Stage 4 Retreaver data (3,652 calls, $33,143.85 all-time) is in ClickHouse and is inside the totals. There is no separate Ringba call population on this tenant to use for the three-status call-detail check.
- 3.8 was not approved, so the money path is proven only up to the local API guards and the Go tests.
- Console logs were not captured.
- `govulncheck` was run on `services/platform-sync` only (clean). The other Go modules were not scanned.
- Disk was full during the first `go test` (`no space left on device`). Docker build cache was pruned (no volumes removed) and the tests were re-run successfully.
