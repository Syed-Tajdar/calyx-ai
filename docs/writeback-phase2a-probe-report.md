# Phase 2A probe — per-target conversion rules

Read-only. `GET /targets`, `GET /targets/{id}` for one id, `GET /calllogs/columns`, `POST /calllogs`. No write endpoint. No sync, integration, or config change. No product code left in the tree. No Phase 2B.

Window used for the call-log figures below: `2026-08-23T20:32:22Z`–`2026-09-22T20:32:22Z` (30 days). That response was 4,043 records, 3,598 distinct `inboundCallId`, 5 pages, last page 43. `[PROVEN]`

Two earlier walks of a 30-day window ending two and five minutes sooner also returned **4,043** records and a last page of 43, with **3,400** and **3,709** distinct ids. Same row count, different id sets. `[PROVEN]` Offset pagination on this account is not a stable id list. Figures below are one response, not a census.

## 1. Answer to P3

Not for every call. `GET /targets` does not return Convert on, Length, or Start Call Length On. On a call-log row, `ringTreeWinningBidDynamicDuration` (title **Winning Bid Dynamic Duration**) is a per-call number. In this response every converted call had that field, and every one of those 696 calls had connected duration **and** whole-call length greater than or equal to it (0 below). The field was absent on the other 728 rows. The column whose title is **Dynamic Duration** (`pingDynamicCallLengthInSeconds`) was absent on all 4,043 rows.

## 2. P1 — targets API

`GET /v2/{accountId}/targets` returned **200**, no `Location` header, **21** objects under `targets`. The body had no `totalCount`, `page`, or `next`. `[PROVEN]`

`docs/evidence/ringba-writeback-probe/postman-collection.json` is not in the workspace. The fallback path was not used, because this URL returned 200.

`GET /v2/{accountId}/targets/{id}` for the first id returned **200**. Extra keys beyond the list object are `caps`, `stats`, `transactionId`, and a `target` wrapper. The `target` object does not add a conversion-rule field the list omitted. `[PROVEN]`

One list object, token and phone redacted, account id replaced:

```json
{
  "accountId": "{accountId}",
  "blockRecordings": false,
  "capsMigrated": true,
  "conversionTimerOffset": 0,
  "enabled": true,
  "highRateCPM": 0,
  "highRateCost": 0,
  "id": "TA0f75c52edae14ce59b29990f053f1163",
  "instructions": {
    "callType": "number",
    "connectionTimeOut": 10,
    "number": "[REDACTED]"
  },
  "isCriteriaInverted": false,
  "isHighRateTarget": false,
  "name": "DA_ADT",
  "owner": { "id": "BUe3a7e5e00400467a9a5ad90d8a26a9db", "name": "Direct Agents" },
  "schedule": { "timeZoneId": "Eastern Standard Time", "concurrencyCap": -1 },
  "targetCallIncrement": "onConvert",
  "version": 1
}
```

`schedule` caps and `owner.capacity` are omitted above; they are cap counters, all `-1` on this object except the timezone.

| Ringba UI concept | In the response? | Field name | Example |
|---|---|---|---|
| Convert on (call length / connected / dialed / postback) | no | — | — |
| Length (seconds) | no | — | — |
| Start Call Length On (Incoming / Dial / Connect) | no | — | — |

Fields that are present and are not those three:

| Field | What the response shows |
|---|---|
| `conversionTimerOffset` | `0` on all 21 targets. Support articles retrieved for this probe do not define this name. What it controls is `[UNKNOWN]`. |
| `instructions.connectionTimeOut` | Integer on the target (0, 10, 15, 20, or 30 on this account). Not named Length. |
| `targetCallIncrement` | `"onConvert"` on the sample object. Not read back for the other 20 in this probe. |
| `criteria` | Present on some targets. Contents are tag/zip filters (`Geo:ZipCode`, `Address:State`, `InboundNumber:State`), not a convert-on value. |

Conversion settings are not in the targets API response. `[PROVEN]`

## 3. P2 — rules on this account

The three rule columns are not in the payload, so they cannot be filled.

