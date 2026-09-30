# Calyx — Retreaver Stage 4 Report

**Spec:** `docs/retreaver-stage4-spec.md`
**Branch:** `stage4/retreaver`

---

# PHASE R0 — Verify what exists (read-only + live GETs)

**Date:** 2026-09-30
**Operator:** Aryan · **Owner:** Syed
**Code changes:** none to product code. One new **test-only** file (R0.4).

## 0. Scoreboard

| Item | Result |
|---|---|
| R0.0 Environment | **PASS** |
| R0.1 Code inventory | **PASS** |
| R0.2 Data state | **PASS** |
| R0.3 Live API facts | **PASS** — all nine answered against the live account |
| R0.4 K1 reproduction | **PASS** — K1 reproduced `[PROVEN]` |
| R0.5 Proposal | **PASS** — final R1 design below |

### Deviation from the spec, recorded

The spec says branch from `main`. On the operator's instruction this branch was cut from
**`convert-cancel-screen` (`c4e3c5d`)**, not `main` (`ba9c825`).

Reason, and it is a good one: the Docker images running in this environment were built from
`c4e3c5d`. Branching from `main` would have left the checked-out code and the running containers
out of step for every live observation in R0. `c4e3c5d` is itself pushed and awaiting review.

---

## R0.0 — Environment

| | |
|---|---|
| Where | **Local dev machine**, not the staging server. macOS 15.7.4 (24G517), x86_64 |
| Repo | `/Users/macbookpro/Desktop/Calyx/calyx-ai` |
| Branch / HEAD | `stage4/retreaver` @ `c4e3c5d` (parent `ba9c825`) |
| `git status --short` | **1 line** — only the new test file from R0.4. No line-ending noise (`git diff --stat` and `git diff --ignore-cr-at-eol --stat` are both empty) |
| Go | **1.27.1** darwin/amd64 (modules declare `go 1.25.0`; Docker builders `golang:1.25.14-alpine`) |
| Node / npm | **v22.17.0** / 10.9.2 |
| Docker | server **28.5.1** |

### Running services `[PROVEN]`

| Running | Exited (not needed for v1) |
|---|---|
| api, web, ingestion, consumer, platform-sync, postgres, redis, redpanda, clickhouse | ad-attribution, ask-ai, compliance-intelligence, enrichment, scoring, providerconfig |
| | migrate, kafka-init, clickhouse-init (init jobs, exited 0) |

The Chapter-2 services are stopped deliberately: this 16 GB / 8-core machine could not run all 18
containers — the Docker VM saturated at ~358% CPU, the API's Kafka group join timed out, and its
by-design exit turned into a restart loop. Nine containers is stable.

### Verify gates `[PROVEN]`

**`scripts/verify-*.ps1` cannot run here — `pwsh` is not installed**, and the Homebrew cask
(`powershell@preview`; the plain `powershell` cask no longer exists) needs an admin password.

Equivalent checks used instead, all currently green:

| Check | Result |
|---|---|
| `node scripts/check-smoke-call-id-prefix.mjs` | PASS (321 files) |
| `node scripts/check-ws-origin.mjs` | PASS |
| `go build ./...` (platform-sync) | OK |
| `go test ./internal/adapter/` | ok (82.4s) |
| `go test ./internal/syncer/` | ok |
| API Jest | 15/15 |
| Service `/healthz` ×5 + web `/login` | all 200 |

---

## R0.1 — Code inventory

### Adapter — `services/platform-sync/internal/adapter/retreaver_adapter.go`

| Aspect | Finding | Label |
|---|---|---|
| Endpoint | `GET https://api.retreaver.com/api/v2/calls.json` (`:18`), overridable via `RETREAVER_BASE_URL` (`:33-38`) | `[PROVEN]` |
| Cursor | `{updated_at_start, page}` (`:52-56`); legacy `created_at_start` migrated in `normalize()` (`:58-66`) | `[PROVEN]` |
| Window | **start only — no end** (`:144`). This is the defect | `[PROVEN]` |
| Sort | `sort_by=updated_at`, `order=desc` (`:145-147`). Comment at `:146` says docs force desc | `[PROVEN]` code / `[UNKNOWN]` live |
| Paging | `page` + `per_page`, per_page hardcoded **100** in `FetchPage` (`:114`, `:148-149`) | `[PROVEN]` |
| `hasMore` | `len(calls) >= perPage` (`:176`) | `[PROVEN]` |
| Safety rewind | 15 min (`:25`), applied when advancing (`:124`) and from `last_sync_at` (`:107`) | `[PROVEN]` |
| Lookback | default **30 days** (`:28-31`), clamped 1–365 (`:68-77`) | `[PROVEN]` |
| Cursor advance | on last page → `maxCallUpdatedAt(calls, now) − 15m` (`:122-125`) | `[PROVEN]` |

**Fields `mapRetreaverCall` sets:** `ExternalCallID`, `CallerID`, `State` (via `NormalizeState`),
`Revenue`, `Payout`, `CampaignID`, `PublisherID`, `TargetID`, `OccurredAt` (← `created_at`),
`UpdatedAt` (← `updated_at`).

**Fields it does NOT set**, though `models.RemoteCall` and the content hash support them:
`HasConverted`, `DurationSeconds`, `CallLengthSeconds`, `ProfitGross`, `WasConversionAdjusted`,
`WasPayoutAdjusted`, `RequiredDurationSeconds`, `MaxRequiredDurationSeconds`, `HasRecording`,
buyer/target names. `CallKey` is explicitly asserted **empty from the API**
(`retreaver_adapter_test.go:38`) — it only ever arrives by webhook. `[PROVEN]`

Whether the Retreaver API *returns* any of these is `[UNKNOWN]` → R0.3.

### Syncer — `services/platform-sync/internal/syncer/syncer.go`

**Answer per feature. The S1A work is Ringba-only, by an explicit platform guard.**

| S1A feature | Retreaver today | Evidence |
|---|---|---|
| Frozen/windowed incremental | ❌ **No** | Retreaver uses the plain tick → `FetchPage` unbounded window |
| Hourly 7-day sweep | ❌ **No** | `runRingbaSweeps` `:93` — `if !strings.EqualFold(item.Platform,"ringba") { continue }` |
| Daily 30-day deep sweep | ❌ **No** | same guard, `RunAllDeepSweeps` `:80-83` |
| Day-windowed full backfill | ❌ **No** | `RunWindow` `:132-134` — `return fmt.Errorf("RunWindow only supported for ringba")` |
| **Hash after publish (F029)** | ✅ **Yes — already shared** | `publishSyncCalls` `:397`, hash committed only after `pub.Publish` succeeds `:419-427`. Platform-agnostic (takes `platform` as a parameter) |

`RunWindow` is the single entry point for both sweeps and backfill, so one guard removes all three
capabilities from Retreaver at once. That is also the good news: making them shared is one
well-defined change, not three.

### Dedup / content hash — `packages/pingdedup/content.go`

The `Content` struct is rich: revenue, payout, duration, call length, `HasConverted`,
`WasPayoutAdjusted`, `WasConversionAdjusted`, required/max duration, profit. `[PROVEN]`

