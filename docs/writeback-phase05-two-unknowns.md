# Calyx — Write-Back Phase 0.5: Two Unknowns (READ-ONLY)

**Date:** 2026-09-19  
**Mode:** Research only. No write of any kind. No convert, void, payments, annotate, or webhook mutation. No product code, UI, or migration. No change to `sync_mode`, allowlist, or integration state. Write routes stayed contained.  
**Account:** paused Ringba Live integration `ce6fd338-af48-47d8-850a-bd10b855e2f4` (token redacted).  
**Endpoints called:** `POST …/calllogs` only.  
**Harness:** `services/platform-sync/cmd/probe-ringba-phase05/`  
**Evidence:** `docs/evidence/ringba-writeback-phase05/phase05.txt`

G-void call (read-only): `RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01`  
U2 window: `2026-09-05T13:36:12Z` → `2026-09-19T13:36:12Z` (14d). Scanned **1716** records.

---

## 1. U1 verdict — is `hasConverted` a stable read-back signal?

**On this call, in this session, `hasConverted` is stable: present `true` with `conversionAmount="0.00"` on every read.** It does not change across ~3.5 minutes, and it does not change between a minimal column set and a full column set. `[PROVEN]`

That matches Phase 0. It does **not** match G-void V4 (`hasConverted` present `false`, amount keys absent). Request shape is **ruled out** as the cause of that earlier difference. `[PROVEN]` Why V4 differed from later reads remains `[UNKNOWN]`.

### U1.1 — three spaced reads (same columns)

Columns each time: `hasConverted`, `conversionAmount`, `payoutAmount`, `wasConversionAdjusted`, `wasPayoutAdjusted`, `hasConnected` (+ `inboundCallId`, `campaignName`).

| Read | time (UTC) | hasConverted (present/val) | conversionAmount | payoutAmount | wasConversionAdjusted | wasPayoutAdjusted |
|------|------------|----------------------------|------------------|--------------|-----------------------|-------------------|
| R1 | 2026-09-19T11:32:37Z | present / `true` | present `"0.00"` | present `"0.00"` | present `true` | present `true` |
| R2 | 2026-09-19T11:33:08Z | present / `true` | present `"0.00"` | present `"0.00"` | present `true` | present `true` |
| R3 | 2026-09-19T11:36:10Z | present / `true` | present `"0.00"` | present `"0.00"` | present `true` | present `true` |

`hasConnected` was present `true` on all three. Campaign on all three: `Calyx-Sandbox-DO-NOT-USE`.

Answers:

1. `hasConverted` is the **same** on all three reads (present `true`). `[PROVEN]`
2. `conversionAmount` is present as `"0.00"` on all three; the key does **not** come and go in this session. `[PROVEN]` Same for `payoutAmount="0.00"`.
3. vs history:

| Capture | when | hasConverted | conversionAmount |
|---|---|---|---|
| G-void V4 | 2026-09-19 (earlier) | present `false` | **absent** |
| Phase 0 | 2026-09-19 (~03:01Z window) | present `true` | `"0.00"` |
| Phase 0.5 R1–R3 | 2026-09-19T11:32Z–11:36Z | present `true` | `"0.00"` |

Phase 0’s `true` / `"0.00"` is what every Phase 0.5 timed read returned. It is **not** still moving in this ~3.5 minute window. `[PROVEN]` Whether it will change again after this session: `[UNKNOWN]`.

### U1.2 — minimal vs full request (back-to-back)

| Read | time | columns requested | hasConverted | conversionAmount | other keys on record |
|---|---|---|---|---|---|
| MIN | 2026-09-19T11:36:11Z | `inboundCallId`, `hasConverted`, `conversionAmount` | present `true` | `"0.00"` | those three only |
| FULL | 2026-09-19T11:36:12Z | identity + amounts + `hasConverted` + adjustment + `hasConnected` + `hasPayout` | present `true` | `"0.00"` | also `payoutAmount="0.00"`, `wasConversionAdjusted=true`, `wasPayoutAdjusted=true`, `hasConnected=true`, `hasPayout=true`, campaign/target |

`hasConverted` does **not** differ between minimal and full. `conversionAmount` does **not** differ. `[PROVEN]`

Unrequested keys are absent on MIN (`payoutAmount`, adjustment, `hasConnected` not asked → not on record). That is column selection, not a change in `hasConverted`.

**What it depends on (this slice):** not time (minutes) and not request shape. `[PROVEN]` those two are ruled out for the live value. The V4 vs later change is therefore not explained by which `valueColumns` were sent. Cause of that historical change: `[UNKNOWN]`.

---

## 2. U2 verdict — do Ringba reason fields explain a convert-miss call?

**No.** On this account and 14-day window, Ringba does not put a reason on Type-B convert-miss calls. `[PROVEN]`

