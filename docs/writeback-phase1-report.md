# Calyx — Write-Back Phase 1 Report (Gate 1)

**Date:** 2026-09-19  
**Scope:** Phase 1 only — safe write path, backend. **No UI. No Phase 2.**  
**Images:** `deploy-api` rebuilt this session (Nest 400-message + 202 mapping). `deploy-platform-sync` from the Phase 1 write-path rebuild; kill-switch drill force-recreated it from that image.  
**Evidence:** `docs/evidence/ringba-writeback-phase1/`  
**Sandbox call (only live fire target this account, 7d):** `RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01`  
**Campaign:** `Calyx-Sandbox-DO-NOT-USE` (`CA98abf9d92be446e786f298680a3bbee6`)  
**Tenant A:** `11111111-1111-1111-1111-111111111111` / integration `ce6fd338-af48-47d8-850a-bd10b855e2f4`  
**Tenant B:** `22222222-2222-2222-2222-222222222222` (no Ringba integration row)

---

## Gate 1 scoreboard

No “partially done”. Live Ringba confirmed convert/void could not be shown on this account’s only sandbox call (voided / dirty). That is a **FAIL** on those two Gate 1 bullets, not a pass with an asterisk.

| Gate 1 item | Result |
|---|---|
| Auth proof (unauth rejected / auth accepted) | **PASS** |
| Both gates working; per-tenant migration with `down` | **PASS** |
| Cross-tenant refusal (1.3) | **PASS** |
| Double-click → single fire (1.4) | **PASS** |
| Confirmed / pending / failed all demonstrated (1.5) | **FAIL** — pending and failed live; confirmed live not shown |
| Void + read-back; reversibility confirmed | **FAIL** — live void refused (call not converted); reversibility is prior G-void research + code flag |
| 400-not-500 on invalid amount | **PASS** |
| Audit rows written for each attempt | **PASS** |
| `verify-ingestion-slice`, `verify-system` exit 0 | **FAIL** — host `go` not on PATH |

**Gate 1 is not closed.** Do not start Phase 2.

---

## What shipped (backend)

Two-level gate, Bearer auth on platform-sync write routes, per-tenant credential bind, inflight Redis lock, 12s settle poll with three outcomes, void path, amount validation, immutable audit.

| Piece | Where |
|---|---|
| Global kill-switch | `WRITEBACK_ENABLED` default **off** in `config.Load`; routes registered only when on **and** `WRITEBACK_INTERNAL_TOKEN` non-empty |
| Per-tenant flag | `integrations.writeback_enabled BOOLEAN NOT NULL DEFAULT FALSE` — `db/postgres/migrations/000025_writeback_enabled.{up,down}.sql` |
| Audit | `writeback_audit` insert-only + trigger — `000026_writeback_audit.{up,down}.sql` |
| Auth | `writeback.RequireBearer` (constant-time; empty token fail-closed) |
| Handler | `POST /writeback/convert` and `POST /writeback/void` |
| Nest | JWT + admin-only; forwards Bearer internal token; tenant from JWT; `writeback_enabled` check |
| Allowlist retired | `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS` removed from compose, `.env.example`, and Go config |

Local compose sets `WRITEBACK_ENABLED=true` so proofs can run. Code default remains off.

---

## 1.1 — Authentication on platform-sync

`[PROVEN]` live, rebuilt `deploy-platform-sync`:

| Call | Result |
|---|---|
| `POST :8088/writeback/convert` no `Authorization` | **HTTP 401** `{"message":"unauthorized"}` |
| Wrong Bearer | **HTTP 401** |
| Valid Bearer, empty JSON | **HTTP 400** `integration_id, tenant_id, and inbound_call_id required` — handler reached |

Nest `POST /integrations/:id/writeback/convert` without JWT: **HTTP 401** `[PROVEN]`.

### Out of scope (unchanged, still unauthenticated)

| Route | Result with no auth |
|---|---|
| `POST :8088/sync` empty body | **HTTP 400** `integration_id and tenant_id required` — not 401 |
| `POST :8088/test-auth` empty body | **HTTP 400** same |

Reported, not changed.

---

## 1.2 — Two-level gate

Both must be true or the write does not proceed.

**Global `WRITEBACK_ENABLED`** `[PROVEN]`:

1. Compose temporarily `false`, force-recreate `platform-sync`, `printenv WRITEBACK_ENABLED` → `false`.
2. Valid Bearer `POST /writeback/convert` → **HTTP 404** `404 page not found` (route not registered).
3. Compose restored `true`, recreate, same call with empty body → **HTTP 400** handler reachable; `printenv` → `true`.

**Per-tenant `writeback_enabled`** `[PROVEN]`:

1. `UPDATE integrations SET writeback_enabled=false` on Ringba Live.
2. Tenant A admin convert → **HTTP 403** `Write-back is not enabled for this integration`.
3. Flag restored `true` (`SELECT` → `t`).

Migrations **with downs**: `000025_writeback_enabled.down.sql` restores the `000024` `get_api_sync_integration_any` signature and drops the column; `000026_writeback_audit.down.sql` drops trigger, function, and table.

**Allowlist retired.** `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS` is gone from `deploy/docker-compose.yml`, `.env.example`, and `services/platform-sync/internal/config/config.go`. Per-tenant flag replaces it (no restart to open one tenant). Leftover mention remains in `README.md` — found, not fixed this phase.

---

## 1.3 — Per-tenant credential binding `[PROVEN]`

Credentials come from `get_api_sync_integration_any(request.tenant_id, request.integration_id)`. There is no default/fixed integration id in the write path.

| Attempt | Result |
|---|---|
| Tenant B JWT (`admin-b@calyx.ai`) against Tenant A’s integration id | Nest **HTTP 404** `Integration not found` |
| Internal Bearer + `tenant_id` = Tenant B + Tenant A’s integration id (direct `:8088`) | **HTTP 404** `writeback: integration not found` |
| Tenant A + unknown inbound id `…FAKEPROBE02` | **HTTP 404** `writeback: call not found on this tenant's Ringba account` |

Lookup uses **this tenant’s** decrypted `account_id`. A call that is not in that account’s 7d call log is refused before fire. `[PROVEN]`

This environment has **one** Ringba account. A second live Ringba account’s call id was not available. The Tenant B session vs Tenant A integration proof is the multi-tenant bind; the fake id is the account-mismatch path. Two distinct live Ringba accounts: `[UNKNOWN]` here.

---

## 1.4 — Idempotency `[PROVEN]`

Redis `SET NX` per `(tenant, action, inbound_call_id)`. In-flight TTL 45s; after pending, lock held 15m so a retry is **409**, not a second fire.

Live (sandbox call, amount $1.00), same rebuilt platform-sync:

| Time (UTC) | What |
|---|---|
| 13:44:58 | Second convert **409** `convert already in progress` (during first request’s 12s poll) |
| 13:45:14 | First convert audit **pending** `fire accepted; not settled within 12s` |
| 13:52:57 | Later convert while lock `pending` **409** same message |

`FireInboundConversion` is only reached after the lock. One fire, two 409s. Unit `TestDoubleClickOneFire`: `fires=1`. `[PROVEN]`

Ringba **did not** record a new `conversionAmount` (still `$0.00`, `hasConverted=true` — voided/dirty call). “One conversion on Ringba” is therefore **one webhook GET**, not a new settled conversion. See 1.5.

---

## 1.5 — Settle window and three outcomes

**Window:** 12s, poll every 2s. Phase G converted in ~4s typical; 12s was the proven cap. Not lengthened after the dirty-call miss — a longer window would not have helped (7d read still `$0`).

| Outcome | Live | Unit |
|---|---|---|
| **Pending** | `[PROVEN]` convert $1 on G-void call; HTTP through Nest was **201** on that request (see limitations); audit `pending` | `TestPendingConvert` |
| **Failed** | `[PROVEN]` unknown call → 404 / audit `failed`; void on unconverted call → 409 / audit `failed`; fire-reject | `TestFailedFire` |
| **Confirmed** | **Not live.** Unit `TestConfirmedConvert` **PASS**. Live blocked — see below. | PASS |

**Never reported “failed” for the pending write.** Pending is `outcome=pending` with message `fire accepted; not settled within 12s`. The 409s are already-in-progress, not a false failure of the first fire.

### Reality differs from the plan (stopped; did not infer a code change)

7d sandbox list: **1** match, the G-void call, `hasConverted=true`, `conversionAmount=0.00`, `dirty_money_or_adjust=true`. Phase F/G already showed a voided/adjusted call will not reconcile via inbound webhook.

Calyx `isConverted` is `hasConverted && conversionAmount>0` as specified. That allowed a convert attempt on this voided call. Ringba accepted the webhook HTTP; read-back did not settle amount. **Did not** tighten the already-converted gate to `hasConverted==true` at $0 — the plan’s reconvert-after-void path needs that state to be eligible. A clean never-converted sandbox call was **not present** in 7d. Did not fire a non-sandbox campaign. Did not use `payments/override` to manufacture a converted state.

---

## 1.6 — Void

Same lock, settle, three outcomes. `VoidReversible=true` on success. Body is G-void proven `calls/void` including `voidConverionAmount`.

