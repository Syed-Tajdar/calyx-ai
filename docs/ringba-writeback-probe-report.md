# Ringba Write-Back — Live Verification Probe Report

**Date:** 2026-07-21  
**Scope:** Research only. No product code, UI, migrations, or shipping changes.  
**Account:** Live Ringba integration credentials via encrypted DB blob (token redacted everywhere).  
**Evidence dir:** `docs/evidence/ringba-writeback-probe/`

---

## 0. Safety first

**No write affected any real call.** `[PROVEN]`

| Check | Result |
|---|---|
| Fake inbound call ID | `RGB0000000000000000000000000000000000000000FAKEPROBE01` |
| Non-existence proof | `POST …/calllogs` over ~30d, **3 pages / 2295 records**, `found=false` (`step4-raw.txt`) |
| Write probes executed | **Only** `POST …/calls/annotate` and `POST …/Calls/Annotate` against that fake ID |
| Destructive probes | **Skipped** — annotate did not return clean call-not-found validation (per brief §4.2) |
| `WebHooks/CreateConversion` | **Not probed** |
| `WebHooks/testConversion` | **Not probed** |
| Unexpected create / modify / void / pay | **None observed.** Annotate returned HTTP 500 `"Failed to annotate the call"` with empty `errors`; response was **not** `isSuccessful:true` |

If annotate had returned success despite a non-existent ID, remaining probes would have stopped under the same rule. They were stopped because validation was **ambiguous**, not because success was observed.

---

## 1. Decision table

| Capability | Path (verified) | Status | Label |
|---|---|---|---|
| Read back converted state | `POST /v2/{accountId}/calllogs` + `valueColumns: ["hasConverted"]` | **Usable.** Key present + `true` on converted calls; **absent** when not. See §4. | `[PROVEN]` |
| Read adjustment / dispute state | Column ids from `GET …/calllogs/columns` (see §3) | **Identifiers known.** Live values mostly silent-omitted on this account’s sample (no adjusted/annotated calls in window). | Ids `[PROVEN]`; live true-samples `[UNKNOWN]` |
| Mark converted (webhook) | `POST …/WebHooks/CreateConversion` (official collection) | **Exists in official docs.** Creates a **webhook config**, not a one-shot conversion on a call. **Not probed** (would mutate account). Inbound conversion firing is a separate webhook *receive* path. | Path `[PROVEN]` (docs); live write `[UNKNOWN]` (intentionally untested) |
| Void / cancel | Official: `POST …/calls/void` | **Documented.** Live existence **not confirmed** — skipped after annotate ambiguity. | Docs `[PROVEN]`; live route `[UNKNOWN]` |
| Adjust payment | Official: `POST …/calls/payments` (+ `/override`) | **Documented** as direct revenue/payout adjust. Live existence **not confirmed** — skipped. | Docs `[PROVEN]`; live route `[UNKNOWN]` |
| Request conversion adjustment | Official: `POST …/{buyerAccountId}/calls/requestConversionAdjustment` | **Documented as BUYER → admin pending request** (not auto-apply). Live existence **not confirmed** — skipped. | Semantics `[PROVEN]` (docs); live route `[UNKNOWN]` |
| Create webhook programmatically | `POST …/WebHooks/CreateConversion` | Listed in official collection; `GET …/WebHooks` **200** proves webhook surface live. Create **not probed**. | List `[PROVEN]`; create `[UNKNOWN]` (intentionally untested) |

---

## 2. Did we obtain a current official spec?

**Yes.** `[PROVEN]`

| Attempt | Result |
|---|---|
| `https://documenter.getpostman.com/view/11229784/2sAYBVhryW` | SPA HTML shell only — not a usable OpenAPI dump |
| Documenter JSON API `https://documenter.getpostman.com/api/collections/11229784/2sAYBVhryW` | **HTTP 200**, `application/json`, ~1.4MB Postman Collection v2.0 |
| Saved copy | `docs/evidence/ringba-writeback-probe/postman-collection.json` |
| Collection metadata | Name “Ringba API Documentation”; `publishDate` **2025-02-05**; owner `11229784`; publishedId `2sAYBVhryW` |
| Separate OpenAPI/Swagger URL | **Not found** linked from Ringba docs in this pass |

**This collection supersedes the third-party SDK path casing** for write-back candidates. Current official paths use lowercase `calls/…` and `calllogs/…` (SDK PascalCase is stale for several reporting routes).

### Official write-related paths (from collection — supersede SDK)