The only reason field that is ever present is `blockReason`, and only on blocked calls (all `"The bid has expired"`). `noConversionReason`, `noPayoutReason`, `incompleteCallReason`, and `offlineConversionUploaded` are **zero** present of 1716. `[PROVEN]`

### U2.1 — presence counts (1716 records)

| Field | records where present (of 1716) | example: what kind of call |
|---|---|---|
| `noConversionReason` | **0** | none — cannot characterise from this account/window |
| `noPayoutReason` | **0** | none — cannot characterise from this account/window |
| `incompleteCallReason` | **0** | none — cannot characterise from this account/window |
| `offlineConversionUploaded` | **0** | none — cannot characterise from this account/window |
| `blockReason` | **239** | blocked; value always `"The bid has expired"` (239/239) |

### U2.2 — the one populated reason field

Example `blockReason` record `…RGB2D02`:

| Field | Value |
|---|---|
| `blockReason` | `"The bid has expired"` |
| `wasBlocked` | `true` |
| `hasConnected` | **absent** |
| `hasConverted` | **absent** |
| revenue / payout | **absent** |
| `campaignName` | Final Expense |
| `noConversionReason` / `noPayoutReason` / `incompleteCallReason` / `offlineConversionUploaded` | **absent** |

This is a **blocked** call, not a convert-miss. `[PROVEN]`

### U2.3 — Type B convert-miss (handful)

Type B rule (same as Phase 0): `hasConnected=true`, `hasConverted` not true, `blockReason` absent, `wasBlocked` not true, not the G-void id.

Count this scan: **939**. `[PROVEN]`

Five consecutive samples; every one has the **same key set**:

```
campaignId, campaignName, hasConnected, hasPreviouslyConnected, inboundCallId, targetName
```

| Sample | inboundCallId (suffix) | campaign | hasConnected | hasConverted | blockReason | revenue/payout | reason fields |
|---|---|---|---|---|---|---|---|
| 1 | `…RGB1A02` | Final Expense | `true` | **absent** | **absent** | **absent** | **absent** |
| 2 | `…RGBA302` | Final Expense | `true` | **absent** | **absent** | **absent** | **absent** |
| 3 | `…RGBB102` | Final Expense | `true` | **absent** | **absent** | **absent** | **absent** |
| 4 | `…RGBC702` | Final Expense | `true` | **absent** | **absent** | **absent** | **absent** |
| 5 | `…RGB9302` | Final Expense | `true` | **absent** | **absent** | **absent** | **absent** |

All five: `hasPreviouslyConnected=false` (key present), `targetName="RingbaX Final Expense paid-search RTT"`.

There is **nothing** on these Type B records that states why they are not converted. The record is: connected, not converted, no reason given. `[PROVEN]` for these five and for the zero presence of the four reason fields on the full 1716.

---

## 3. What this means for the plan (facts only)

- **Read-back on the G-void call now:** `hasConverted` present `true` and `conversionAmount="0.00"` (also `payoutAmount="0.00"`, both adjustment flags `true`). That shape is stable across minutes and across minimal vs full `valueColumns`. `[PROVEN]`
- **We still do not have one proven explanation** for G-void V4 (`hasConverted` present `false`, amount keys absent) vs every later read. Request-column set is not that explanation. `[PROVEN]` / cause `[UNKNOWN]`
- **Type B (the convert-miss Calyx would target):** Ringba does not supply `noConversionReason` or any other reason field on those calls in this window. `[PROVEN]` “Why not converted” is **not** present on the target call type here.
- **`blockReason` is populated**, but only on blocked calls (`wasBlocked=true`, `hasConnected` absent). That is a different outcome from Type B. `[PROVEN]`

---

## 4. Honest limitations

1. U1 spacing is ~30s then ~3 minutes, not hours. Longer drift after this session is untested.
2. U1 is one call (the G-void sandbox id). Stability is proven for that id in this session, not for every converted-then-voided call.
3. U2 is 14 days / 1716 records on this account. The four empty reason fields are uncharacterised here; they may populate on other accounts, older traffic, or outcomes not in this window.
4. The five Type B samples are the first matches in page order; all five are the same Final Expense campaign/target. Count 939 is the full-window Type B total; the extra keys on the other 934 were not printed one-by-one.
5. `isIncomplete` was requested on the U2 scan; it did not appear on the Type B samples (absent, not reported as a reason value).
6. No write was issued. This slice cannot test whether a new convert or void would move `hasConverted` again.
7. Token redacted. ANI not requested.

---

## 5. Found but not fixed

- G-void V4 vs later `hasConverted` mismatch is still unexplained (request shape ruled out; time-of-read within minutes ruled out).
- `noConversionReason`, `noPayoutReason`, `incompleteCallReason`, `offlineConversionUploaded` remain unseen on this account’s 14-day log.
- Type B convert-miss records carry no Ringba reason for non-conversion.
- Containment, `sync_mode`, allowlist, and integration rows were not changed. No write endpoint was called.