| Target name | Target ID | Convert on | Length (s) | Start counting on |
|---|---|---|---|---|
| DA_ADT | `TA0f75c52edae14ce59b29990f053f1163` | not in response | not in response | not in response |
| Soleo - Bath remodel 120sec | `TA6573ba8cd3474ef583148cc2cd0cc655` | not in response | not in response | not in response |
| Direct Agents / Vivint | `TAe2acb80aca1445849a383aede6607df0` | not in response | not in response | not in response |
| AGM FE IB | `TA67c4304fdec44a4080088b4eab997623` | not in response | not in response | not in response |
| OC24 - Fire/Smoke/Water/Mold 120s | `TA01772ae1cd57429ea031cb1e1abfdb41` | not in response | not in response | not in response |
| Soleo Roofing | `TAc960559846d34965b4e4be8150bfe109` | not in response | not in response | not in response |
| Nick Davis | `TAc70790d35e214d9cacde4e5cd74411f5` | not in response | not in response | not in response |
| Soleo - Hvac | `TA7da527bb4f51487b9b9b9894736a919e` | not in response | not in response | not in response |
| ACA Static 10s  - | `TA6806780f91bd45488746d7166db0ce24` | not in response | not in response | not in response |
| CALYX - TARGET - SANBOX - DO - NOT - USE | `TAd5332f67bf994754b8a23b8847feb413` | not in response | not in response | not in response |
| Aragon Advertising - Solar $45/120s | `TAc7b8a258782f47adb8f69722b6f12a1b` | not in response | not in response | not in response |
| CX3 - Medicare - Dynamic | `TA12a2e4e3faa544bfbf6e4e00a239bb22` | not in response | not in response | not in response |
| Ace Solutions - Cecil Samuals Med Transfers | `TAc51c39fb23794d2b8c67764f23b25f84` | not in response | not in response | not in response |
| Benu.io - USA Home Adviser - RevShare - 4988 | `TAa1ccc2971fe84736a67a5bf6882f48a6` | not in response | not in response | not in response |
| Soleo - Home security - 120sec | `TAbeab20789e1b447f8439c22508ea9f94` | not in response | not in response | not in response |
| Soleo - Pest Control - $48/120s | `TA0fe3fb99c11a4b66908249068434eee9` | not in response | not in response | not in response |
| ACA Transfers Xen | `TAd6f1b08f48fa4ef4aca665d188d0e0c8` | not in response | not in response | not in response |
| C3 - ACA - Dynamic $ | `TA60909119a08d41948ebe817a7620285c` | not in response | not in response | not in response |
| Soleo Window Replacement, 180 secs | `TA26cfbe2a63064f489bf0df531c9eb12b` | not in response | not in response | not in response |
| Aragon Advertising - Windows $30/120s | `TA2e3b7c5ec2e44827b33cb6f15e4a88e8` | not in response | not in response | not in response |
| RPM - Static _ 135s | `TAc42abf1a4e79401fa5269b1eb2c0b713` | not in response | not in response | not in response |

Distinct rules: **not readable** from this response. Whether any target uses Postback/Webhook: `[UNKNOWN]`.

The call-log rows in this window do not use those `TA…` ids, except two. Calls that have a `targetId` are on these ids:

| Name on the call | `targetId` | In `GET /targets`? | Rows with this `targetId` |
|---|---|---|---|
| RingbaX Final Expense paid-search RTT | `PI26c665f49b8149a2ada0b52047205657` | no | 2,596 connected-or-converted in the comparison below |
| RingbaX Medicare … RTT | `PI3ab69e33666342bf913461aeae9df589` | no | 61 |
| Health Quotes - Med IB RTB | `PIb7b3bccd30734768b3e8e37561856d25` | no | 42 |
| Ticket ID (41949) - XY7 Final Expense - XY7 Final Expense RT | `PIea385bd547d9459fad4644a8f50d4b44` | no | 32 |
| Health Quotes - FE IB RTB | `PI3e925cb4533e4354b7db9424d4436df9` | no | 29 |
| CALYX - TARGET - SANBOX - DO - NOT - USE | `TAd5332f67bf994754b8a23b8847feb413` | yes | 4 connected, 0 converted |
| ACA Transfers Xen | `TAd6f1b08f48fa4ef4aca665d188d0e0c8` | yes | 2 connected, 0 converted |

`PI…` ids are not in the 21-target list. `[PROVEN]` Ringba's ring-tree help page says a ring tree has its own revenue settings (static or dynamic call length) which a ring tree target can override. `[PROVEN]` that page exists. This probe did not call a ring-tree endpoint. Whether that endpoint returns Convert on / Length / Start Call Length On is `[UNKNOWN]`.