**But the Retreaver mapper populates only revenue, payout, state, campaign, publisher, target.**
Every other hashed field is the zero value for every Retreaver call. Consequence: a Retreaver call
whose **conversion state or duration** changes produces an **unchanged hash** and is **not
republished**. Today that is invisible because those fields are never mapped — it becomes a live
bug the moment R1 starts mapping them without also confirming they are in the hash. `[INFERRED]`

### Throttling

`RingbaAccountLimiters` (`ratelimit.go:71-90`) — dual-window **5/sec ∧ 80/min** per Ringba
`account_id`. **There is no Retreaver limiter of any kind.** Retreaver requests are unthrottled.
Retreaver's actual limits are `[UNKNOWN]` → R0.3. `[PROVEN]`

### Write-back

Retreaver is blocked at **three independent layers** today — so nothing can reach convert/void by
accident:

| Layer | Guard | Evidence |
|---|---|---|
| API | `throw new BadRequestException('Write-back currently supports Ringba only')` | `integrations.service.ts` `row.platform !== 'ringba'` |
| platform-sync | `return Result{}, ErrNotRingba` | `writeback/convert.go:147-148` |
| UI | `WRITEBACK_PLATFORMS = ['ringba']`, component returns `null` otherwise | `web/src/components/native/ConversionActions.tsx` |

**Generic (reusable for Retreaver):** the `Service` shell, store/audit/decryptor, per-tenant
`writeback_enabled` (migration `000025`), `WRITEBACK_ENABLED` kill switch, Redis idempotency lock,
immutable audit (`000026`), the settle/read-back loop, amount validation.

**Ringba-specific:** the `RingbaAPI` interface (`LookupCall`, `ListConversionWebhooks`,
`FireInboundConversion`, `VoidCall`), the `ringba_response_ref` result field, and the platform
guard. Adding Retreaver means turning `RingbaAPI` into a per-platform interface behind a registry —
the design goes in R3.4.

### UI

Call list shows Retreaver rows normally (platform badge, call id, state, revenue). Call detail
renders the deep view. **The Stage 3 convert/void controls do not appear for a Retreaver call** —
`ConversionActions` returns `null` for any platform outside `WRITEBACK_PLATFORMS`. The spec's R1
requirement to hide a non-working button is therefore **already satisfied**; no UI change is needed
in R1. `[PROVEN]` by code; visual confirmation deferred until real Retreaver rows exist.

⚠️ **One consequence to fix in R1.** The Stage 3 "should have converted" filter treats
`has_converted = 0 AND revenue > 0` as needing review. Because the Retreaver mapper never sets
`HasConverted`, **every Retreaver call with revenue will be flagged "Review"** — a false positive
across the whole platform. Either map the real field (R0.3 must confirm it exists) or scope the
badge to platforms that report conversion state.

---

## R0.2 — Data state

### Postgres `[PROVEN]`

⚠️ **Method note:** `integrations` has RLS. A plain query as `callloom_api` returns **0 rows** and
looks like "no integrations exist". You must
`SET app.current_tenant = '11111111-1111-1111-1111-111111111111';` first. Anyone re-running this
will otherwise reach the wrong conclusion.

| platform | integration_id | name | mode | data source | active | wb | last sync | cursor | creds |
|---|---|---|---|---|---|---|---|---|---|
| callgrid | `45513f17-…1967` | CallGrid Dev Postback | webhook | mock | t | f | never | null | no |
| **retreaver** | **`2d25a9be-24bb-4135-ad59-5ee860c0c259`** | **Retreaver Dev Postback** | **webhook** | mock | t | f | **never** | null | **no** |
| ringba | `9993dc39-…ae15` | Ringba Dev Pixel | webhook | mock | t | f | never | null | no |
| trackdrive | `ab1f70d9-…0c74` | TrackDrive Dev Postback | webhook | mock | t | f | never | null | no |

**There is no Retreaver `api_sync` integration and no stored credentials.** The only Retreaver row
is a seeded dev webhook. This is exactly why R0.3 is blocked.

### ClickHouse `[PROVEN]`

| platform | rows | distinct calls |
|---|---|---|
| ringba | 25 | 25 |
| **retreaver** | **10** | **10** |
| callloom-native | 8 | 8 |
| callgrid | 5 | 5 |
| trackdrive | 5 | 5 |

All 10 Retreaver rows carry the **`demo-`** prefix (fixture), spanning 2026-09-23 → 2026-09-29.
**Zero real Retreaver calls**, confirming `docs/deploy-checklist.md`. No `apisync-`, `smoke-`,
`csp-live-` or `proof-` rows anywhere.

---

## R0.3 — Live API facts (GET only)

**Account confirmed under Rule 7** by the owner: Calyx integration
`53200af1-010b-4377-91b8-1062eebd3ccd` ("My test"), `api_sync`, **Data source = Test**,
Retreaver **company ID ending `5270`**. Credentials decrypted in-process from the stored
integration; never taken from chat, never written to a file.

⚠️ **The account name could not be reported.** `GET /api/v2/companies.json` → **404**, and the call
object exposes `company_id` but no company/account name. Rule 7's "account name" is therefore
`[UNKNOWN]` from the API; the owner confirmed on the company-ID last 4 instead. `[PROVEN]`

**Scale:** the `Total` response header reports **96,747 calls** for a 1-year `updated_at` window on
this account. Backfill is not a small job. `[PROVEN]`

### 🔴 Security incident during this phase — credential exposed in session output

Retreaver's **`Link` response header echoes the entire request query string**, and the request
carries `api_key` and `company_id` as query parameters. My probe printed response headers verbatim,
so **the live Retreaver API key and full company ID were rendered in plaintext in this session's
terminal output.**

| | |
|---|---|
| Reached a repo file? | **No** — `grep` across the repo: 0 files |
| Reached the scratchpad? | **No** — 0 files |
| Committed or pushed? | **No** — nothing staged at the time |
| Where it exists | This session's terminal / transcript only |

**Fix applied:** the probe now runs every printed string through a `redact()` that replaces the
api_key (raw and URL-encoded) and masks the company ID to its last 4.

🙋 **Recommendation to the owner: rotate the Retreaver API key.** The exposure is narrow and
contained, but rotation is cheap and it is the clean response. R1 should also treat any Retreaver
response header as credential-bearing and never log `Link` unredacted.

### The nine questions

| # | Question | Answer | Label |
|---|---|---|---|
| 1 | Is **`updated_at_end`** honoured? | **YES** | `[PROVEN]` |
| 2 | Are `created_at_start` / `created_at_end` honoured? | **YES** | `[PROVEN]` |
| 3 | Which sort orders work? | `updated_at` is **forced desc**; `created_at` honours **asc and desc** | `[PROVEN]` |
| 4 | Maximum `per_page` | **100** — above it the API silently returns **25** | `[PROVEN]` |
| 5 | Timezone | **UTC**, ISO-8601 with milliseconds and `Z` | `[PROVEN]` |
| 6 | Paging shape | `Total` + `Per-Page` + RFC-5988 `Link` headers; body is a bare array with no total | `[PROVEN]` |
| 7 | Rate limits | 429 documented, **no published number**; 3 req/s observed clean | `[PROVEN-docs]` + `[UNKNOWN]` number |
| 8 | Single-call endpoint | **Yes** — `/api/v2/calls/{uuid}.json` | `[PROVEN]` |
| 9 | Conversion + duration fields | **Present and rich** — the adapter ignores them | `[PROVEN]` |

