# Calyx — Write-Back Phase 0: Field Verification (READ-ONLY)

**Date:** 2026-09-19  
**Mode:** Research only. No write of any kind. No convert, void, payments, annotate, or webhook mutation. No product code, UI, or migration. No change to `sync_mode`, allowlist, or integration state.  
**Account:** paused Ringba Live integration `ce6fd338-af48-47d8-850a-bd10b855e2f4` (credentials decrypted in-process; token redacted).  
**Endpoints called:** `GET …/calllogs/columns` and `POST …/calllogs` only.  
**Harness:** `services/platform-sync/cmd/probe-ringba-phase0/`  
**Evidence:** `docs/evidence/ringba-writeback-phase0/phase0.txt`

Window for live records: `2026-09-05T03:01:52Z` → `2026-09-19T03:01:52Z` (14d). Scanned **1716** call-log records (2 pages).

---

## 1. Field verification table

Columns endpoint: **HTTP 200**, `application/json`, **110** entries (prior dump was 109). `[PROVEN]`

Title → id mapping uses the dump `title` field (exact, case-insensitive). One title collision (`Connected`) was resolved by a live accept/422 on this admin token, not by guessing.

| UI title | API `id` | type | roles | Label | Sample live value this pull |
|---|---|---|---|---|---|
| Converted | `hasConverted` | boolean | admin, user | `[PROVEN]` | Type A **absent**; Type B **absent**; G-void **present `true`** |
| Block Reason | `blockReason` | keyword | admin, user | `[PROVEN]` | Type A **`"The bid has expired"`**; Type B **absent** |
| No Conversion Reason | `noConversionReason` | keyword | admin, user | id `[PROVEN]`; live value on A/B **absent** | Type A **absent**; Type B **absent** |
| No Payout Reason | `noPayoutReason` | keyword | admin, user | id `[PROVEN]`; live value on A/B **absent** | Type A **absent**; Type B **absent** |
| Incomplete Call Reason | `incompleteCallReason` | keyword | admin, user | id `[PROVEN]`; live value on A/B **absent** | Type A **absent**; Type B **absent** |
| Offline Conversion Uploaded | `offlineConversionUploaded` | boolean | admin, user | id `[PROVEN]`; live value on A/B **absent** | Type A **absent**; Type B **absent** |
| Blocked | `wasBlocked` | boolean | admin, user | `[PROVEN]` | Type A **`true`**; Type B **absent**. Dump id is **`wasBlocked`**, not `isBlocked`. |
| Connected | `hasConnected` | boolean | admin, user, affiliate | `[PROVEN]` | Type A **absent**; Type B **`true`**. Dump also lists buyer-only `buyerHasConnected` (same title); that id returned **422** `"not available for this role(admin)"`. |
| Previously Connected | `hasPreviouslyConnected` | boolean | admin, user, affiliate | `[PROVEN]` | Type A **`false`** (key present); Type B **`false`** (key present) |
| Conversion Adjusted | `wasConversionAdjusted` | boolean | admin, user | `[PROVEN]` | Type A **absent**; Type B **absent**; G-void **`true`** |
| Payout Adjusted | `wasPayoutAdjusted` | boolean | admin, user | `[PROVEN]` | Type A **absent**; Type B **absent**; G-void **`true`** |
| Has Annotations | `hasAnnotations` | boolean | admin, user | id `[PROVEN]`; live true-sample **not seen** | Type A **absent**; Type B **absent**; G-void **absent** |
| TCPA Shield Count | `tcpaCount` | long | admin, user | id `[PROVEN]`; live value **absent on all 1716** | always **absent** |
| TCPA Shield Blocked | `wasBlockedByTCPA` | boolean | admin, user | id `[PROVEN]`; live value **absent on all 1716** | always **absent** |
| TCPA Shield Cost | `tcpaCost` | money | admin, user | id `[PROVEN]`; live value **absent on all 1716** | always **absent** |
| Error Message | — | — | — | `[UNKNOWN]` | No dump entry with title `Error Message`. Not substituted. |
| Provider Id | — | — | — | `[UNKNOWN]` | No dump entry with title `Provider Id` / `ProviderId`. Not substituted. |

### P0.1 table (as specified)