| Method | Path | Official title / note |
|---|---|---|
| POST | `/{accountId}/calls/annotate` | Add Annotation to a Call |
| POST | `/{accountId}/calls/void` | Void Revenue and/or Payment for a Call |
| POST | `/{accountId}/calls/payments` | Adjust revenue and/or payout (relative) |
| POST | `/{accountId}/calls/payments/override` | Adjust revenue and/or payout (absolute) |
| POST | `/{buyerAccountId}/calls/requestConversionAdjustment` | **BUYER** request for payout/revenue adjustment (relative) |
| POST | `/{buyerAccountId}/calls/requestConversionOverride` | **BUYER** request (absolute) |
| GET | `/{accountId}/calllogs/columns` | Get Column Names |
| GET | `/{accountId}/calllogs/tags` | Get Tags (collection lists it; **live host returned 301** — see §3) |
| GET | `/{accountId}/WebHooks` | Get Webhooks |
| POST | `/{accountId}/WebHooks/CreateConversion` | Create New Webhook — **do not probe** |

---

## 3. STEP 1 — Read-only probes

Auth: `Authorization: Token {REDACTED}`  
Base: `https://api.ringba.com/v2/{accountId}`

Raw: `docs/evidence/ringba-writeback-probe/step1-raw.txt`  
Columns dump: `docs/evidence/ringba-writeback-probe/calllogs-columns.json` (**109 columns**)

| # | Candidate | Exact URL pattern | Status | Classification |
|---|---|---|---|---|
| 1 | `/WebHooks` | `GET …/WebHooks` | **200** | **Live** — returns `webHooks` array |
| 1b | `/webhooks` | `GET …/webhooks` | **200** | Same payload (case-insensitive) |
| 2 | `/CallLogs/AvailableGroupBys` | `GET …/CallLogs/AvailableGroupBys` | **301** | **Moved** — body points to `https://developers.ringba.com/#cebe3d19-2f2d…` (no usable `Location` header in capture) |
| 2b | lowercase | `GET …/calllogs/availablegroupbys` | **301** | Same move message |
| 3 | SearchFields | `GET …/CallLogs/SearchFields/{converted\|hasConverted\|payoutAdjusted\|hasAnnotations}` (+ lowercase) | **301** | **Moved** — same reporting-move message; omitting `{fieldName}` was not the cause |
| 4 | Columns | `GET …/calllogs/columns` | **200** | **Live** — full column dictionary |
| 4b | PascalCase | `GET …/CallLogs/columns` | **200** | Same |
| 5 | Tags | `GET …/calllogs/tags` (+ PascalCase) | **301** | **Moved** on this host despite collection listing it |

### Columns dump — write-back–relevant ids `[PROVEN]`

Full JSON: `calllogs-columns.json` (109 entries). Subset:

| API `id` | UI title | Type | Roles |
|---|---|---|---|
| `hasConverted` | Converted | boolean | admin, user |
| `hasPayout` | Paid Out | boolean | admin, user, affiliate |
| `conversionAmount` | Revenue | money | admin, user |
| `payoutAmount` | Payout | money | admin, user, affiliate |
| `wasConversionAdjusted` | Conversion Adjusted | boolean | admin, user |
| `wasPayoutAdjusted` | Payout Adjusted | boolean | admin, user |
| `convAdjustmentRequestCount` | Requested Adjustments | long | admin, user |
| `convAdjustmentsApproved` | Approved Adjustments | long | admin, user |
| `convAdjustmentsRejectedCount` | Rejected Adjustments | long | admin, user |
| `convAdjustmentsApprovedAmount` | Approved Adjustment | double | admin, user |
| `hasAnnotations` | Has Annotations | boolean | admin, user |
| `noConversionReason` | No Conversion Reason | keyword | admin, user |
| `noPayoutReason` | No Payout Reason | keyword | admin, user |
| `buyerHasConverted` | Paid out | boolean | **buyer only** |
| `buyerConversionAdjustmentStatus` | Conversion Adjustment Status | keyword | **buyer only** |

**Not present as column ids:** `converted`, `conversionAdjusted`, `payoutAdjusted`, `requestedAdjustments`, `approvedAdjustments`, `rejectedAdjustments`.

**Tags dump:** unavailable on live host (301). Annotate `tagType` vocabulary therefore remains `[UNKNOWN]` from this slice.

---

## 4. STEP 2 — Converted read-back (blocking)

### 4.1 Omission hypothesis — **confirmed for `hasConverted`**

Cross-tab on **200** call-log records (`step2-xtab.txt`):

| Slice | `hasConverted` true | false | absent |
|---|---|---|---|
| `conversionAmount > 0` (n=70) | **70** | 0 | **0** |
| `conversionAmount == 0` (n=130) | 0 | 0 | **130** |

