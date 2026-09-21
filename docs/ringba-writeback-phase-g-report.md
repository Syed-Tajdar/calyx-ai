# Ringba Write-Back — Phase G Report

**Date:** 2026-09-18  
**Scope:** Research only. No product code, UI, migrations, or shipping write-back endpoints.  
**Sandbox call (only fire target):** `RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01`  
**Campaign:** `Calyx-Sandbox-DO-NOT-USE` (`CA98abf9d92be446e786f298680a3bbee6`)  
**Target:** `CALYX - TARGET - SANBOX - DO - NOT - USE` (`TAd5332f67bf994754b8a23b8847feb413`)  
**Harness:** `services/platform-sync/cmd/probe-ringba-phase-g/`  
**Evidence:** `docs/evidence/ringba-writeback-phase-g/phase-g.txt`

---

## Verdict

**Convert loop CLOSED. `[PROVEN]`**

`GET` inbound conversion webhook (existing **My Webhook**) with `call_id` + `call_revenue=1.00` + `converted=yes` → HTTP 200 → call log read-back **`hasConverted=true`**, **`conversionAmount=1.00`**.

Earlier Phase F failure was the **dirty call**, not a missing API. That call had already been voided/adjusted. This clean call converted on the first GET.

---

## G0 — Isolation (read-only)

| Assert | Result |
|---|---|
| Campaign is sandbox | **PASS** |
| Target ID matches sandbox | **PASS** |
| `conversionAmount` absent / 0 | **PASS** (key absent) |
| `payoutAmount` 0 | **PASS** (`0.00`) |
| `hasConverted` absent | **PASS** |
| `wasConversionAdjusted` absent | **PASS** |

`hasPayout=true` with `payoutAmount=0.00` was already on the untouched call — do not treat `hasPayout` as “converted”.

No payments / void / annotate in this slice.

---

## G2 — Fire existing My Webhook

Did **not** create a new webhook. Used preexisting `WH748080906786471ba75e9f20bf3f3c81` (“My Webhook”).

| Step | Result |
|---|---|
| URL shape | `https://webhook.ringba.com/hook/{id}?call_id=…&call_revenue=1.00&converted=yes` |
| Method | **GET** (POST not needed) |
| HTTP | **200**, empty body (same as Phase F) |
| Read-back | `conversionAmount=1.00`, `hasConverted=true`, `wasConversionAdjusted` absent |
| Extra webhook | **None created, none deleted** |

`PHASE_G_CONVERT_LOOP=true`

---

## `hasConverted` semantics (pinned)

| Call state | `hasConverted` on Get Call Log | `conversionAmount` |
|---|---|---|
| Clean, never converted | **Key absent** (= not converted) | **Key absent** |
| After inbound webhook convert $1 | **Present `true`** | **`1.00`** |
| After payment/void/override lifecycle (Phase E dirty call) | May stay **present** `true`/`false` even at `$0.00` | Unreliable |

**Product read-back:** treat **absent → false**, **present true → converted**, and always pair with `conversionAmount`. Do not convert a call that already has adjustment flags if you need a clean reconcile.

Get Call Log `valueColumns` still **silent-omit** on the sync path unless requested — the adapter must keep requesting `hasConverted` + `conversionAmount`.

---

## What Phase F got wrong

Hypothesis “target must be Postback/Webhook” was **not required** for this success. Account-level inbound conversion webhook was enough on a **clean** call.

HTTP 200 + empty body is **not** a convert signal. Only call-log read-back is.

---

## Safety

| Rule | Outcome |
|---|---|
| Only this sandbox inbound call ID | **Held** |
| No payments / void / annotate | **Held** |
| No `WebHooks/testConversion` | **Held** |
| Preexisting webhooks not modified/deleted | **Held** |
| Production campaigns | No writes observed |

Sandbox call **left converted at $1.00** as proof. Disposable campaign.

---

## Product (still not shipped)

Phase G **unblocks** gate J for *design*: we now have a reliable convert write + read-back pair.

Do **not** implement Calyx write-back UI/API until:

1. Product chooses inbound webhook fire vs `payments` / `payments/override` (different semantics).
2. Sync adapter maps `hasConverted` (requested column, not silent-omit).
3. Writes stay campaign-allowlisted (sandbox / customer-approved only).

---

## Harness note

`probe-ringba-phase-g` decrypts the paused `Ringba Live` integration blob (`ce6fd338-…`). Scheduler stays paused (`is_active=false`).

---

## G-Void — read-back signal after void (2026-09-19)

**Research only.** Harness: `services/platform-sync/cmd/probe-ringba-gvoid/`.  
**Only write target:** `RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01` (Phase G convert at $1.00).  
**Evidence:** `docs/evidence/ringba-writeback-phase-g/g-void.txt`  
No webhooks touched. No other `RGB` ID. No product code.

### Four-step table

| Step | Action | `conversionAmount` | `hasConverted` present? | `hasConverted` val |
|------|--------|--------------------|-------------------------|--------------------|
| **V0** | read | **`1.00`** | **true** | **true** |
| **V1** | `POST …/calls/void` (void current conv `$1.00` / payout `$0.00`; proven body shape) | response `result`: `conversionAmount=0.0`, `payoutAmount=0.0` HTTP **200** | — | — |
| **V2** | read back | **key absent** (treat as `$0`) | **true** | **false** |
| **V3** | `POST …/calls/payments/override` → `$0/$0` | response `result`: `0.0` / `0.0` HTTP **200** | — | — |
| **V4** | read back | **key absent** (treat as `$0`) | **true** | **false** |

Campaign on every read: `Calyx-Sandbox-DO-NOT-USE`. V0 matched expect conv≈1.00 + present true.

### Decisive answers

1. **After void (V2):** `hasConverted` does **not** go absent. It **stays present** with val **`false`**. It does **not** stick at `true`.
2. **At conv=0.00 (V2 and V4):** `hasConverted` can still be **present**. On this known convert→void history it is present **`false`**, not `true`. The old contaminated call’s **`true` at $0.00** is **refuted** for this path.

### Conclusion `[PROVEN]`

> The reliable way for Calyx to detect Ringba conversion state is: **`hasConverted==true AND conversionAmount>0`**.

`hasConverted` **presence** is not a convert signal (V2/V4: key still present after void). `hasConverted==true` alone would have worked on this void (val became `false`), but amount keys **omit** at `$0`, and a positive convert is the pair proven at V0 (`true` + `$1.00`). Cancel/zero: val `false` or amount not `>0`.

### Cleanup

Call left at conv **absent / $0** (V4). `hasConverted` present `false`. No webhook, other call, or product object changed.