| UI title | Expected API id (prior / to confirm) | Confirmed id | type | roles |
|---|---|---|---|---|
| Converted | `hasConverted` | `hasConverted` | boolean | admin, user |
| Block Reason | `blockReason` ? | `blockReason` | keyword | admin, user |
| No Conversion Reason | `noConversionReason` | `noConversionReason` | keyword | admin, user |
| No Payout Reason | `noPayoutReason` | `noPayoutReason` | keyword | admin, user |
| Incomplete Call Reason | `incompleteCallReason` ? | `incompleteCallReason` | keyword | admin, user |
| Offline Conversion Uploaded | ? | `offlineConversionUploaded` | boolean | admin, user |
| Blocked | `isBlocked` ? | `wasBlocked` | boolean | admin, user |
| Connected (was call connected) | ? | `hasConnected` | boolean | admin, user, affiliate |
| Previously Connected | ? | `hasPreviouslyConnected` | boolean | admin, user, affiliate |
| Conversion Adjusted | `wasConversionAdjusted` | `wasConversionAdjusted` | boolean | admin, user |
| Payout Adjusted | `wasPayoutAdjusted` | `wasPayoutAdjusted` | boolean | admin, user |
| Has Annotations | `hasAnnotations` | `hasAnnotations` | boolean | admin, user |
| TCPA Shield Count | ? | `tcpaCount` | long | admin, user |
| TCPA Shield Blocked | ? | `wasBlockedByTCPA` | boolean | admin, user |
| TCPA Shield Cost | ? | `tcpaCost` | money | admin, user |
| Error Message | ? | `[UNKNOWN]` | — | — |
| Provider Id | ? | `[UNKNOWN]` | — | — |

Dump titles that exist nearby but were **not** mapped onto the missing UI titles: `GoogleAds Upload Error Code` (`googleAdsUploadErrorCode`), `Publisher ID` (`publisherId`), `Buyer ID` (`buyerId`), `Campaign ID` (`campaignId`), `Target ID` (`targetId`). Those are different titles. `[PROVEN]` they exist; they are **not** Error Message / Provider Id.

CallInfo UI labels `IsDuplicate` / `BlockedReason` were not rows in the P0.1 table. Dump titles that exist: `Is Duplicate` → `isDuplicate` (boolean; admin, user, affiliate); `Block Reason` → `blockReason` (already above). No dump title `BlockedReason`.

---

## 2. A-vs-B comparison

**Type A** = first record in the 14d scan whose `blockReason` equals `"The bid has expired"`. Count in window: **239**. `[PROVEN]`

**Type B** = first record with `hasConnected==true`, `hasConverted` not true (absent or not yes), `blockReason` absent, `wasBlocked` not true, excluding the G-void sandbox id. Count in window: **939**. `[PROVEN]`

Talk duration was **not** requested. Type B is “connected” by `hasConnected`, not by talk-time.

| Field | Blocked call (A) | Convert-miss call (B) |
|---|---|---|
| inboundCallId | `RGB6BEAEA84640DD1F2791393C0FC85AFCFA48AE2DBRGB1D02` | `RGB784DE4A8D79875FD6B7064F4CD9BFFB1E6D04B3FV359T01` |
| campaignName | Final Expense | Final Expense |
| Connected (`hasConnected`) | **absent** | **`true`** |
| Converted (`hasConverted`) | **absent** | **absent** |
| Block Reason (`blockReason`) | `"The bid has expired"` | **absent** |
| Blocked (`wasBlocked`) | **`true`** | **absent** |
| No Conversion Reason | **absent** | **absent** |
| Offline Conversion Uploaded | **absent** | **absent** |
| Incomplete Call Reason | **absent** | **absent** |
| Revenue (`conversionAmount`) | **absent** | **absent** |
| Payout (`payoutAmount`) | **absent** | **absent** |
| Previously Connected | `false` (present) | `false` (present) |
| targetName | **absent** | `RingbaX Final Expense paid-search RTT` |
| Adjustment / annotations / TCPA | **absent** | **absent** |

### Distinguishing fields (this pair)

On these two records, the keys that differ are:

| Key | A | B |
|---|---|---|
| `hasConnected` | absent | present `true` |
| `wasBlocked` | present `true` | absent |
| `blockReason` | present `"The bid has expired"` | absent |
| `targetName` | absent | present |

`hasConverted` is **absent on both**. It does not separate A from B on this pair. `[PROVEN]`

Type B exists in this account’s 14d window (939 records under the rule above). `[PROVEN]`

---

## 3. TCPA Shield status

Requested `tcpaCount`, `wasBlockedByTCPA`, `tcpaCost` on all **1716** scanned records.

| Field | present | nonempty |
|---|---|---|
| `tcpaCount` | 0 / 1716 | 0 |
| `wasBlockedByTCPA` | 0 / 1716 | 0 |
| `tcpaCost` | 0 / 1716 | 0 |

Live values on this account, this window: **always absent**. `[PROVEN]`

Whether TCPA Shield is **enabled** on this account: `[UNKNOWN]`. Absence is compatible with a disabled add-on and with silent-omit of zero. This pull cannot separate those.

---

## 4. Converted-then-voided read (P0.4)

Call: `RGB0B96293B9F8915CA75C5CCDCF7BCFD0CFAAEDB9CV3NYI01` (Phase G convert, then G-void). **Read only this pass.**

Phase 0 live keys:

