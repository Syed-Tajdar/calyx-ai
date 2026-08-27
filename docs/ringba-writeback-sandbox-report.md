# Ringba Write-Back — Sandbox Verification Report

**Date:** 2026-07-27  
**Scope:** Research only. No product code, UI, migrations, or shipping changes.  
**Account:** Production/running Ringba account — isolation is **campaign-level only**.  
**Sandbox call (only write target):** `RGBC281C0CEFCB973E29DDA985A8F7C5EE522897406V3GHG01`  
**Evidence:** `docs/evidence/ringba-writeback-sandbox/`  
**Harness:** `services/platform-sync/cmd/probe-ringba-sandbox/` (research binary only)

---

## ⚠️ Relative vs absolute — unmissable line

**`[PROVEN]` `POST …/calls/payments` is RELATIVE (adds). `POST …/calls/payments/override` is ABSOLUTE (sets).**  
Observed on the sandbox call: override → `$10`, then payments `+$5` → `$15` (not `$5`).

Getting this backwards on a live `$25` call produces `$50`. Do not “remember” the Postman titles alone — this was live-verified.

---

## 1. Final capability table

| Capability | Endpoint (live-verified) | Semantics | Label |
|---|---|---|---|
| Read back converted state | `POST …/calllogs` + `hasConverted` | Present=`true` means converted on never-touched calls; **after payment writes, key may stay present as `true`/`false` even at `$0.00`** | `[PROVEN]` (prior + this slice) |
| Mark converted (webhook inbound) | `POST …/WebHooks/CreateConversion` + fire `webhook.ringba.com/hook/{id}` | **Create + getUrl work.** Firing returned HTTP 200 but **call did not convert** on read-back (sandbox + known-good “My Webhook”). Full loop **not** closed. | Create/getUrl `[PROVEN]`; fire→convert `[UNKNOWN]` |
| Set revenue (absolute) | `POST …/calls/payments/override` | Sets `newConversionAmount` / `newPayoutAmount` | `[PROVEN]` |
| Adjust revenue (relative) | `POST …/calls/payments` | Adds `adjustConverionByAmount` / `adjustPayoutByAmount` (note Ringba spelling `Converion`) | `[PROVEN]` |
| Void / cancel | `POST …/calls/void` | Subtracts void amounts → `$0` in result; **reversible** via `payments/override` afterward | `[PROVEN]` |
| Annotate | `POST …/calls/annotate` | Collection body with `tagType: "Annotations"`; validates call IDs (fake → 500 fail; sandbox → 200 success) | `[PROVEN]` |
| Create webhook programmatically | `POST …/WebHooks/CreateConversion` | Creates enabled conversion webhook; preexisting untouched | `[PROVEN]` |
| Retrieve webhook URL | `POST …/WebHooks/{id}/getUrl` | Returns URL template (not in Postman collection; **live 200**) | `[PROVEN]` |
| Delete webhook | `DELETE …/WebHooks/{id}` | Deletes only specified id | `[PROVEN]` |
| Buyer adjustment request | — | Needs buyer sub-account / buyer token | `[UNKNOWN]` (out of scope) |

---

## 2. Void semantics + reversibility

**`[PROVEN]`**

| Step | Result |
|---|---|
| Before void | `conversionAmount=10`, `payoutAmount=10`, `hasConverted=true` |
| `POST …/calls/void` (void $10/$10) | HTTP 200 `result: {conversionAmount:0, payoutAmount:0}` |
| After void (call log) | Amounts omitted/`0`; `hasConverted` present **`false`**; `wasConversionAdjusted`/`wasPayoutAdjusted` remain **`true`** |
| `payments/override` → `$5` after void | **Succeeded** → `5.00` / `5.00`, `hasConverted=true` |
| **Reversible?** | **Yes** — void does not permanently lock the call against later absolute override |

“Cancel” in Calyx via void is therefore **undoable** with a later override (on this account/API). Product UX must still treat void as a financial action.

---

## 3. Sandbox final state (read-back, not assertion)

| Check | Result |
|---|---|
| Call amounts | `conversionAmount=0.00`, `payoutAmount=0.00` |
| Flags | `hasConverted=true`, `hasPayout=true`, `wasConversionAdjusted=true`, `wasPayoutAdjusted=true` |
| Sandbox webhook `WHf49f4ba001d346d994a6eea6379c6ccb` | **Deleted**; `GET /WebHooks` ids = `[WH748080906786471ba75e9f20bf3f3c81]` only (“My Webhook”) |
| Preexisting webhooks | Untouched throughout |
| Writes to any other `RGB…` | **None** — only sandbox ID + verified fake ID |
| Calyx ClickHouse `pings` | **0 rows** for this `external_call_id` (not synced into Calyx) |