**Rule (document explicitly):** `[PROVEN]`

- Identifier is **`hasConverted`** (title “Converted”).
- **Present + `true` ⇒ converted.**
- **Absent ⇒ not converted** (absence-means-false). Do not treat absence as “unknown.”
- On this sample, `hasConverted` never appeared as `false`.

UI Converted-column eyeball cross-check was **not** performed in this slice; the proxy was `conversionAmount > 0`, which matched `hasConverted` 70/70 and 0/130. `[INFERRED]` that this matches the UI Converted column given the official column title mapping.

### 4.2 `conversionAmount > 0` as sole derivation

| Approach | Verdict |
|---|---|
| Use only `conversionAmount > 0` | **Knowably incomplete** for $0 conversions. Ringba webhook / conversion docs describe conversions recorded at **0.00** revenue. That rule would produce **false negatives**. Label: do not adopt alone. |
| Use `hasConverted` present=true | Preferred read-back. Behaviour for true $0 conversions: **`[UNKNOWN]`** — none observed in the 200-record window (`hasConverted|rev=0` was always absent). |

### 4.3 Generalisation to other omit-when-empty fields

On the same 200 records:

| Field | Present | Notes |
|---|---|---|
| `hasPayout` | 68/200, all `true` | Same present=true / omit-otherwise pattern as `hasConverted` (2 converted calls without payout in window). `[PROVEN]` pattern on sample |
| `noPayoutReason` | 0/200 | Never appeared — even among non-paid calls. Absence-means-false **not proven** for keyword reasons; may omit empty strings differently. `[UNKNOWN]` |
| `noConversionReason` | 0/200 | Same. `[UNKNOWN]` |
| `hasAnnotations` | 0/200 | No annotated calls in window. `[UNKNOWN]` for true-case |
| `wasConversionAdjusted` / `wasPayoutAdjusted` | 0/200 | No adjusted calls in window. `[UNKNOWN]` for true-case |

Failed attempt that included illegal column `converted` returned **422** `"Unknown value column converted"` (`step2-raw.txt`) — confirms `converted` is **not** an API id.

Detail endpoints probed (read-only): `GET …/calllogs/detail` → **200** empty `records`; `…/details` → **404** HTML.

---

## 5. STEP 3 — Adjustment family identifiers

Prefer columns dump (done). Live accept/silent table (`step3-raw.txt`, 50-record page):

| Report column | API id | Live probe |
|---|---|---|
| Converted | `hasConverted` | **ACCEPT_PRESENT** (`true`) |
| Paid Out | `hasPayout` | **ACCEPT_PRESENT** (`true`) |
| Conversion Adjusted | `wasConversionAdjusted` | ACCEPT_SILENT (no true sample) |
| Payout Adjusted | `wasPayoutAdjusted` | ACCEPT_SILENT |
| Requested Adjustments | `convAdjustmentRequestCount` | ACCEPT_SILENT |
| Approved Adjustments | `convAdjustmentsApproved` | ACCEPT_SILENT |
| Rejected Adjustments | `convAdjustmentsRejectedCount` | ACCEPT_SILENT |
| Has Annotations | `hasAnnotations` | ACCEPT_SILENT |
| No Conversion Reason | `noConversionReason` | ACCEPT_SILENT |
| No Payout Reason | `noPayoutReason` | ACCEPT_SILENT |
| (buyer) Paid out | `buyerHasConverted` | **422** not available for role admin |
| (buyer) Conversion Adjustment Status | `buyerConversionAdjustmentStatus` | **422** admin role |

Candidates `conversionAdjusted` / `payoutAdjusted` / bare `requestedAdjustments` etc. are **wrong names** relative to the live columns list — do not probe-guess further.

---

## 6. STEP 4 — Write-endpoint existence (fake ID only)

### 6.1 Non-existence proof `[PROVEN]`

```
FAKE_ID=RGB0000000000000000000000000000000000000000FAKEPROBE01
window_pages=3 total_records_scanned=2295 found=false
```

Evidence: `step4-raw.txt`

### 6.2 Annotate first (safest write-shaped probe)

| Probe | URL | Status | Body (redacted token) | Classification |
|---|---|---|---|---|
| Official casing | `POST https://api.ringba.com/v2/{accountId}/calls/annotate` | **500** | `{"message":"Failed to annotate the call","messageCode":0,"errors":[]}` | Route **likely exists** (not 301/404 HTML). **Not** clean call-not-found. **Not** success. |
| SDK casing | `POST …/Calls/Annotate` | **500** | Same | Same |

**Stop rule applied:** brief requires clean ID validation before void / payments / requestConversionAdjustment. Annotate did **not** clearly say “call not found.” Remaining probes **skipped**.