| Field | Present | Value |
|---|---|---|
| `hasConverted` | yes | `true` |
| `conversionAmount` | yes | `"0.00"` |
| `payoutAmount` | yes | `"0.00"` |
| `wasConversionAdjusted` | yes | `true` |
| `wasPayoutAdjusted` | yes | `true` |
| `hasAnnotations` | **no** | absent |
| `hasConnected` | yes | `true` |
| `campaignName` | yes | `Calyx-Sandbox-DO-NOT-USE` |

**Phase 0 read signature `[PROVEN]` (this timestamp):**  
`hasConverted` present `true` · `conversionAmount="0.00"` · `payoutAmount="0.00"` · `wasConversionAdjusted=true` · `wasPayoutAdjusted=true` · `hasAnnotations` absent.

Prior G-void V4 capture (`docs/evidence/ringba-writeback-phase-g/g-void.txt`): `hasConverted` present `false`; `conversionAmount` / `payoutAmount` keys **absent**. Adjustment columns were not requested on that V4 read. Both captures are `[PROVEN]` at their times. This slice does not explain the difference.

---

## 5. Offline Conversion Uploaded (P0.5)

| Item | Result | Label |
|---|---|---|
| API id | `offlineConversionUploaded` | `[PROVEN]` (dump title match) |
| type | boolean | `[PROVEN]` |
| roles | admin, user | `[PROVEN]` |
| Type A | key **absent** | `[PROVEN]` |
| Type B | key **absent** | `[PROVEN]` |
| G-void | key **absent** | `[PROVEN]` |

On A and B (neither converted), the key is silent-omit. This pull has **no** record with the key present, so it does **not** show a difference between “already offline-converted” and “never converted.” `[PROVEN]` behaviour on A/B: absent. Distinguishing power: `[UNKNOWN]` (no positive sample).

---

## 6. What is now `[PROVEN]` vs still `[UNKNOWN]`

### `[PROVEN]` (this slice)

- Columns dictionary still live: HTTP 200, 110 entries.
- UI title → API id for every P0.1 row except Error Message and Provider Id (table above).
- `Blocked` id is `wasBlocked`, not `isBlocked`.
- Admin-token Connected id is `hasConnected` (buyer twin `buyerHasConnected` 422).
- Type A live: `blockReason="The bid has expired"`, `wasBlocked=true`, `hasConnected` absent, `hasConverted` absent, revenue/payout absent. 239 such rows in 14d.
- Type B exists: `hasConnected=true`, `hasConverted` absent, `blockReason` absent, `wasBlocked` absent, revenue/payout absent. 939 such rows in 14d.
- A vs B differ on `hasConnected`, `wasBlocked`, `blockReason` (and `targetName` on this pair). `hasConverted` does not differ.
- TCPA Shield columns accepted in `valueColumns`; values absent on 1716/1716.
- G-void adjustment flags: `wasConversionAdjusted=true`, `wasPayoutAdjusted=true`; `hasAnnotations` absent.
- `offlineConversionUploaded` id/type; absent on A, B, and G-void.

### `[UNKNOWN]`

- Error Message — no dump title.
- Provider Id — no dump title.
- Live **true** sample for `hasAnnotations`.
- Live **non-absent** sample for `noConversionReason`, `noPayoutReason`, `incompleteCallReason`, `offlineConversionUploaded`.
- Live **non-absent** sample for `tcpaCount`, `wasBlockedByTCPA`, `tcpaCost`.
- Whether TCPA Shield is enabled on this account.
- Whether `offlineConversionUploaded` is present after a real offline upload.
- Why this G-void read shows `hasConverted=true` at `$0.00` while G-void V4 showed `hasConverted=false` with amount keys absent.

---

## 7. Honest limitations

1. Window is 14 days / 1716 records, not the full account history.
2. Type A and Type B rows are the **first** matches in page order, not a hand-picked UI call.
3. Type B uses `hasConnected`, not connected-call length. “Talked to buyer” as talk-time is not in this pull.
4. Boolean/keyword silent-omit: absent is reported as absent. `hasPreviouslyConnected=false` **was** present; `hasConnected` / `hasConverted` false was not observed on A/B.
5. `buyerHasConnected` and other buyer-only columns are not readable on this admin token (422).
6. Prior `docs/evidence/ringba-writeback-probe/calllogs-columns.json` is not in the repo; reconcile used this live dump, not that file.
7. G-void `hasConverted` this read ≠ G-void V4 read. Both recorded; cause not established.
8. Token redacted. ANI not requested.

---

## 8. Found but not fixed

- Repo is missing the prior 109-column `calllogs-columns.json` artifact (live dump is now 110).
- `Error Message` and `Provider Id` are not in the columns dictionary under those titles.
- TCPA Shield columns never appear on this account’s 14d sample.
- G-void call’s `hasConverted` / amount-key shape is not the same as the V4 capture.
- Write routes remain uncalled. Containment, `sync_mode`, allowlist, and integration rows were not changed.
