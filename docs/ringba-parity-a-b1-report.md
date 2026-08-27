# Ringba call-detail parity — Bucket A + B1 report

**Date:** 2026-07-23  
**Scope:** Ringba only. Deliverable for Part 2 (A + B1).  
**Integration:** `ae17d28e-3f98-4288-abf9-ef1bfa978776` (“Real R. Test”) — flipped **Live** for republish, returned to **Test** (`sync_mode=mock`) at close.  
**Evidence dir:** `docs/evidence/ringba-parity-a-b1/`

Official column meanings: [Call Details Report Column Dictionary](https://support.ringba.com/hc/en-us/articles/26894023126551).  
API identifiers: proven against live `POST /v2/{accountId}/calllogs` responses (not guessed from display names).

---

## 1. Scoreboard

| Item | Result |
|------|--------|
| **P-A** | **PASS** |
| **P-B1.1** | **PASS** |
| **P-B1.2** | **PASS** |
| **P-B1.3** | **PASS** |
| **P-B1.4** | **PASS** |
| **P-B1.5** | **PASS** |

PNG screenshots were not captured in-session (no browser automation). **Deployed Docker API deep-view JSON** is the primary proof for UI field presence; call URLs for hard-refresh verification are listed under Limitations.

---

## 2. Decisions (up front)

| Decision | Choice | Justification |
|----------|--------|---------------|
| **Duration** | Calyx `duration_seconds` = Ringba **`connectedCallLengthInSeconds`** (Connected Call Length / talk time) | Call-quality and billing care about connected talk time. Whole-call **Duration** (`callLengthInSeconds`) is also stored as `call_length_seconds` and labelled **Total call length** in the UI so the two are never conflated. |
| **Per-call RPC** | **Not applicable** | On a single call, RPC ≡ revenue. Show Revenue / Payout / Profit (gross). Close audit row as N/A. |
| **Tags** | **Deferred** | Modern Get Call Log returns requested tag columns as **flat keys** (e.g. `tag:InboundNumber:State`). No nested `tags[]` bag on sampled records. Full custom/user/SIP/annotation tag bag not in this response shape without further discovery — roadmap, not forced into B1. |
| **Converted** | **Absent (API identifier not proven)** | `converted` / `isConverted` / `Converted` → HTTP 422. `hasConverted` accepted but **never appeared** on sampled records (silent omit). Do **not** invent from `conversionAmount`. |
| **Hash / republish** | **(a) Accept deliberate one-time republish** | ~30d lookback forced once (clear cursor + `last_sync_at`). Content-hash correctly treated new B1 fields as changed. Schema-versioned hash rejected as unnecessary for this volume. |
| **Recording URL** | **B2 only** | `hasRecording` in B1. Live API also accepts `recordingUrl` (found, not mapped) — see Found / roadmap G. |

---

## 3. Updated parity table

| Field (audit) | Status |
|---------------|--------|
| Inbound Call ID | **closed** (already Yes) |
| Revenue | **closed** (already Yes) |
| Payout | **closed** — rendered; API vs webhook vs $0 criteria distinguished |
| RPC (per-call) | **not-applicable** — same as revenue |
| Campaign ID / name | **closed** — name primary, ID secondary |
| Publisher ID / name | **closed** |
| Target ID / name | **closed** |
| Buyer | **closed** — `buyer` + `buyerId` |
| State | **closed** (already Yes) |
| Duration | **closed** — Connected length from Ringba; independent of transcript |
| Timestamp | **closed** — `occurred_at` (`callDt`) preferred; fallback `received_at` |
| Recording URL | **deferred-with-trigger** — B2 / roadmap G (Has Recording boolean **closed** in B1) |
| Transcript | **closed** (already Yes when pipeline ran) |
| Tags (full bag) | **deferred-with-trigger** — state tag remains; full bag not in flat calllogs shape |

---

## 4. P-B1.1 — Display name → API identifier map

### Mapping table (live evidence)

| Calyx field | Ringba display name | API `valueColumns` id | Present in real response? | Sample (redacted) |
|-------------|---------------------|-------------------------|---------------------------|-------------------|
| payout | Payout | `payoutAmount` | Yes | `24.48` |
| campaign_id | Campaign ID | `campaignId` | Yes | `(id)` |
| campaign_name | Campaign | `campaignName` | Yes | `Final Expense` |
| publisher_name | Publisher | `publisherName` | Yes | `Optimize To Convert` |
| target_id | Target ID | `targetId` | Yes | `(id)` |
| target_name | Target | `targetName` | Yes | `RingbaX Final Expense…` |
| buyer_name | Buyer | `buyer` (**not** `buyerName`) | Yes | `RingbaX` |
| buyer_id | Buyer ID | `buyerId` | Yes | `(id)` |
| duration (chosen) | Connected Call Length | `connectedCallLengthInSeconds` | Yes | `244` |
| call_length (extra) | Duration | `callLengthInSeconds` | Yes | `247` |
| has_recording | Has Recording | `hasRecording` | Yes | `true` |
| converted | Converted | *(none proven)* | **Absent** | — |
| occurred_at | Call Date | `callDt` | Yes | `(timestamp)` |
| is_duplicate (platform) | Duplicate | `isDuplicate` | Yes | `false` |
| profit_gross | Profit | `profitGross` | Yes | `6.12` |

**Rejected examples (HTTP 422 Unknown value column):** `campaign`, `buyerName`, `connectedCallLength`, `converted`, `duplicate`, `profit`, `Duration`, `Campaign`, …

**Silent drop (HTTP 200, key never on record):** `hasConverted`, `targetGroupId`, `targetSubId`, `noPayoutReason`.

### Response size / duration (`size=1000`)

| | Bytes | Duration |
|--|------:|---------:|
| Baseline (9 cols) | 315 971 | ~1075–1186 ms |
| Baseline + accepted B1 cols (25) | 992 305 | ~2030 ms |

No client-timeout risk at current 60s HTTP client timeout. Request count / quota posture unchanged (same pagination).

### Tags

- Requested geo tags appear as **flat record keys** (`tag:InboundNumber:State=CA`).
- No nested `tags` array on first 50 records in the 7d window.
- Full tag families (custom / user / SIP / annotations) **deferred**.

### Discovery endpoints

`GET …/calllogs/searchfields` and `…/availablegroupbys` (and PascalCase variants) → **HTTP 301** “endpoint moved” to developers.ringba.com (SPA; not usable for identifier dump). Identifiers therefore taken from **live calllogs accept/reject + response keys only**.

---

## 5. P-A — Render fields already present

**Change:** `CallDeepViewPanel` + deep-view `occurred_at` select (CH already had column; not a sync mapper / hash change for P-A alone).

**Fields:** Payout, Campaign ID, Target ID, Call time (`occurred_at` → else `received_at`).

**Proof (deployed API after Docker rebuild):**  
`docs/evidence/ringba-parity-a-b1/api-mode-deep-view.json` — includes `payout`, `campaign_id`, `target_id`, `occurred_at`.

**UI verify URL:** `http://localhost:3001/calls/RGB0025CE7041ED3F874DC0F0AAB9F1EF8F89D8D403RGB6C02?platform=ringba` (hard-refresh).

P-A shipped in the same rebuild as B1.3; mapper/hash changes are attributable to **B1.3**, not P-A.

---

## 6. P-B1.2 — Content-hash republish plan + proof

### Expected volume

| Query | Result |
|-------|--------|
| Ringba CH 30d before force lookback | **931** uniq / rows higher |
| Strategy | **(a)** one-time full lookback (cleared `sync_cursor` + `last_sync_at`) |

Without clearing cursor, live tick only covers **last_sync − 2h overlap** (~2 publishes) — insufficient for B1 field backfill. Forced lookback is the planned burst.

### Live sync log (evidence)

```
api sync completed … platform=ringba calls=3334 published=2133 unchanged=1201 page1_refresh=true
```

CH within 20m of republish: `with_name=2135`, `with_duration=1752`, `api_sync_calls=2135`.

### Analytics before / after (30d window, `argMax` per call)

| Phase | uniq_calls | sum argMax(revenue) | sum argMax(payout) |
|-------|------------|---------------------|--------------------|
| before | 931 | 9847.76 | 3408.94 |
| after | 2673 | 24504.61 | 14658.38 |

**Interpretation:** uniq and revenue **rose because lookback ingested Ringba history not previously in CH**, not because `argMax` double-counted. Per-call check remains one logical call:

- Aggregate pattern: `(platform, external_call_id)` + `argMax(*, received_at)`.
- Example multi-version call still `uniqExact=1` with single latest revenue.

### Enrichment before / after (ReplacingMergeTree load test)

| Phase | `count()` | `uniqExact(tenant_id, ani_hash, attribute_key)` |
|-------|-----------|--------------------------------------------------|
| before | 3 197 540 | 3 197 540 |
| after (pre-OPTIMIZE) | 4 233 900 | 4 027 460 |
| after `OPTIMIZE … FINAL` | **4 027 460** | **4 027 460** |

Rows converged to unique count after merge. Growth vs before ≈ new ANIs from expanded lookback, **not unbounded duplicate attrs**.

---

## 7. P-B1.3 — Implement

| Piece | Detail |
|-------|--------|
| Migration | `db/clickhouse/migrations/000011_pings_ringba_parity_b1.{up,down}.sql` (+ compose `clickhouse-init` phase11) |
| Adapter | Extended `valueColumns` + mapper for names, buyer, durations, recording, profit, `isDuplicate` |
| Hash | `packages/pingdedup` Content append-only B1 fields |
| Kafka/CH | platform-sync producer, consumer insert, ingestion `sync_path=webhook` |
| UI | Names primary / IDs secondary; Connected length + Total call length; Has Recording |

**Duration without transcript:**  
`RGB0025CE7041ED3F874DC0F0AAB9F1EF8F89D8D403RGB6C02` — `duration_seconds=17`, scoring duration `0` / no useful transcript score — see evidence JSON.

**Unit tests:** `pingdedup`, `adapter`, `syncer` PASS (Windows GOOS).

---

## 8. P-B1.4 — Webhook vs API vs empty payout

| State | Treatment | Evidence |
|-------|-----------|----------|
| Value present | Render | `api-mode-deep-view.json` (names, durations, recording) |
| Not provided via webhook | Copy: *“Not provided via webhook — available in API mode”* | `webhook-mode-deep-view.json` (`sync_path=webhook`, empty B1 fields) — synthetic CH row `webhook-parity-b14-demo` for UI contract |
| Legitimately empty payout | `$0.00` + muted *“No payout — criteria may not have been met”* (not error styling) | `zero-payout-deep-view.json` — `Roofing IB`, `payout=0`, `sync_path=api_sync` |

Webhook demo row is **synthetic** (labelled call id) because the live webhook integration did not produce a convenient blank B1 sample during this window.

---

## 9. P-B1.5 — Target field set (TrackDrive / CallGrid)

**Label: canonical target field set for TrackDrive Phase 3 and CallGrid Phase 4.**

| Calyx field | Ringba source | Expected universal? | Notes for TrackDrive / CallGrid |
|-------------|---------------|---------------------|----------------------------------|
| external_call_id | `inboundCallId` | **universal** | Required join key |
| occurred_at | `callDt` | **universal** | Platform call time |
| revenue | `conversionAmount` | **universal** | Gross earnings |
| payout | `payoutAmount` | **universal** | May be 0 when criteria unmet |
| duration_seconds | `connectedCallLengthInSeconds` | **universal** | Prefer talk/connect time if platform has both |
| call_length_seconds | `callLengthInSeconds` | platform-specific | Whole-call; keep if available |
| state | `tag:InboundNumber:State` | **universal** | Geo; source varies |
| campaign_id / campaign_name | `campaignId` / `campaignName` | **universal** | Name optional if only ID exists |
| publisher_id / publisher_name | `publisherId` / `publisherName` | **universal** | Affiliate/publisher |
| target_id / target_name | `targetId` / `targetName` | platform-specific | Buyer-side “target”; TD/CG may use different entity |
| buyer_id / buyer_name | `buyerId` / `buyer` | platform-specific | Ringba buyer model |
| has_recording | `hasRecording` | platform-specific | Boolean only in B1 |
| profit_gross | `profitGross` | platform-specific | Optional money |
| platform_duplicate | `isDuplicate` | platform-specific | Platform-declared dup |
| sync_path | Calyx | **universal** | `api_sync` \| `webhook` for empty-state UX |
| converted | *(unproven)* | deferred | Do not invent |

---

## 10. B2 — Roadmap

Updated `docs/enterprise-roadmap.md` §G: Has Recording in B1; URL/storage/player deferred; PII gates; trigger customer requirement OR GA. Note: live API accepts `recordingUrl` — still B2.

---

## 11. Rebuild / verify

```text
docker compose build api platform-sync web consumer ingestion  → OK
docker compose up -d --force-recreate …                     → OK
Ringba sync_mode: real (work) → mock (end)                  → OK
```

| Check | Result |
|-------|--------|
| `verify-system` Docker/health/API/routes/web build | PASS |
| `verify-system` WS / pipeline / unit (first run) | FAIL — host `GOOS=linux` left from Linux probe binaries broke `go run ./cmd/platformsig` |
| `verify-ingestion-slice` (after clearing GOOS) | **PASS** |
| `node scripts/ws-smoke-test.mjs` (after clearing GOOS) | **PASS** (3 events) |
| `verify-api-sync.ps1` | **Not run** (known stale vs C3; roadmap H) |

---

## 12. Honest limitations

1. **No PNG screenshots** — use evidence JSON + hard-refresh URLs above.  
2. **Converted** column not mapped — identifier unproven.  
3. **Full Ringba tag bag** deferred.  
4. **Not every historical CH call** received B1 fields (e.g. `RGB87B657F7…` still two pre-B1 rows only) — outside returned lookback set or not in API pages.  
5. **Webhook P-B1.4** proven with a synthetic `sync_path=webhook` row.  
6. First `verify-system` WS failure was **environment pollution** (`GOOS=linux`), not product regression.

---

## 13. Found but not fixed

| Finding | Action |
|---------|--------|
| API accepts **`recordingUrl`** | B2 only — do not store/play yet |
| SearchFields / AvailableGroupBys **301** | Use calllogs probe; no fix |
| `hasConverted` silent omit | Do not map |
| Host GOOS pollution breaks local `go run` smokes | Clear `GOOS`/`GOARCH` after Linux cross-builds |
| `verify-api-sync.ps1` stale | Roadmap H |
| Analytics uniq jumped 931→2673 on lookback | Expected new history, not argMax inflation |

---

## 14. Per-item evidence → change → proof (summary)

| Item | Evidence | Change | Proof |
|------|----------|--------|-------|
| P-B1.1 | Live probe accept/reject + size | Mapping table only | This report §4 |
| P-A | Deep-view already had payout/IDs | Panel + occurred_at | evidence JSON |
| P-B1.2 | CH counts + sync log | Strategy (a) + forced lookback | §6 tables |
| P-B1.3 | — | Adapter/mapper/CH/UI/hash | API duration w/o transcript |
| P-B1.4 | — | `sync_path` + UI copy | webhook + zero-payout JSON |
| P-B1.5 | — | Table only | §9 |
| B2 | — | Roadmap G text | `enterprise-roadmap.md` |