Interpretation:

- **Route exists and is current** — `[INFERRED]` from 500 application JSON vs moved/absent patterns.
- **ID validation behaviour** — `[UNKNOWN]` (opaque 500).
- **Did not treat as “acted”** — no `isSuccessful:true`, no indication annotation was stored.

### 6.3 Not probed (by design)

- `calls/void`, `calls/payments`, `calls/requestConversionAdjustment`
- `WebHooks/CreateConversion`, `WebHooks/testConversion`
- Any other endpoint

---

## 7. STEP 5 — Semantics of `RequestConversionAdjustment` (docs only)

**Not a live probe.** Sources: official Postman collection descriptions + Ringba product update “Call Disputes and Tasks Management”.

### What Ringba’s own docs say `[PROVEN]`

1. **Postman collection** titles the endpoint **“BUYER Request for Payout Adjustment”** and describes it as: buyer sends a request to the **admin** of the account handling the call; admin is notified; this is a **request**, not a silent direct state change.
2. **Call Disputes product docs** (updates.ringba.com): Buyers submit disputes / void requests; **Administrators** Approve (full or partial) or Deny via **Tasks**. Adjustment applies **after admin approval**. Denied buyers may re-request.
3. Buyer permission flag **“Can Dispute Conversion”** gates UI dispute ability (support Buyers article).

### What remains undetermined `[UNKNOWN]`

- Whether an **API**-submitted `requestConversionAdjustment` is auto-approved for any role (docs describe notice + admin handling; no statement of API auto-approve).
- Whether an **admin** token calling the buyer-path endpoint succeeds, is rejected, or bypasses the queue.
- Exact mapping between UI “Call Void request” and API `calls/void` vs `requestConversionAdjustment` (product copy mixes “void” and “dispute”).

### Why it matters for Calyx

If Calyx uses the **request** endpoint, “cancel from Calyx” means **request a cancellation pending admin approval**, not **cancel**. Direct cancel/adjust would be `calls/void` / `calls/payments` — different financial semantics and risk. Management must be told the difference before anything is built.

---

## 8. Per-probe evidence index

| Step | Artifact |
|---|---|
| Spec | `postman-collection.json` |
| STEP 1 | `step1-raw.txt`, `calllogs-columns.json` |
| STEP 2 | `step2-raw.txt` (422 on illegal `converted`), `step2-xtab.txt` (omission proof) |
| STEP 3 | `step3-raw.txt` |
| STEP 4 | `step4-raw.txt` |

All requests used `Authorization: Token REDACTED` in captures.

---

## 9. Open questions (what would resolve them)

| Question | Resolver |
|---|---|
| Clean call-not-found behaviour for annotate / void / payments | Ringba support ticket, or **sandbox** account with permission to probe writes safely |
| Live confirmation of void / payments / requestConversionAdjustment routes | Same — only after clean ID validation is proven |
| True-sample for adjusted / annotated / reason fields | Find (or create in sandbox) calls with known adjusted/annotated state; re-query columns |
| `$0` conversion + `hasConverted` | Find a call converted at 0.00 revenue; request `hasConverted` |
| Current path for call-log **tags** | Support / updated docs — collection path 301s on live host |
| Annotate request body (`tagType` vocabulary) | Tags endpoint replacement, or support article + sandbox annotate on a disposable call |
| Admin vs buyer token on requestConversionAdjustment | Support clarification |

---

## 10. Honest limitations

1. **Write routes beyond annotate are unverified live** — correctly skipped for financial safety.
2. **Annotate returned opaque 500** — cannot claim clean “route exists + ID rejected” yet; only that the path is not 301/404 and did not report success.
3. **No UI eyeball** of the Converted column for a specific call ID in this slice.
4. **No adjusted/annotated true-samples** in the scanned window — adjustment/annotation read-back ids are known, values unproven when true.
5. **Tags list unavailable** live — annotate payload constraints incomplete.
6. **Third-party SDK remains `[INFERRED]`** for anything not confirmed by Postman collection or live HTTP — and several SDK reporting paths are proven stale (301).
7. Probe tooling lives under `services/platform-sync/cmd/probe-ringba-*` for reproducibility; it is research harness only, not product.

---

## 11. Outside-scope findings (reported, not acted on)

- Official collection also documents `POST …/calls/payments/override` and `POST …/calls/requestConversionOverride` — adjacent to listed probes; **not called**.
- `GET …/calllogs/detail` returns 200 with empty records for the parameters used — not investigated further.
- Existing account already has at least one conversion webhook (`GET /WebHooks` 200) — left untouched.