Live void on the sandbox call **15s after pending convert:** **HTTP 409** `writeback: call is not converted` (read-back still `$0`). Audit `failed`. `[PROVEN]` the refuse path. Live void **confirmed** + reversibility this session: **FAIL**.

Reversibility of Ringba void remains `[PROVEN]` from G-void research (`payments/override`), not re-run here.

---

## 1.7 — Amount validation `[PROVEN]`

No upper cap (unit: `$250` accepted). Nest + Go require `amount > 0`.

| Body | HTTP | Message |
|---|---|---|
| `amount: -1` | **400** | `amount must be a positive number` |
| `amount: 0` | **400** | `amount must be a positive number` |
| amount omitted | **400** | `amount must be a positive number` |

Not a raw 500. Client (UI) validation is **out of scope** this phase (no UI).

---

## 1.8 — Audit trail `[PROVEN]`

Table `writeback_audit`: tenant, user, integration, inbound_call_id, action, amount, requested_at, outcome, ringba_response_ref, campaign_id, message, offline_conversion_uploaded. `BEFORE UPDATE OR DELETE` trigger raises `writeback_audit is immutable`. `[PROVEN]` both UPDATE and DELETE.

Rows this phase:

| action | call | amount | outcome |
|---|---|---|---|
| convert | FAKEPROBE01 | 0 | failed (not found) |
| convert | G-void sandbox | 1.00 | **pending** |
| void | G-void sandbox | 0 | failed (not converted) |
| convert | FAKEPROBE02 | 0 | failed (not found) |

409 already-in-progress does **not** insert an audit row (lock reject before work). That is recorded in platform-sync logs, not the table.

---

## 1.9 — Already-converted / offline-uploaded

Before fire: LookupCall. If `hasConverted && revenue>0` → 409, no fire. `[PROVEN]` unit `TestAlreadyConvertedRefused`. Live already-converted refuse: not shown (no sandbox call with amount > 0).

`offlineConversionUploaded` is read and stored. All four audit rows: **`absent`**. `[PROVEN]` this session. Still **not gated** — no positive live sample of `true`. Behaviour when present remains `[UNKNOWN]`.

---

## Unit tests `[PROVEN]`

```
ok  github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/writeback  0.141s
ok  github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter     81.201s
```

Includes: amount, Bearer 401/200, tenant disabled, cross-tenant missing integration, call not on account, already converted (no fire), confirmed, pending, fire failed, double-click one fire, void confirmed, webhook name pick.

---

## Honest limitations

- Live **confirmed** convert: **FAIL** (dirty sandbox call; no clean 7d sandbox call).
- Live **confirmed** void: **FAIL** (same call not converted at read-back).
- Nest first pending convert returned **HTTP 201** (`@Post` default). Code now maps platform-sync **202** → Nest **202** and confirmed → **200**. That mapping is in the rebuilt `deploy-api` image; **not re-fired** to avoid a second webhook on the dirty call. Live Nest 202: `[UNKNOWN]`.
- Platform-sync pending HTTP 202 is in handler code; the live pending was observed through Nest 201. Direct `:8088` 202 live: `[UNKNOWN]`.
- `/sync` and `/test-auth` remain unauthenticated (out of scope).
- `WRITEBACK_INTERNAL_TOKEN` is service-to-service; whoever has it can put any `tenant_id` in the JSON. Nest binds tenant from JWT. Direct `:8088` is not a user API.
- `LookupCall` scans up to 8 pages of the account’s 7d log. A call outside that window or those pages 404s. Not changed this phase.
- README still mentions the retired campaign allowlist.

---

## Found but not fixed

- `README.md` line still documents `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`.
- Other historical docs (`enterprise-roadmap.md`, contain/audit reports) still describe the contained allowlist.
- No second Ringba account in this environment for a two-account live bind.
- No clean sandbox inbound call in 7d to close the convert loop again.

---

## Verify scripts

`scripts/verify-ingestion-slice.ps1` was run. It logged in, read analytics/coverage (`total_pings=4153` / `4151`), then **exited 1**:

```
go : The term 'go' is not recognized as the name of a cmdlet, function, script file, or operable program.
At scripts\verify-ingestion-slice.ps1:25 char:19
+     $signature = (go run ./cmd/platformsig ringba $query).Trim()
```

`scripts/verify-system.ps1` was **not** started: it also invokes `go test ./...` on eight services. Installing a host Go toolchain was not done.

Write-back + adapter tests **did** run in `golang:1.22-bookworm` and passed (see Unit tests). That is not a substitute for the two named gate scripts.

---

## Stop

Phase 1 backend is in the rebuilt images. **Gate 1 is not closed** because live confirmed convert/void were not demonstrated. **Phase 2 has not been started.** No UI was added. Wait for an explicit instruction before Phase 2.