## 4. P3.1 — Dynamic Duration

`GET /calllogs/columns` returned **110** columns. Title match, not a guessed id:

| UI title | API id | Type |
|---|---|---|
| Dynamic Duration | `pingDynamicCallLengthInSeconds` | timespan |
| Winning Bid Dynamic Duration | `ringTreeWinningBidDynamicDuration` | timespan |
| Winning Bid Max Duration | `ringTreeWinningBidMaxDynamicDuration` | timespan |

No column description text was returned (`description` empty on these objects).

Over the 4,043-row response, key present vs absent (Ringba omits the key when it does not send the value):

| Field | present | absent | Values in this response |
|---|---|---|---|
| `pingDynamicCallLengthInSeconds` | 0 | 4,043 | none |
| `ringTreeWinningBidDynamicDuration` | 3,315 | 728 | 0×5, 10×111, 15×49, 35×10, 36×3, 40×13, 45×2, 60×33, 90×2,416, 95×1, 100×379, 105×3, 120×281, 125×9 |
| `ringTreeWinningBidMaxDynamicDuration` | 81 | 3,962 | 200×39, 250×42 |

`pingDynamicCallLengthInSeconds` was absent on all 3,315 rows where `ringTreeWinningBid` was present, and on all 728 rows where it was not. `[PROVEN]` The column titled Dynamic Duration was not populated on the ring-tree rows in this window. The operator's statement that this column is RTB-only is not confirmed here: it was absent on both sides.

`ringTreeWinningBidDynamicDuration` was present on the same 3,315 rows as `ringTreeWinningBid`, and absent on the same 728. `[PROVEN]` That column is populated exactly where a ring-tree winning bid is present, in this response.

## 5. P3.2 — other per-call fields

Columns whose id or title matched duration, convert, dynamic, ring tree, routing, criteria, minimum, winning bid, call length, or connected were requested, except `buyer*` columns. Those four returned **422** `not available for this role(admin)`: `buyerCallLengthInSeconds`, `buyerConnectedCallLengthInSeconds`, `buyerHasConnected`, `buyerHasConverted`.

Also requested, because the title is a conversion reason: `noConversionReason`, `incompleteCallReason`. Both were **absent on all 4,043 rows**.

No column title in the 110 is "Convert on", "Start Call Length On", or "Length". `[PROVEN]`

Other accepted columns that are not a duration requirement: `ringTreeWinningBid` (money), `ringTreeWinningBidMinimumRevenueAmount` (money; values seen include 0.00, 8.00, 10.00, 20.00), `avgPingTreeBidAmount`, `winningBid`, `winningBidCallAccepted`, `winningBidCallRejected`, `hasRpcCalculation` (title Has Predictive Routing, absent on all 4,043 in the prior walk the same evening). None of these is a seconds-to-convert field.

## 6. P3.3 — target and duration

Converted means `hasConverted` true and `conversionAmount` > 0. `conversionAmount` is a string in the JSON (`"13.00"`). One row had `hasConverted` true and amount parsing to 0; it is not in the converted counts. Not-converted means connected, not blocked, and not in the converted set. Percentiles are linear interpolation at `p*(n-1)` on the sorted values.

Connected duration (`connectedCallLengthInSeconds`):

| Target | Converted n, min, p10, median | Not converted n, max, p90, median | Single boundary |
|---|---|---|---|
| RingbaX Final Expense paid-search RTT | 643, 13, 106.4, 635 | 1,953, 100, 73.0, 24 | overlap. miss max 100, conv min 13. 1,586/1,953 misses are ≥ conv min |
| RingbaX Medicare … RTT | 31, 109, 138, 465 | 30, 68, 50.2, 27.5 | gap (68, 109] **in this response only** |
| Health Quotes - Med IB RTB | 15, 125, 166.6, 427 | 27, 80, 73, 39 | gap (80, 125] |
| XY7 Final Expense RT | 2, 307, 307.6, 310 | 30, 105, 93.2, 44 | gap (105, 307] |
| Health Quotes - FE IB RTB | 5, 35, 35, 35 | 24, 88, 78.5, 32.5 | overlap. miss max 88, conv min 35. 7/24 misses are ≥ conv min |
| CALYX sandbox target | 0 | 4, max 96, median 12 | no converted calls |
| ACA Transfers Xen | 0 | 2, max 4 | no converted calls |

