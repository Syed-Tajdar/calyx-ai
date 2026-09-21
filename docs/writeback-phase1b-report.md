# Calyx — Write-Back Phase 1b Report

**Date:** 2026-09-21  
**Scope:** Close the three Gate 1 FAILs only. No UI. No Phase 2. Write path / settle window / lock / verify-script assertions unchanged.  
**Only write target:** `RGB512060522F815F149F10AD1B143CCF4BD5E5F709V3LIN01`  
**Did not write:** `…V3NYI01` or any other call. Did not re-convert after void.  
**Product path:** Nest `POST /integrations/:id/writeback/{convert,void}` with tenant A JWT.  
**Evidence:** `docs/evidence/ringba-writeback-phase1b/`  
**Spec:** `docs/writeback-phase1b-spec.md`

---

## Gate 1 — is it closed?

**Yes.**

---

## Final scoreboard

| Gate 1 item | Phase 1 | Phase 1b |
|---|---|---|
| Auth proof | PASS | — |
| Two-level gate + migrations with down | PASS | — |
| Cross-tenant refusal | PASS | — |
| Double-click → single fire | PASS | — |
| Confirmed / pending / failed all demonstrated | FAIL | **PASS** — live confirmed convert on virgin call; pending + failed remain from Phase 1 |
| Void + read-back | FAIL | **PASS** — live confirmed void; field signature recorded |
| 400-not-500 on invalid amount | PASS | — |
| Audit rows per attempt | PASS | — (convert + void `confirmed`; 409-no-row gap unchanged) |
| Gate scripts exit 0 | FAIL | **PASS** — slice **0**; system **0** (`go1.22.12 windows/amd64` on PATH) |

---

## Integration state (not changed)

Ringba Live `ce6fd338-af48-47d8-850a-bd10b855e2f4`:

| Field | Value |
|---|---|
| `sync_mode` | `real` (already from Phase 1) |
| `is_active` | `f` (ticker still paused) |
| `writeback_enabled` | `t` |
| `connection_mode` | `api_sync` |
| `WRITEBACK_ENABLED` | `true` on api + platform-sync |

The spec said “leave `sync_mode=mock`”. **Observed state was already `real` + paused.** Changing it to mock would refuse writes (`ErrNotLive`). Changing it was out of scope. Left as-is. **Did not** set `is_active=true` and did not trigger sync. `[PROVEN]`

---

## 1b.1 — Virgin call (before any write) `[PROVEN]`

Read via `LookupCall` (same 7d scan the product uses) plus raw Get Call Log key presence. Not found would have been reported; it was found on page 0 of 724 records.

| Check | Result |
|---|---|
| `hasConverted` | **ABSENT** (`hasConvertedPresent=false`) |
| `conversionAmount` | **ABSENT** |
| `payoutAmount` | **ABSENT** |
| `wasConversionAdjusted` | **ABSENT** |
| `wasPayoutAdjusted` | **ABSENT** |
| `hasConnected` | **true** |
| `campaignName` | `Calyx-Sandbox-DO-NOT-USE` |
| `campaignId` | `CA98abf9d92be446e786f298680a3bbee6` |
| `connectedCallLengthInSeconds` | **96** (whole-call `callLengthInSeconds`=166) |

File: `docs/evidence/ringba-writeback-phase1b/1b1-clean-lookup.txt`

---

## 1b.2 — Live CONFIRMED convert `[PROVEN]`

One Nest request. Amount **$1.00**. Tenant A admin JWT.

```
POST /integrations/ce6fd338-af48-47d8-850a-bd10b855e2f4/writeback/convert
HTTP=200
TIME_TOTAL=10.37s
```

Body (token redacted; none in body):

```
{"outcome":"confirmed","inbound_call_id":"RGB512060522F815F149F10AD1B143CCF4BD5E5F709V3LIN01","campaign_id":"CA98abf9d92be446e786f298680a3bbee6","webhook_name":"My Webhook","amount":1,"has_converted":true,"conversion_amount":1,"audit_id":"96a21128-923c-4620-8c54-89c9a3a322a2","message":"confirmed","offline_conversion_uploaded":"absent"}
```

Settle: confirmed inside the existing **12s** window (~10.4s). Window was **not** too short for this call. Window was **not** changed.

Audit: `outcome=confirmed`, amount `1.00`, action `convert`.

Independent read-back:

| Field | After convert |
|---|---|
| `hasConverted` | present **true** |
| `conversionAmount` | **1.00** |