Note: ending with `hasConverted=true` at `$0.00` is a side effect of the payment/void/override lifecycle (and/or late webhook interaction). Amounts are zero; adjustment flags remain true. Disposable sandbox call — acceptable.

---

## 4. Phase B — Isolation verify (no write)

Evidence: `phase-b.txt`, `final-state.txt` (Calyx count)

### Call ID format

`…V3GHG01` **resolved** — 1 match in call log (90d scan). Not a blocker.

### Requested columns (actual)

| Field | API value |
|---|---|
| `campaignName` | `Calyx-Sandbox-DO-NOT-USE` |
| `publisherName` | `Calyx - Test` |
| `targetName` | `CALYX - TARGET - SANBOX - DO - NOT - USE` (API spelling **SANBOX**) |
| `targetId` | `TAd5332f67bf994754b8a23b8847feb413` |
| `buyer` | **absent** |
| `conversionAmount` / `payoutAmount` | **absent** at first read (treated as 0) |
| `hasConverted` / `hasPayout` | **absent** at first read |
| `profitGross` | absent |
| `hasRecording` | `true` |
| `recordingUrl` | present (`media.ringba.com/recording-public?REDACTED`) |

### Assertions

| Assert | Result |
|---|---|
| Campaign is sandbox (not production) | **PASS** — name is sandbox; exact operator string differed by **case** (`CALYX-SANDBOX-…` vs `Calyx-Sandbox-…`) |
| Amounts 0 | **PASS** (keys absent → 0) |
| `hasConverted` absent | **PASS** (initial) |
| Target ID matches | **PASS** exact |
| Target name exact | **FAIL** exact string — API has typo `SANBOX` vs operator `SANDBOX`; still clearly sandbox target |

**Publisher/buyer (evidence, not trust):** publisher `Calyx - Test` as stated. Buyer column empty — API did not return a buyer name on this record.

**DoD B:** met for isolation purposes (target ID + sandbox campaign + $0). Name typos recorded.

---

## 5. Phase C — Annotate 500 resolution

Evidence: `phase-c.txt`

**Collection body (verbatim shape):**
```json
{
  "inboundCallId": "<id>",
  "tags": [
    { "tagType": "Annotations", "tagName": "Any Name", "tagValue": "Any Value" }
  ]
}
```

| # | Call ID | Body | Status | Body snip |
|---|---|---|---|---|
| C1 | Fake `…SANDBOXFAKE01` (2128 records scanned, `found=false`) | valid `Annotations` | **500** | `Failed to annotate the call` |
| C2 | Sandbox call | valid `Annotations` | **200** | `isSuccessful: true` |
| C3 | Fake | `tagType: NOT_A_REAL_TAGTYPE_XYZ` | **500** | same opaque fail as C1 |

**Decision:** C1 fails, C2 succeeds → **prior 500 was ID validation (opaque), not a dead route.** Endpoint validates call IDs. **`tagType` that worked: `Annotations`.**

C3 did not differ from C1 — server does **not** distinguish invalid `tagType` from bad ID when ID is fake (both opaque 500).

C2 read-back: `hasAnnotations` still absent immediately after success — annotation may not surface on that column quickly, or column semantics differ. Annotation itself succeeded (`isSuccessful: true`). Annotations are permanent per Ringba docs — left on disposable sandbox call.

**DoD C:** met. Proceeded to D.

---

## 6. Phase D — Destructive route existence (fake ID)

Evidence: `phase-d.txt`  
Fake ID proof: 2128 records, `found=false`.

| # | Path | Status | Body | Class |
|---|---|---|---|---|
| 1 | `POST …/calls/void` | **400** | `Could not find Call with given id` | **live** |
| 2 | `POST …/calls/payments` | **400** | same | **live** |
| 3 | `POST …/calls/payments/override` | **400** | same | **live** |

Buyer-path `requestConversionAdjustment` / `requestConversionOverride` **not probed** (open item).

**DoD D:** met.

---

## 7. Phase E — Relative vs absolute + void

Evidence: `phase-e.txt`, `phase-e-void.txt`

| Step | Write | Read-back |
|---|---|---|
| E0 | — | amounts absent / 0 |
| E1 | override → 10/10 | **10.00 / 10.00**, `hasConverted=true` |
| E2 | payments +5/+5 | **15.00 / 15.00**, `wasConversionAdjusted=true` |
| Verdict | | **payments=RELATIVE, override=ABSOLUTE `[PROVEN]`** |
| E3 | override → 0/0 | amounts cleared; flags show adjusted |
| E4 | set 10 → void 10 | void result 0/0; log `hasConverted=false` |
| E4b | override → 5 after void | **5.00 / 5.00** — reversible |
| E5 | override → 0 | **SANDBOX_RESTORED_ZERO=true** |