Whole-call length (`callLengthInSeconds`), same groups: RingbaX FE overlap (conv min 18, miss max 108); Medicare gap (74, 114]; Med IB RTB gap (83, 129]; XY7 gap (109, 312]; FE IB RTB overlap (conv min 35, miss max 93).

A walk two minutes earlier, also 4,043 rows, had Medicare connected conv min **67** and miss max **68** (overlap), not 109 and 68. `[PROVEN]` A per-target gap in one page set is not stable for that target.

Against `ringTreeWinningBidDynamicDuration` on the same response (connected, not blocked):

| | connected duration ≥ field | connected duration < field | call length ≥ field | call length < field |
|---|---|---|---|---|
| Converted (696) | 696 | 0 | 696 | 0 |
| Not converted | 1 | 2,063 | 111 | 1,953 |

The one not-converted row at or above the field on connected time: `RGB587DCF02F86A16B85286995AC559F7D4576BEDD1RGBD002`, target RingbaX Final Expense paid-search RTT, connected 40, call length 51, field 35. `noConversionReason` was absent.

On RingbaX FE the field is not one number. The response-wide histogram is dominated by 90 (2,416), 100 (379), 120 (281), and 10 (111), and that target holds 2,596 of the 2,760 compared rows. A single target-level cut is not what the rows carry. The two `TA…` targets that appear have the field absent (`noReq` 4 and 2) and no converted calls.

P1 has no Length to compare the field to.

## 7. P3.4 — the eight short converted calls

Same eight ids as the Phase 2A 10–30s bucket. All are RingbaX Final Expense paid-search RTT (`PI26c665f49b8149a2ada0b52047205657`). `wasConversionAdjusted` was **absent** on all eight (the column was requested; the key was not in the record). Two other rows in this response had `wasConversionAdjusted` true. These eight are not those two. `ringTreeWinningBidMaxDynamicDuration` was absent on all eight. P1 has no rule for this `PI…` id.

| Call | Connected s | Call length s | Amount | Winning Bid Dynamic Duration |
|---|---|---|---|---|
| `…RGB7302` | 13 | 18 | 13.00 | 10 |
| `…RGB1002` | 14 | 19 | 31.50 | 0 |
| `…V3KXZ01` | 15 | 18 | 15.00 | 10 |
| `…RGB6202` | 17 | 23 | 15.00 | 10 |
| `…RGBCE02` | 20 | 26 | 15.00 | 10 |
| `…V38FU01` | 21 | 25 | 13.00 | 10 |
| `…RGBE502` | 22 | 26 | 15.00 | 10 |
| `…V3WYX01` | 27 | 31 | 13.00 | 10 |

Seven sit on a per-call value of 10. One sits on a present value of 0. They are the same target as the long conversions, not a second target, and they are not the two `wasConversionAdjusted=true` rows.

## 8. P4 — which clock

Start Call Length On is not on `GET /targets` and not in the column list. Which start event each target uses is `[UNKNOWN]`.

On the eight short calls the whole-call length is 3, 5, 3, 6, 6, 4, 4, and 4 seconds above connected duration.

In the comparison table, 111 not-converted calls are at or above the per-call field on **call length**, and 1 is at or above it on **connected duration**. Every converted call clears both, which follows if call length is at least connected duration. The field lines up with connected duration in this response. `[PROVEN]` for this page set. It is not identified as Incoming, Dial, or Connect.

## 9. Limitations

- Three finished walks each returned 4,043 rows and a different distinct-id count (3,709, 3,400, 3,598). Medicare's converted minimum moved from 67 to 109 between the last two.
- `noConversionReason` and `incompleteCallReason` were requested and absent on every row, so they do not say why a call did not convert.
- `buyer*` duration and converted columns are rejected for this admin token.
- `conversionTimerOffset` is unexplained by the articles retrieved here.
- Ring-tree and campaign endpoints were not called.
- Column descriptions in the dump are empty. Titles are the labels Ringba returned.

## 10. Found, not changed

- `GET /targets` does not contain conversion settings, and it does not contain the `PI…` targets that the calls in this window used.
- The column titled Dynamic Duration was empty on every row of this window. Winning Bid Dynamic Duration was present on the ring-tree winning-bid rows and absent otherwise.
- Offset pagination still returns a full-looking page set that omits a different id set on the next walk.
- No integration row, sync flag, ClickHouse migration, or product code was changed. Phase 2B was not started.