#### Q1 — `updated_at_end` is honoured — this decides the R1 design

Bounded window `[T-14d, T-7d)`:

```
HTTP 200  rows=100
rows inside window = 100   outside = 0   unparsed = 0
returned updated_at range: 2026-09-22T15:22:10Z .. 2026-09-22T23:04:26Z
```

Every row lies inside the requested bound, and the returned range sits inside it. With the same
start and **no** end, the API returns the newest rows instead. **The frozen-window design is
available** — no keyset fallback needed. Corroborated by the official docs, which list
`updated_at_start` / `updated_at_end` as accepted parameters.

#### Q2 — `created_at_*` also honoured

`[T-14d, T-7d)` on `created_at`: HTTP 200, 100 rows, **100 inside / 0 outside**.

#### Q3 — sort behaviour (the adapter's comment is correct)

| `sort_by` | `order` | Actual |
|---|---|---|
| `updated_at` | `desc` | DESC |
| `updated_at` | **`asc`** | **DESC — `asc` ignored** |
| `created_at` | `asc` | **ASC** |
| `created_at` | `desc` | DESC |

So ascending keyset paging on `updated_at` is **not possible**. Ascending is only available on
`created_at`. This is why Q1 mattered so much — with `updated_at_end` honoured we do not need it.

#### Q4 — `per_page` max is 100, and exceeding it fails silently

```
per_page=100  -> rows=100   bytes=448429
per_page=250  -> rows=25    bytes=97559
per_page=500  -> rows=25    bytes=97559
per_page=1000 -> rows=25    bytes=97559
```

Above 100 the API does not error — it **drops to the default 25**. A future "let's raise the page
size" change would silently quadruple the request count instead. Worth a comment in the adapter.

#### Q5 — timestamps are UTC

```
created_at = "2026-09-29T23:03:07.464Z"
updated_at = "2026-09-29T23:03:15.679Z"
start_time = "2026-09-29T23:03:12.355Z"
```

ISO-8601, millisecond precision, `Z` suffix. R2's day-by-day reconciliation can use UTC days
directly with no account-timezone conversion.

#### Q6 — paging metadata is in headers, not the body

```
Total:     96747
Per-Page:  <n>
Link:      <…page=48374…>; rel="last", <…page=2…>; rel="next"
```

`Total` gives an exact expected count — **directly useful as the reconciliation denominator in R2**,
and as a cheap gap check after each window. The body is a bare JSON array with no envelope.

⚠️ `Link` contains the api_key — see the security incident above. Never log it unredacted.

#### Q7 — rate limits: documented but unquantified

Official docs (`https://retreaver.github.io/core-api-docs/#introduction`):

> **429 Too Many Requests – Slow down. Highly concurrent requests are frowned upon.**

No numeric limit is published, and no `X-RateLimit-*` headers are returned. Full header set:
`Cache-Control, Content-Type, Date, Etag, Link, Per-Page, Referrer-Policy, Server, Status,
Strict-Transport-Security, Total, X-Content-Type-Options, X-Frame-Options,
X-Permitted-Cross-Domain-Policies, X-Powered-By, X-Request-Id, X-Runtime, X-Xss-Protection`.

Observed: **20 sequential GETs in 6.78 s (~3.0 req/s), all HTTP 200, zero 429s.** `[PROVEN]`

The published guidance is about **concurrency**, not rate — which argues for serialising Retreaver
requests per account rather than parallelising them.

#### Q8 — single-call endpoint exists

```
GET /api/v2/calls/{uuid}.json -> HTTP 200
shape: OBJECT { "call": { … } }      uuid matches request: true      fields: 72
converted=true  connected=true  status=finished  total_duration=940
```

This is the **A1 proof endpoint** for certification, and the natural **read-back endpoint** for
write-back in R3.

Note it returns **72** fields versus **71** on the list endpoint — the shapes are not identical, so
R2 should reconcile against the list endpoint it actually syncs from.

#### Q9 — the conversion and duration data exists, and we throw it away

A real call object carries **71 fields**. The adapter maps **9**.

Present and unmapped, directly relevant to Stage 4:

| Purpose | Fields |
|---|---|
| **Conversion** | **`converted`** (bool), `conversion_seconds`, `conversions_determined_at`, `payout`, `revenue` |
| **Duration** | `total_duration`, `dialed_call_duration`, `billable_minutes`, `hold_duration`, `ivr_duration`, `time_to_connect_in_seconds`, `time_to_call_in_seconds`, `connected` |
| Money | `profit_gross`, `profit_net`, `charge_total`, `total_cost`, `receivable`, `payable` |
| Duplicates | `duplicate`, `payable_duplicate`, `receivable_duplicate`, `repeat`, `number_repeat`, `affiliate_repeat` |
| Naming | `target_name`, `affiliate_name`, `target_group`, `status` |
| Pixels | `fired_pixels`, `fired_pixels_count`, `postback_value` |

Sample rows (allowlisted values only):

```
converted=false connected=true status=forwarded revenue=null payout=null
  total_duration=null  conversions_determined_at=null      duplicate=false profit_gross=0
converted=false connected=true status=finished  revenue=null payout=null
  total_duration=14 dialed_call_duration=10 billable_minutes=1
  conversions_determined_at=2026-09-29T23:00:26.314Z       duplicate=false profit_gross=-0.05
```

⚠️ **`revenue` and `payout` can be `null`**, not `0`. R1 must distinguish "no value reported" from
"zero", or a null will be written as a real 0.00 and look like a genuine zero-revenue call.

#### Live proof that the window really does move

Call `a31df5bd…` read **`converted=false, status=forwarded`** in the list query, then
**`converted=true, status=finished, total_duration=940`** from the single-call endpoint minutes
later. The dataset changed underneath the probe during R0 itself — which is precisely the condition
that makes the current unbounded desc paging lose calls. `[PROVEN]`

## R0.4 — K1 reproduction `[PROVEN]`

**New test-only file:** `services/platform-sync/internal/adapter/retreaver_k1_paging_test.go`.
No product code touched.

### The mechanism

The request sends `updated_at_start` but **no `updated_at_end`**, sorted `updated_at desc`. The
result set is therefore live. When a call's `updated_at` changes during the page walk it jumps to
position 1 and pushes every later row down one place. A row due at the top of page 2 slides to the
bottom of page 1 — already fetched — and is never returned.

### The test