**DoD E:** met.

---

## 8. Phase F — Conversion webhook loop

Evidence: `phase-f1.txt` … `phase-f5.txt`, `phase-f3-diag.txt`, `phase-f4-diag.txt`

### F.1 Create — `[PROVEN]`

- Before: only `WH748080906786471ba75e9f20bf3f3c81` (“My Webhook”)
- Created: `WHf49f4ba001d346d994a6eea6379c6ccb` (`CALYX-SANDBOX-CONV`)
- Body used (plan values + collection shape):
  ```json
  {
    "name": "CALYX-SANDBOX-CONV",
    "incomingCallIdQueryStringParameter": "Call Token",
    "conversionAmountQueryStringParameter": "Revenue Token",
    "conversionCriteriaParameterMatch": "Fire Conversions"
  }
  ```
- Preexisting untouched: **true**

### F.2 getUrl — `[PROVEN]`

`POST …/WebHooks/{id}/getUrl` → 200  
URL shape (secrets N/A):  
`https://webhook.ringba.com/hook/{id}?Call Token=[callUUID]&Revenue Token=[conversionAmount]&Fire Conversions`  
(Not listed in the 2025-02-05 Postman collection; live anyway.)

### F.3 Fire — HTTP 200, **no conversion on read-back**

| Attempt | Result |
|---|---|
| POST sandbox webhook (encoded params, $1.00) | status 200, empty body |
| GET variants / diagnostic | status 200, empty body |
| GET preexisting “My Webhook” (`call_id` / `call_revenue` / `converted=yes`) with sandbox call ID | status 200, empty body |
| F4 read-back after fires | `hasConverted=false` (or still not converted) |

**Full feature loop (fire → `hasConverted` + `$1.00`) not proven.**

Likely blockers (not fully isolated):
1. **Operator UI step incomplete:** target must convert on Postback/Webhook (plan F.3). Not verified via API in this slice.
2. Call already voided/adjusted — may reject inbound conversion posts (`[INFERRED]`).
3. Space-containing query param names are awkward; even the known-good webhook failed to convert this call.

### F.5 Cleanup — `[PROVEN]`

- Override zero issued; webhook **deleted**; only preexisting “My Webhook” remains.

**DoD F:** partially met — create/getUrl/cleanup yes; fire→convert **no**.

---

## 9. Honest limitations

1. Production account — campaign isolation only; safety depends on $0 internal publisher/buyer (API showed publisher; buyer field empty).
2. Postman collection remains dated/stale in places; live responses are authority (`getUrl` missing from collection but works; `calllogs/tags` still 301 from prior work).
3. Webhook conversion end-to-end **not** closed — do not claim Calyx can mark converted via webhook until a successful read-back exists.
4. After payment writes, `hasConverted` omission-means-false rule from the prior slice is **incomplete**: key can remain present as `true`/`false` at `$0.00`.
5. `hasAnnotations` did not appear immediately after successful annotate.
6. Sandbox call never appeared in Calyx ClickHouse (`pings` count 0).
7. Buyer-path adjustment APIs untested.
8. Recording was on for this call (operator note) — disable for future sandbox calls.

---

## 10. Found but not fixed / open items

| Item | Next step |
|---|---|
| Webhook fire does not convert this call | Operator: set sandbox target conversion to **Postback/Webhook**; re-fire on a **fresh** sandbox call (no prior void/adjust), read back `hasConverted` + amount |
| Prefer clean query param names | Recreate webhook with `call_id` / `call_revenue` / `converted=yes` (match working “My Webhook” shape) |
| Buyer `requestConversionAdjustment` | Buyer sub-account token |
| Sync sandbox call into Calyx | Live sync / wider window — optional E2E evidence only |
| `hasAnnotations` read-back lag | Wait longer or find alternate annotation column after annotate |

---

## 11. Safety summary

| Rule | Outcome |
|---|---|
| Only sandbox call ID written (plus fake ID for existence probes) | **Held** |
| Fake ID verified absent before write-shaped probes | **Held** |
| No `WebHooks/testConversion` | **Held** |
| No buyer-path probes | **Held** |
| Preexisting webhook not deleted/modified | **Held** |
| Unexpected money movement on production campaigns | **None observed** |
| Sandbox webhook removed | **Confirmed by `GET /WebHooks`** |
| Sandbox amounts at 0 | **Confirmed by call log read-back** |