Matches amount sent. `[PROVEN]`

Nest **202** mapping: this request confirmed with **200**, so 202 stays **`[UNKNOWN]`**. Acceptable per spec.

---

## 1b.3 — Live CONFIRMED void `[PROVEN]`

One Nest void on the same call. No second convert. Call left voided.

```
POST /integrations/ce6fd338-af48-47d8-850a-bd10b855e2f4/writeback/void
HTTP=200
TIME_TOTAL=7.93s
```

```
{"outcome":"confirmed","inbound_call_id":"RGB512060522F815F149F10AD1B143CCF4BD5E5F709V3LIN01","campaign_id":"CA98abf9d92be446e786f298680a3bbee6","amount":1,"has_converted":false,"conversion_amount":0,"audit_id":"cd6a37a6-4c06-4564-8843-2d5792489be0","message":"void confirmed; reversible via a later convert","offline_conversion_uploaded":"absent","ringba_response_ref":"TR47005109-08a8-4c01-8664-aad1822dc8f3","void_reversible":true}
```

Audit: `outcome=confirmed`, amount `1.00`, `ringba_response_ref=TR47005109-08a8-4c01-8664-aad1822dc8f3`.

### Field signature after void on a cleanly converted call `[PROVEN]`

This is the first void on a call whose history this session was: never converted → convert $1 → void.

| Field | Before convert | After convert $1 | After void |
|---|---|---|---|
| `hasConverted` | **ABSENT** | present **true** | present **false** (key stays) |
| `conversionAmount` | **ABSENT** | **1.00** | present, empty / **0** via LookupCall |
| `payoutAmount` | **ABSENT** | **ABSENT** | present **0.00** |
| `wasConversionAdjusted` | **ABSENT** | **ABSENT** | present **true** |
| `wasPayoutAdjusted` | **ABSENT** | **ABSENT** | **ABSENT** |

Calyx `isConverted` (`hasConverted && amount>0`) is false after void. Ringba still shows `hasConverted` **present** at false — not silent-omit. `wasConversionAdjusted=true` is the dirt flag. Do not treat this call as a future confirmed-path virgin.

Reversibility: not re-tested. Still `[PROVEN]` from G-void research. `void_reversible=true` in the product response.

---

## 1b.4 — Verify scripts

Host Go **was** already installed at `C:\Program Files\Go` (`go1.22.12`). Earlier sessions did not see it on PATH. This run prepended `C:\Program Files\Go\bin` (User/Machine PATH already contained it). Scripts were **not** edited.

### `verify-ingestion-slice.ps1` — **exit 0** `[PROVEN]`

`docs/evidence/ringba-writeback-phase1b/verify-ingestion-slice.txt`

```
INGESTION SLICE VERIFY PASS
VERIFY_INGESTION_SLICE_EXIT=0
```

### `verify-system.ps1` — **exit 0** `[PROVEN]`

Earlier attempts failed only because nested `go` was missing (`verify-system-attempt1.txt`, `verify-system-attempt2.txt`). With `go.exe` on PATH:

```
Docker:     14 / 14
Healthz:    10 / 10
API:        PASS (20/20)
Web routes: 13 / 13
WebSocket:  PASS
Unit tests: PASS
Web build:  PASS
Pipeline:   PASS (4/4)
SYSTEM VERIFY PASS
VERIFY_SYSTEM_EXIT=0
```

Evidence: `docs/evidence/ringba-writeback-phase1b/verify-system.txt`

---

## 1b.5 — Phase 1 leftovers

**README:** removed `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`. Product-convert bullet now names `WRITEBACK_ENABLED` + per-tenant `integrations.writeback_enabled`. Docs only.

**409 already-in-progress writes no audit row:** still true. Lock reject happens before `InsertWritebackAudit`. **Not changed.** Known gap: a user’s repeated clicks show in platform-sync logs, not in `writeback_audit`. Phase 1 double-click 409s remain log-only.

---

## What was not done (on purpose)

- No Phase 2 (no status column, no duration threshold productized — 96s is recorded for later).
- No UI.
- No `sync_mode` change; ticker still paused.
- No write-path / 12s window / lock change.
- No write to `…V3NYI01`.
- No re-convert after void.
- Verify-script assertions unmodified.

---

## Stop

Phase 1b convert + void proofs are closed. **Gate 1 is closed.** Wait for an explicit instruction before Phase 2.