101 calls, `per_page=100` (the adapter's hardcoded size). Page 1 returns the newest 100. Between
page 1 and page 2 the oldest call `C101` is updated, jumping it to the front. Page 2 asks for rows
101+ of the **new** ordering and returns `C100` again.

```
K1 REPRODUCED [PROVEN]: C101 never returned. rows=101 distinct=100 duplicated_ids=1
root cause [PROVEN]: updated_at_start="2026-08-30T22:42:16Z", updated_at_end absent,
                     order="desc", per_page="100"
```

101 rows fetched, only **100 distinct**, one duplicated, and **one call permanently lost**. It is
not late — nothing will fetch it again until something else changes it.

This is the same failure that silently skipped **431 Ringba calls** before S1A.

`TestRetreaverWindowHasNoEndBound` pins the root cause and will fail the moment R1 starts sending
`updated_at_end`, forcing the test to be updated deliberately rather than silently.

### No regression

`go build ./...` OK · `go test ./internal/adapter/` **ok (82.4s)** · Ringba-only subset **ok** ·
`go test ./internal/syncer/` **ok**. Per Rule 8 no Ringba integration was made Live; Ringba is
proven by unit tests only, and this phase changed no Ringba code.

---

## R0.5 — Final R1 design

R0.3 settled the open question: **`updated_at_end` is honoured**, so the frozen-window design is
available and the ascending-keyset fallback is not needed. (It could not have been used anyway —
Q3 proves `order=asc` is ignored on `updated_at`.)

### Retreaver's problem is narrower than Ringba's

Ringba has no "updated since" filter at all, which is why it needed day-slicing, binary-halving and
two sweep tiers. Retreaver has `updated_at_start`/`_end` and bumps `updated_at` on revenue change.
Copying Ringba wholesale would be cargo-culting. Three Ringba bugs Retreaver does **not** have:

| Ringba bug | Retreaver | Why |
|---|---|---|
| Downtime gap (F003b) | Not present | The cursor is a watermark advanced only on success, not a stored absolute window (`retreaver_adapter.go:118-125`). `[INFERRED]` — R1 adds a test |
| Long calls missed | Not present | Ringba filters on call **start**; Retreaver filters on `updated_at`, which moves when the call ends |
| Late updates missed | Not present | Same reason — confirmed by Q9's `conversions_determined_at` moving after the call |

**The one real defect is K1: the unbounded, desc-sorted, offset-paged window.**

### R1.1 — Frozen window (fixes K1)

Send **both** bounds. Capture `end = now` once when the window opens and carry `[start, end)` in the
cursor so every page of the walk queries the same frozen set.

```
window_start = last_successful_end − overlap(15m)
window_end   = now                         # fixed for the whole page walk
GET calls.json?updated_at_start=…&updated_at_end=…&sort_by=updated_at&order=desc
                &page=N&per_page=100
```

A row that updates mid-walk moves *out* of the frozen window and is collected by the next window,
whose start is `end − 15m`. Nothing is skipped; at worst a row is fetched twice, and the content
hash suppresses the duplicate.

- **Cursor** becomes `{window_start, window_end, page}`. Old `{updated_at_start, page}` cursors
  migrate by treating `updated_at_start` as `window_start` and computing a fresh `window_end`.
- **Page cap** so a pathological window cannot walk forever; log and carry the remainder into the
  next window.
- **`per_page` stays 100** with a comment recording that >100 silently degrades to 25 (Q4).
- **Gap check, free:** compare the `Total` header against rows collected for the window and log a
  warning on mismatch (Q6).

### R1.2 — Map conversion + duration, **and put them in the content hash** *(Syed item 1)*

Map from Q9: `converted` → `HasConverted`, `total_duration` → `CallLengthSeconds`,
`dialed_call_duration` → `DurationSeconds` (connected talk time), plus `conversion_seconds`,
`conversions_determined_at`, `profit_gross`, `duplicate` → `PlatformDuplicate`, `target_name`,
`affiliate_name`, `status`, `connected`.

**Mapping alone is not enough — and this is the trap.** `packages/pingdedup/content.go` already
hashes `HasConverted`, `DurationSeconds`, `CallLengthSeconds` and `ProfitGross`, but the Retreaver
mapper leaves them zero today, so the hash never moves for them. Map them **and** verify they are
in `remoteCallContent`, otherwise a call that converts with no revenue change produces an unchanged
hash and is **never republished** — a silent miss created by the fix itself.

**R1 must add a test:** same call, only `converted` flips false→true, assert the content hash
changes and the row republishes.

⚠️ **`revenue`/`payout` can be `null` (Q9).** Map null as "not reported", not `0.00`.

### R1.3 — Per-account Retreaver throttle *(Syed item 2)*

No published number exists (Q7). The documented guidance is
*"Highly concurrent requests are frowned upon"* —
`https://retreaver.github.io/core-api-docs/#introduction`. Observed: 3.0 req/s clean over 20
requests.

**Conservative documented default**, reusing Ringba's `DualWindowLimiter` keyed on `company_id`:

| Setting | Default | Reasoning |
|---|---|---|
| `RETREAVER_RATE_PER_SEC` | **2** | Below the 3 req/s observed clean, leaving headroom for the owner's own usage on the same account |
| `RETREAVER_RATE_PER_MIN` | **60** | Matches 1 req/s sustained; a 96,747-call backfill at 100/page is ~968 requests ≈ 16 min, acceptable |
| Concurrency | **1 (serialised) per `company_id`** | The docs' only concrete guidance is about concurrency, not rate |

Also handle **429 explicitly** with backoff honouring `Retry-After` — the status is documented even
though no limit is. The numbers are a defensible default, not a proven limit; revisit if Retreaver
publishes one or a 429 is ever observed.

### R1.4 — Share sweeps and backfill *(Syed item 3)*

Remove the Ringba-only guard at `syncer.go:132-134`
(`return fmt.Errorf("RunWindow only supported for ringba")`) and the `continue` at `:93`.

Smallest change that makes it shared, per Rule 4: give `PlatformAdapter` a capability method
(e.g. `SupportsWindow() bool`, or a `WindowedAdapter` interface the registry type-asserts), and have
`RunWindow` / `runRingbaSweeps` consult that instead of a hardcoded string. Ringba returns true;
Retreaver returns true once R1.1 lands; TrackDrive/CallGrid return false and are unaffected.

Sweep tiers mirror Ringba, with Retreaver-specific env names and the same defaults
(hourly/7 days, daily/30 days). They matter less for Retreaver — its incremental window already
catches `updated_at` changes — so they are the safety net for anything that changes **without**
bumping `updated_at`. `conversions_determined_at` (Q9) suggests conversions do bump it; the deep
sweep covers the case where they do not.

Day-windowed full backfill then works for Retreaver through the same `RunWindow` path.

### R1.5 — Fix the Stage 3 "should have converted" false positive *(Syed item 4)*

Today the predicate is `has_converted = 0 AND (revenue > 0 OR duration ≥ required_duration)`.
Because the Retreaver mapper never sets `HasConverted`, **every Retreaver call with revenue is
flagged "Review"** — a false positive across the whole platform.

R1.2 largely fixes this by mapping the real `converted` value. But two gaps remain:

1. Retreaver has **no `required_duration_seconds`** equivalent in the 71 fields, so the duration
   half of the predicate can never fire for Retreaver — it degrades to `revenue > 0 AND !converted`.
   Acceptable, but should be stated in the UI tooltip rather than implied.
2. A platform that reports **no** conversion state at all (TrackDrive, CallGrid today) would still
   show false positives. The predicate needs a notion of "this platform reports conversion state" —
   otherwise `has_converted = 0` means "not converted" for Ringba and "we never asked" for others.

Proposal: add a `reports_conversion_state` capability per adapter; the badge and the
`should_convert` filter only apply to platforms where it is true. Both the API predicate
(`pings.filters.ts`) and the shared UI helper (`shouldHaveConverted` in `web/src/lib/api.ts`) must
change together — they were deliberately built to share one definition.

### R1.6 — Not needed

- **Hash-after-publish (F029)** — already shared and already applies to Retreaver.
- **Hiding the convert button for Retreaver** — already the case at three layers.

### Settings introduced by R1 (all documented with defaults)

| Setting | Default |
|---|---|
| `RETREAVER_WINDOW_OVERLAP_MINUTES` | 15 (reuses the existing safety delta) |
| `RETREAVER_SWEEP_DAYS` | 7 |
| `RETREAVER_SWEEP_INTERVAL` | 60m |
| `RETREAVER_DEEP_SWEEP_DAYS` | 30 |
| `RETREAVER_DEEP_SWEEP_INTERVAL` | 24h |
| `RETREAVER_RATE_PER_SEC` | 2 |
| `RETREAVER_RATE_PER_MIN` | 60 |
| `RETREAVER_MAX_PAGES_PER_WINDOW` | 200 |

### Open question for R1

**Dedup TTL vs the 30-day sweep.** `DEDUP_TTL_HOURS` is 240h (10 days) but the deep sweep re-reads
30 days. An unchanged call older than the TTL has no hash, so the sweep will republish it. The spec
requires proving ClickHouse still holds exactly one logical row and the dashboard counts it once.
`argMax` should make this safe, but it is unproven for Retreaver and is an explicit R1 proof item.

## 1. Changes

| File | Type |
|---|---|
| `docs/retreaver-stage4-spec.md` | spec, brought onto the branch |
| `docs/retreaver-stage4-report.md` | this report |
| `services/platform-sync/internal/adapter/retreaver_k1_paging_test.go` | **new, test-only** |

No product code changed in R0.

Commit hash: recorded below after commit.

## 2. Server deploy note

**Nothing.** R0 changed no product code, no env variable, no migration. The new file is a Go test
and is not part of any image's runtime.

## 3. Open questions for the owner 🙋

1. 🔴 **Rotate the Retreaver API key.** It was rendered in plaintext in this session's terminal
   because Retreaver's `Link` header echoes the request query string and my probe printed headers
   verbatim. It reached **no file, no commit, no push** (verified by grep across repo and
   scratchpad). The probe is fixed. Rotation is cheap and is the clean response.
2. **Rule 7 — account name is unavailable.** `companies.json` 404s and the call object has no
   company name. You confirmed on company ID ending **`5270`**; recorded as the identification
   method for this stage.
3. **Rule 10** — before R2 makes this integration Live, confirm no other environment (your laptop,
   the staging server) is live-syncing the same Retreaver account. **96,747 calls** means the
   backfill is substantial.
4. **Throttle numbers need your sign-off.** Retreaver publishes no limit, so R1.3's 2/sec · 60/min
   serialised is a conservative default I chose, not a proven ceiling. If you know the real limit
   or want it more cautious, say so before R1.
5. **`pwsh`** — the three PowerShell gates still cannot run here. `brew install --cask
   powershell@preview` needs your admin password. Until then R1/R2 use the R0.0 substitutes.
6. **Branch base** — confirm you accept that this branch was cut from `convert-cancel-screen`
   (`c4e3c5d`) rather than `main`, so the code matches the running containers.

## 4. Honest limitations

1. **The K1 fix is not built.** R0 proves the bug and designs the fix; nothing is repaired yet.
2. **`[INFERRED]`, not proven:** that Retreaver escapes the downtime-gap and long-call bugs. Both
   come from reading the cursor logic, not from a test. R1 should add tests rather than trust it.
3. **The throttle default is a judgement call**, not a measured ceiling. 3 req/s was observed clean
   over only 20 requests — that is not a sustained-load test.
4. **`Total` = 96,747 is for a 1-year `updated_at` window**, not necessarily the account's whole
   history. Full-history scale is still `[UNKNOWN]`.
5. **Sample size for Q9 was 3 calls.** `revenue`/`payout` were null on all three and `converted`
   false on all three; the single-call probe found one with `converted=true`. Field *presence* is
   proven; the distribution of values is not.
6. **The list endpoint returns 71 fields, the single-call endpoint 72.** The extra field was not
   identified. R2 should reconcile against the list endpoint it actually syncs from.
7. **Local dev machine, not staging.** Chapter-2 services are stopped here.
8. **The three PowerShell gates were not run** — substitutes listed in R0.0.
9. **No Retreaver data in ClickHouse**, so no UI observation of real Retreaver rows was possible.

---

**Gate R0 — accepted by the owner.** See `docs/retreaver-r1-amendments.md`.


---

# PHASE R1 — Paging fix (K1) + sync parity  ·  **GATE R1 MET** (one item partial)

**Date:** 2026-09-30 · **Branch:** `stage4/retreaver` · **Amendments:** `docs/retreaver-r1-amendments.md`

> **Status: all R1 code items are complete and proven, and all three PowerShell gates pass.**
> `pwsh` was installed (A5) and the gates were run — they needed six fixes first, every one of
> them a Windows-only assumption in the scripts, none in the product. C3 was resolved **without**
> a migration, by evidence rather than by inventing one. **C2 remains partial**: a short walk is
> refused, but bounded retry and a degraded sync status are not built.

## 1. Scoreboard

| Item | Result |
|---|---|
| **B1** transcript scan | **DONE** — key **IS** present on disk |
| **B2(1)** header auth | **DONE** — `[PROVEN]` impossible |
| **B2(2)** shared redaction | **DONE — found and fixed a real leak in product code** |
| **B2(3)** forced-transport-error test | **DONE** — 4 tests |
| **B2(4)** scan stored data | **DONE** — **CLEAN** |
| **B3** R0 commit hash | **N/A** — R0 was never committed (see §6) |
| **C1** frozen window + split, no truncation | **DONE** — K1 proven fixed |
| **C2** `Total` mismatch blocks | **MOSTLY** — windowed path refuses to advance on a short walk; retry/degraded-status not built |
| **C3** `null` revenue/payout | **RESOLVED — no migration needed** (100-call proof) |
| **C4** field-meaning table | **DONE** — §7 |
| **C5** hash test per mapped field | **DONE** — both directions |
| **C6** `reports_conversion_state` | **DONE** — Ringba counts identical, 18 false positives removed |
| **C7** downtime + long-call tests | **DONE** — plus a crash-resume test |
| **C8** stronger account check | **READY** — needs a UUID from the owner (§5) |
| **C9** per-account throttle | **DONE** — implemented + tested |
| **C10** Go 1.25 parity | **DONE** — exit 0 in `golang:1.25.14-alpine` |
| **D** field mapping | **DONE** — 9 fields → 18 |
| **D** capability method (share sweeps/backfill) | **DONE** — `RangeFetcher`, both guards removed |
| **E** three PowerShell gates | **PASS** — all three, after six script portability fixes (§13) |

## 2. B — security

### B1 — the key IS on disk. My R0 claim was wrong. `[PROVEN]`

Counts only, never the match:

```
~/.claude/projects — 267 files scanned
  api_key     : 1 file  (3 lines)
  company_id  : 2 files (4 lines)
```

R0 said "terminal only". That was inaccurate — the grep covered the repo and scratchpad but not the
transcripts. The owner has decided against rotation (A1), so per B1 these files stay private to this
machine: not shared, not uploaded, not attached to any report.

### B2(4) — nothing leaked through normal operation `[PROVEN]`

Postgres `last_sync_error` / `sync_status` / `sync_cursor` across all integrations, plus current
`platform-sync` and `api` container logs:

```
api_key     matching lines: 0
company_id  matching lines: 0
```

### B2(1) — the key cannot leave the URL `[PROVEN]`

Docs: *"Retreaver expects for the API key to be included in all API requests to the server in a
query string parameter"* — https://retreaver.github.io/core-api-docs/#authentication

Tested live anyway, GET only:

| Attempt | Result |
|---|---|
| `?api_key=` (documented) | **HTTP 200** |
| `Authorization: Bearer <key>` | HTTP 401 |
| `Authorization: Token <key>` | HTTP 401 |
| `X-Api-Key: <key>` | HTTP 401 |
| `Api-Key: <key>` | HTTP 401 |

So redaction is the only defence — exactly why the owner made B2 mandatory once rotation was ruled out.

### B2(2) — a real leak in product code, found and fixed 🔴 `[PROVEN]`

Writing the B2(3) test surfaced a genuine vulnerability that was **not** my probe:

`formatHTTPStatusError` (`internal/adapter/httpclient.go`) interpolated the upstream **response
body** raw:

```go
return fmt.Sprintf("%s %d: headers={%s} body=%s", prefix, status, headers, bodyStr)
```

Transport errors were already redacted via `sanitizeHTTPError` → `redact.SanitizeErr`, and `Link`
was already excluded from the safe-header allowlist. But the **status-error body path was not
redacted at all**. Any Retreaver 4xx/5xx whose body echoes the request URL would have written
`api_key=…` into the error — and that error is stored as the integration's `last_sync_error`.

**Fix:** the body now passes through `redact.RedactMessage`. This is shared code, so it protects
every adapter, not just Retreaver.

**Before the fix** the test failed with `status error leaked the api_key`. After, it passes.

### B2(3) — four redaction tests `[PROVEN]`

`internal/adapter/retreaver_redaction_test.go`, using a synthetic canary key:

| Test | Covers |
|---|---|
| `TestRetreaverTransportErrorDoesNotLeakCredentials` | unresolvable host → `*url.Error` carries the URL |
| `TestRetreaverStatusErrorDoesNotLeakLinkHeader` | 4xx whose `Link` header and body echo the query string |
| `TestSafeResponseHeadersExcludeLink` | pins `Link` out of the loggable allowlist |
| `TestRetreaverAuthErrorIsClassifiedWithoutCredentials` | the `ClassifySyncError` path that reaches `last_sync_error` |

Observed transport error after the fix:

```
Get "https://…/api/v2/calls.json?api_key=[REDACTED]&company_id=[REDACTED]&order=desc&page=1…"
  dial tcp: lookup …: no such host
```

## 3. C1 — the frozen window, and a correction to my own R0 design

### What was built

`internal/adapter/retreaver_window.go` (new) + a rewritten `FetchPage` / `fetchWindowPage`:

- Every walk runs inside `[window_start, window_end)`, the end captured once when the window opens.
- The cursor is now `{window_start, window_end, page}`; a crash resumes the **same** window.
- Both legacy cursor shapes migrate into it without losing position (two tests).
- `updated_at_end` is now sent. `per_page` stays 100, with a comment recording that above 100 the
  API silently returns 25.
- The `Total` header drives `hasMore` when present, and `Link` is deliberately never read.

### C1's split rule — the owner caught a bug in my R0 design

R0.5 said a page-cap overflow should "carry the remainder into the next window". The owner's C1
notes that is **K1 again**: with `order=desc` the leftover rows are the *oldest* in the window, and
the next window starts at `end − overlap`, so they would never be fetched. He was right.

Implemented instead: on page 1, if `Total > per_page × MAX_PAGES`, **shrink the window end**
(halve, recursively, down to a 1-minute floor) and re-fetch page 1. Shrinking the end is safe and
self-healing — the dropped span is simply not covered yet, and the next window starts exactly where
this one ended. Nothing is ever cut off.

### A second correction, found by the test itself

My first K1 test moved the victim row to a *past* timestamp still inside the window, and it was
**still skipped** — because freezing the window bounds does not freeze the ordering *within* the
window.

That scenario cannot occur in reality: `updated_at` is a last-modified stamp, so an update during a
walk sets it to **now**, which is at or past the frozen end. The row therefore leaves the window and
the next window collects it. The test now models that correctly, and the assertion that matters is
not "window 1 returned everything" but **"nothing is lost across consecutive windows"**.

```
K1 FIXED [PROVEN]: all 101 calls delivered across 2 windows
  (w1=100 rows, w2=1 row); C101 was updated mid-walk and collected by window 2
```

### C9 — throttle `[PROVEN]`

`RetreaverAccountLimiters`, keyed on `company_id`, reusing Ringba's `DualWindowLimiter`.
Defaults **2/sec · 60/min** (owner-approved, A4) — a conservative choice, not a measured ceiling.
Tests prove same `company_id` shares one limiter, different ones do not, an empty id is still
bounded, the env overrides work, and the limiter actually delays.

### C10 — Go parity `[PROVEN]`

`docker run --rm golang:1.25.14-alpine go test ./...` → **exit 0**, adapter suite ok (85.9s).
Matches the host run on Go 1.27.1.

### Rule 8 — Ringba no regression `[PROVEN]`

`go test ./...` across platform-sync: adapter **ok (87.8s)**, syncer ok, writeback ok, redact ok,
state ok. `packages/pingdedup` ok. No Ringba integration was made Live; Ringba is proven by unit
tests only, per Rule 8.

## 4. C3 — 🙋 BLOCKED, as the amendments require

```
revenue  Decimal(18, 4)   ← NOT nullable
payout   Decimal(18, 4)   ← NOT nullable
```

R0.3 Q9 proved Retreaver returns `revenue` and `payout` as **`null`**, not `0`. The current schema
cannot distinguish "not reported" from "zero", so mapping them today writes a silent `0.00`.

C3 says: stop and propose, do not invent a migration inside R1. **Proposed** (for the owner to
approve, not built): make `revenue` / `payout` `Nullable(Decimal(18,4))`. Precedent exists in the
same table — `required_duration_seconds` is `Nullable(UInt32)` and `was_payout_adjusted` is
`Nullable(UInt8)`. The content hash must then distinguish null from 0, and every `sum()` must
decide explicitly whether null counts as 0.

**This blocks the revenue/payout half of the D mapping.** The non-money fields
(`converted`, durations, `duplicate`, names, `status`) are not blocked.

## 5. Open questions for the owner 🙋

1. **C3 migration** — approve `Nullable(Decimal)` for `revenue`/`payout`, or state that a silent 0
   is acceptable. D mapping cannot finish either way until this is decided.
2. **C8 account check** — ready to run: give one recent call UUID from the Retreaver web UI and I
   will fetch it via `/api/v2/calls/{uuid}.json` and report `status`, `created_at`, `revenue` for
   you to compare.
3. **`pwsh`** — still not installed; the three gates cannot run, so Gate R1 cannot be met.
4. **B1** — the key is in 1 transcript file on this machine. With rotation ruled out, confirm you
   are content with "keep it private to this machine" as the mitigation.

## 6. Honest limitations

1. **Gate R1 is not met.** C2 partial; C3 blocked; C4, C5, C6, C7 and both D items not started;
   PowerShell gates not run.
2. **No live Retreaver sync was run.** Everything is unit-tested against a fake API plus GET-only
   probes. The integration stays on **Test**.
3. **B3 cannot be satisfied as written** — R0 was never committed (the operator instructed no
   commits at that point), so there is no R0 hash. R0 and R1 will land in one commit.
4. **C2 is only half true today.** `Total` is used to drive paging, but a mismatch does not yet
   retry, hold the cursor, or mark the integration degraded.
5. **The K1 fix is proven against a fake server** that reproduces the behaviours R0.3 proved live
   (`updated_at_end` honoured, desc-only, `Total` header). It has not met the real API.
6. **Window splitting is unit-tested arithmetically**, not against a real oversized window.


---

## 7. C4 — field-meaning table

The dashboard shows Ringba and Retreaver side by side, so a column must mean the same thing for
both. Retreaver returns 71 fields; the adapter mapped 9 before R1 and maps 18 now.

| Calyx field | Ringba source | Retreaver source | Meaning | Note |
|---|---|---|---|---|
| `Revenue` | `conversionAmount` → `revenue` | `revenue` | what the buyer pays | Retreaver sends `null` when unconverted — see C3 |
| `Payout` | `payoutAmount` → `payout` | `payout` | what the publisher is paid | same null behaviour |
| `ProfitGross` | `profitGross` | `profit_gross` | gross margin | Retreaver always populates it, even unconverted (negative = telco cost) |
| `HasConverted` | `hasConverted` | **`converted`** | platform says the call converted | both booleans; Retreaver always present |
| `HasConvertedPresent` | presence of `hasConverted` | presence of `converted` | distinguishes "false" from "never asked" | works around `UInt8 DEFAULT 0` (F038/K8) |
| `HasConnected` | `hasConnected` | `connected` | the call actually connected | |
| **`DurationSeconds`** | connected duration | **`dialed_call_duration`** | **connected talk time** | ⚠️ the close call — see below |
| **`CallLengthSeconds`** | call duration | **`total_duration`** | **whole call incl. IVR/hold** | |
| `PlatformDuplicate` | `isDuplicate` | **`duplicate`** | platform says the *lead* is a repeat | rule (b): out of revenue, in call counts |
| `TargetName` | target name | `target_name` | the buyer | |
| `PublisherName` | publisher name | **`affiliate_name`** | the traffic source | Retreaver says "affiliate" for the same concept |
| `EventType` | event | `status` | call outcome | Retreaver: `finished`, `forwarded`, … |
| `OccurredAt` | `callDt` | `created_at` | when the call happened | |
| `UpdatedAt` | n/a | `updated_at` | last modification | Ringba has no last-modified — the reason it needed day-slicing |

### The one genuinely approximate mapping

**`DurationSeconds` (connected talk time).** Retreaver offers `total_duration`,
`dialed_call_duration`, `billable_minutes`, `hold_duration`, `ivr_duration` and
`time_to_connect_in_seconds`. Ringba's "connected duration" is time on the phone with the buyer.

`dialed_call_duration` is the closest: it measures the dialed leg, i.e. buyer-connected time,
whereas `total_duration` includes IVR and hold before the buyer answered. In the R0.3 sample
`total_duration=14` with `dialed_call_duration=10`, and `total_duration=30` with
`dialed_call_duration=30` — consistent with total ≥ dialed.

Labelled `[INFERRED]`: it is a reasoned choice from field semantics and observed values, **not**
confirmed against Retreaver documentation. If R2's reconciliation shows duration drift against the
Retreaver UI, this is the first mapping to re-examine.

### Deliberately not mapped

`CallKey` (never returned by the API — webhook only, asserted by an existing test),
`BuyerID`/`BuyerName` (Retreaver models the buyer as the target),
`HasRecording`, `WasConversionAdjusted`, `WasPayoutAdjusted` (no Retreaver equivalent found),
`RequiredDurationSeconds` (**no Retreaver equivalent exists** — so the duration half of the
"should have converted" rule can never fire for Retreaver; it degrades to `revenue > 0 AND
!converted`, which is correct but narrower than for Ringba).

## 8. C3 resolved — by evidence, not by a migration

R0 reported `revenue`/`payout` as `null` from a **3-call** sample and concluded a
`Nullable(Decimal)` migration was required. The operator pushed back — the UI was showing revenue.

He was right to push, and the 3-call sample was too small. A 100-call sample settles it:

| field | null | zero | non-zero |
|---|---|---|---|
| `revenue` | 60 | **0** | 40 |
| `payout` | 60 | **0** | 40 |
| `converted` | — | 60 false | 40 true |

The correlation is exact: **`revenue = null` ⟺ `converted = false`**, and never once `0`.

So "not reported" and "zero" are not ambiguous for Retreaver once `converted` is mapped beside the
amount. Mapping `null → 0.00` alongside `converted` loses nothing:

| `converted` | stored `revenue` | meaning |
|---|---|---|
| false | 0.00 | not converted, no money — correct |
| true | 37.50 | converted, real money |

**No migration is needed.** Labelled `[PROVEN on a 100-call sample]` — a strong empirical
correlation, not a documented API guarantee. The only case it could not express is "converted for
exactly $0", which does not occur in the sample. 🙋 The owner can overrule this and take the
migration if he wants the schema to be unambiguous by construction.

*(The revenue visible in the Ping Feed during this discussion was **mock**: 43 `apisync-retreaver-*`
rows, all exactly $11.25 — one distinct value — generated by our own Test-mode adapter. There are
still **zero real Retreaver rows** in ClickHouse.)*

## 9. C6 proof — Ringba unchanged, false positives gone

`should_convert` counts, `show_demo=true`, before and after:

| platform | before | after | |
|---|---|---|---|
| ringba | 25 | **25** | ✅ identical, as C6 requires |
| retreaver | 57 | **57** | ✅ identical |
| trackdrive | 5 | **0** | ✅ no longer flagged |
| callgrid | 5 | **0** | ✅ no longer flagged |
| callloom-native | 8 | **0** | ✅ no longer flagged |
| **all** | 100 | **82** | 18 false positives removed |

One source of truth: `services/api/src/common/platform-capabilities.ts`. The API stamps
`conversion_reported` onto every ping row, and the web reads that field — the UI holds no platform
list of its own, so badge and filter cannot drift. Rows from non-reporting platforms render `n/a`
rather than a truthful-looking "Review".

## 10. D — sweeps and backfill are now shared

`RunWindow` and the sweeps were gated on `strings.EqualFold(item.Platform, "ringba")`, and
`Registry.FetchRange` type-asserted `*RingbaAdapter`. Both are replaced by a capability:

```go
type RangeFetcher interface {
    FetchRange(ctx, creds, start, end) (PageResult, error)
}
```

Ringba and Retreaver implement it; TrackDrive and CallGrid do not and keep the plain incremental
path. Sweep tiers are per-platform env vars so the two vendors' very different rate budgets can be
tuned independently.

## 11. Tests added in R1

| File | Covers |
|---|---|
| `retreaver_redaction_test.go` | B2(3) — 4 tests; found the real leak |
| `retreaver_k1_paging_test.go` | K1 fixed across two windows; window is bounded |
| `retreaver_window_test.go` | C1 splitting, window construction, C9 throttle (6 tests) |
| `retreaver_mapping_test.go` | D mapping, C3 null semantics, C5 hash both directions |
| `retreaver_resilience_test.go` | C7 downtime, C7 long call, crash resume |

**Verification:** platform-sync `go test ./...` **all ok** · `packages/pingdedup` ok ·
`golang:1.25.14-alpine` **exit 0** (C10) · `check-smoke-call-id-prefix` PASS (328 files) ·
`check-ws-origin` PASS · API `tsc --noEmit` clean · secret scan of every changed file clean.

## 12. What is still not done

1. **The three PowerShell gates — `pwsh` is not installed.** This alone means Gate R1 is not met.
2. **C2 is partial.** `FetchRange` refuses to advance when a window returns fewer rows than
   `Total`, and `Total` drives `hasMore` on the incremental path. Not built: bounded retry with
   backoff, and marking the integration's sync status *degraded* on repeated mismatch.
3. **No live Retreaver sync.** Everything is unit-tested against a fake API that reproduces the
   behaviours R0.3 proved live. The integration stays on **Test**; ClickHouse still holds zero real
   Retreaver rows. R2 is where this meets reality.
4. **`DurationSeconds` is `[INFERRED]`** — see §7.
5. **Window splitting is unit-tested arithmetically**, never against a real oversized window.
6. **C8 not run** — needs a call UUID from the owner.


---

## 13. Gate E — the three PowerShell gates

```
SYSTEM VERIFY PASS            Docker 15/15 · Healthz 11/11 · LiveStatus PASS · API 20/20
                              Web routes 13/13 · WebSocket PASS · SmokePrefix PASS
                              BrowserCSP PASS · Unit tests PASS · Web build PASS · Pipeline 4/4
INGESTION SLICE VERIFY PASS
PRESENTATION VERIFY PASS      20/20
```

`pwsh` installed as **PowerShell 7.7.0-preview.5**. Note the binary is **`pwsh-preview`**, not
`pwsh` — the plain `powershell` cask no longer exists, and the preview cask does not create a
`pwsh` symlink. `Makefile` targets that call `powershell`/`pwsh` will not find it on macOS
without an alias.

### Worth recording: the previously-failing WebSocket check now passes

`s1a-followup-report.md` item 4 recorded `verify-system` = **FAIL (WS)**. It now reports
`LiveStatus: PASS` with `group=calyx-api-live-feed is_group_member=True status=ok
recreate_count=0`, and `WebSocket: PASS`. Not fixed by this phase — recorded as an observation
`[PROVEN]`, since that FAIL was the outstanding blemish on the "checks pass" claim.

### Six script fixes were required — all Windows-only assumptions

None of these are product changes. Rule: *adapt the scripts, never weaken the product.*

| Script | Problem | Fix |
|---|---|---|
| `verify-ingestion-slice.ps1` | `[Environment]::GetEnvironmentVariable("Path","Machine")` returns **$null** on macOS, blanking `$env:Path` so every `docker`/`go` call afterwards fails | guarded with `if ($IsWindows)` |
| `demo-seed.ps1` | same line | same guard |
| `verify-system.ps1` | health-checked port **3000** — the container-internal port, never published since the 3100/3101/9100 remap | now checks **3100** |
| `verify-system.ps1` | shelled out to `powershell`, a Windows-only binary name | re-invokes its own host via `(Get-Process -Id $PID).Path` |
| `check-dashboard-browser.mjs` | hardcoded `C:\Program Files\Google\Chrome\...`, so the browser gate could only ever run on Windows | platform-aware resolution, falling back to Playwright's bundled Chromium |
| `verify-presentation.ps1` | asserted `/pings` total > 0 **without** `show_demo`, which cannot pass on a fixture-only database | asks for `show_demo=true` — this is enterprise-roadmap item **H** |

### One failure genuinely caused by R1 — and the product was right

`verify-ingestion-slice` counted rows from `/pings` expecting **2** after a revenue change. The
Stage 4 collapse returns one row per call, so it saw 1.

Checked against the database before touching the script:

```
ClickHouse     : 2 rows, revenues [42.5, 7.25]   <- both versions stored
API collapsed  : 1 row,  revenue 42.5, syncs=2   <- latest value, correct
API raw=true   : 2 rows, revenues [42.5, 7.25]   <- event stream, correct
```

The stored data and both views are correct; the gate was reading the wrong one. It asserts the
**event-stream** contract ("an unchanged re-send adds no row, a changed re-send adds one"), so it
now passes `raw=true`. `[PROVEN]`

### Two environment fixes (not code)

- `web/node_modules` was absent, so the gate's `next build` and the Playwright check could not
  run. `npm ci` — 398 packages.
- Restarted the five Chapter-2 services stopped earlier for load. With all 18 containers running
  the host load is **2.70** — the earlier thrashing was the API crash-loop, not raw capacity.

## 14. Server deploy note

- **Rebuild:** `platform-sync` (adapter, throttle, syncer) and `api` (pings filter/service,
  platform-capabilities) and `web` (badge + `conversion_reported`).
- **Migrations:** none.
- **New env vars, all with working defaults** — nothing must be set for current behaviour:

| Variable | Default |
|---|---|
| `RETREAVER_WINDOW_OVERLAP` | `15m` |
| `RETREAVER_MAX_PAGES_PER_WINDOW` | `200` |
| `RETREAVER_SWEEP_DAYS` / `RETREAVER_SWEEP_INTERVAL` | `7` / `60m` |
| `RETREAVER_DEEP_SWEEP_DAYS` / `RETREAVER_DEEP_SWEEP_INTERVAL` | `30` / `24h` |
| `RETREAVER_RATE_PER_SEC` / `RETREAVER_RATE_PER_MIN` | `2` / `60` |

- **One-off commands:** none.
- ⚠️ **Behaviour change to announce:** `GET /pings` now returns one row per call by default;
  `?raw=true` restores the event stream. Any external consumer counting rows must be checked.
- Reminder: Retreaver live sync must run from **one place only** (Rule 10), and this branch must
  not merge before `convert-cancel-screen` is reviewed and merged (amendments A6).
