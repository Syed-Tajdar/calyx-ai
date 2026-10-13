# Retreaver convert / void — Stage 4b report

**Branch:** `stage4b/retreaver-writeback`, from `main` @ `14c3068`
**Spec:** `docs/retreaver-writeback-spec.md`
**Status:** Phase 0 and Phase 1 done, plus the owner's three decisions of 2026-10-12 (§0).
Phase 2 GET-only reads done on 2026-10-13 (§5). **STOPPED 🙋**: of the three open items, A and C
are resolved and B (company-level pixels) is still open (§5.4). No Retreaver or Ringba write was made
by Calyx tooling. Only GET requests were sent (14 in total). The api_key and company_id were decrypted in-process and never
printed.

Every claim is labelled `[PROVEN]` (shown by a test or command in this run), `[PROVEN-docs]`
(quoted from Retreaver's docs, with URL), `[INFERRED]` or `[UNKNOWN]`.

---

## 0. Owner decisions (2026-10-12) and what changed

| # | Decision | Done in |
|---|---|---|
| 1 | Keep the R1 redaction helper's `[REDACTED]` output. `***` is only for chat / report display. | No code change. The helper is untouched; every write-back error and audit row still goes through it. |
| 2 | Do **not** mark Retreaver "failed" when the read-back has not matched within the settle window. Same rule for both platforms: the write errors after retries → `failed`; read-back not matching yet → `pending` (unconfirmed); read-back shows a contradicting value → `failed`. | `f1df7e3` (§3.1 "Unsettled rule") |
| 3 | Fix the UI toast that showed "not confirmed yet" for a confirmed write, as a separate commit with a test. Ringba otherwise unchanged. | `25412c8` (§3.2) |

---

## 1. Scoreboard

| Phase | State | Evidence |
|---|---|---|
| 0 — Verify the docs | ✅ Done. No design change, so no 🙋 stop. Five corrections and additions listed in §2. | §2 |
| 1 — Build behind the flag | ✅ Done. Fake Retreaver API, unit tests only, no Docker, no credentials, no network writes. | §3, §4 |
| 2 — Sandbox proof and plan (GET only) | 🙋 Stopped. Reads done; $0 terms and no campaign/target/publisher pixels or postbacks proven; Company ID now matches; recording off. Still open: company-level pixels cannot be listed by the API (B). | §5 |
| 3 — Sandbox run with probe | ⏸ Not started. Needs the owner's written approval of the Phase 2 plan. | §6 |
| 4 — Local end-to-end through Calyx | ⏸ Not started. | §8 |

Gates in this run:

| Gate | Result |
|---|---|
| platform-sync `go test ./...` | ✅ 217 pass, 5 skip, 0 fail. Baseline: 146 pass, 4 skip. Every baseline test still passes `[PROVEN]`. The extra skip is the DB-backed store test without `DATABASE_URL`; run with it, it passes on Postgres 16 `[PROVEN]`. |
| Ringba write-back tests (`writeback/convert_test.go`) | ✅ File unchanged, all 15 green `[PROVEN]` |
| Retreaver sync tests (`adapter/retreaver_*_test.go`, `syncer`) | ✅ Files unchanged, all green `[PROVEN]` |
| API `jest` | ✅ 61 / 61. Baseline: 46 / 46. |
| Web `node --test` | ✅ 23 / 23. Baseline: 9 / 9. |
| Web `next build` | ✅ |
| Migration 000032 on real Postgres 16 (up, down, up) | ✅ `[PROVEN]` |
| Secret scan of every commit | ✅ No secrets. detect-secrets only flagged existing dev defaults and test fakes (§9, item 4). |
| v1 gates `pwsh-preview -V1` | ⏸ Not run. They need the Docker stack, and Phase 0–1 forbids Docker. They run on the owner's machine with Phase 4. |

How this run was done: it ran in a cloud sandbox, not on the owner's machine. The sandbox cannot reach
`proxy.golang.org`, so I did two things for local testing only, with no repo change:
- built Go 1.25.0 from the `golang/go` GitHub tag;
- served the `golang.org/x/*` and `gopkg.in/yaml.v3` modules from their GitHub mirrors through a file
  `GOPROXY`.

The other Go services (consumer, scoring, …) were not re-run here. They need more vanity modules, and
this branch changes no file in any of them (`git diff --stat`, §3.3).

---

## 2. Phase 0 — doc verification

**Sources** (read in full; the HTML page and its Markdown source are the same text):
- Legacy reference: https://retreaver.github.io/core-api-docs/#update-a-call. Source:
  `github.com/retreaver/core-api-docs` @ `fdf0ea0` (30 Sep 2026), `source/index.html.md` L1079–1150.
- Current docs: https://learn.retreaver.com/api/calls and https://learn.retreaver.com/api/errors.

### 2.1 Bullet by bullet

| Spec bullet | Verdict | Quote |
|---|---|---|
| `PUT https://api.retreaver.com/api/v5/calls/{uuid}.json?api_key=…&company_id=…` | ✅ Confirmed `[PROVEN-docs]` | "`PUT https://api.retreaver.com/api/v5/calls/addcf985-017e-4962-be34-cf5d55e74afc.json?api_key=woofwoofwoof&company_id=1`" |
| Body `{"call": {...}}`, `Content-Type: application/json` | ✅ Confirmed `[PROVEN-docs]` | "`Content-Type: application/json`" · "`{"call":{"revenue":20.0,"payout":5.0,"session_notes":"Credited after QA review"}}`" |
| Fields: revenue, payout, receivable, payable, session_notes | ✅ Confirmed: exactly these five `[PROVEN-docs]` | Parameter table: `revenue` float, `payout` float, `receivable` boolean, `payable` boolean, `session_notes` string |
| revenue ≥ 0.01 → receivable; `revenue: 0` clears | ✅ Confirmed `[PROVEN-docs]` | "Money you earned for the call. Setting a value of `0.01` or more marks the Call `receivable`. Passing `0` clears the revenue." |
| Payout and payable work the same way | ✅ Confirmed `[PROVEN-docs]` | "Money you owe the publisher for the call. Setting a value of `0.01` or more marks the Call `payable`. Passing `0` clears the payout." |
| ➕ Not in the spec | ➕ Added `[PROVEN-docs]` | `receivable`: "Set the receivable flag directly. **Ignored when `revenue` is present in the same request.**" (`payable` has the same rule for `payout`.) |
| Values are absolute: set, not added | ✅ Consistent, but `[INFERRED]` | The docs say "Changes any attributes you have passed in on the Call." and that "Passing `0` clears" the value. They never say "absolute". Phase 2 step g proves it. |
| 🔴 "If an edit newly marks the call as converted, its conversion pixels fire." | ✅ Confirmed `[PROVEN-docs]` | "If an edit <strong>newly</strong> marks the Call as converted, its conversion pixels will fire." |
| Pixels and postbacks already fired are not re-sent | ✏️ Correction: the docs say something stronger `[PROVEN-docs]` | "Editing a Call **changes reporting values that may already have been sent up- or downstream**. Postbacks and pixels that already fired, exported reports, and third-party systems that synced the Call will **not** automatically receive the corrected values." So a **void does not recall** a pixel that already fired. The void popup now says so (§3.2). |
| Every edit is saved to version history | ✅ Confirmed `[PROVEN-docs]` | "Every edit is saved in the Call's version history, so all changes are auditable." |
| No convert, void or unconvert endpoint | ✅ Confirmed `[PROVEN-docs]` | The Calls section has V1–V4 reads, "Update a Call" and "Call Data Writing" (tags only), nothing else. learn.retreaver.com/api/calls has no PUT at all. |
| No documented rate limit for PUT | ✅ Confirmed `[PROVEN-docs]` | Legacy: "429 · Too Many Requests -- Slow down. Highly concurrent requests are frowned upon." New: "429 – Too Many Requests – Returned by certain rate-limited endpoints." No numbers. The R1 limits (2/s, 60/min, shared) stay. |
| No documented permissions for PUT | ✅ Confirmed, with one note: `[UNKNOWN]` whether PUT needs more than read rights | Legacy 403: "You didn't pass in your Company ID or you're using the wrong API key." New 403: "You're authenticated, but you don't have permission to access this resource." Nothing about PUT specifically. A 403 on the Phase 3 PUT stops the run. |
| Read-back stays `GET /api/v2/calls/{uuid}.json` | ✅ v2 still documented `[PROVEN-docs]`. Note: the new docs recommend v4. | Legacy: "`GET https://api.retreaver.com/api/v2/calls/{uuid}.json`". New: "**v4 (recommended)**: `GET https://api.retreaver.com/api/v4/calls/:uuid.json`". I kept v2 per the spec; sync uses v2 too (open question Q3). |
| Read-back fields exist | ✅ All nine are in the v2 example `[PROVEN-docs]` | `converted`, `revenue`, `payout`, `receivable`, `payable`, `conversions_determined_at`, `fired_pixels_count`, `postback_value`, `updated_at` |

### 2.2 Two more findings

- ✏️ **The update page is now labelled "Legacy"** `[PROVEN-docs]`. Front matter: "Legacy reference for
  the Retreaver Core API. For the current Retreaver API documentation, visit learn.retreaver.com/api."
  The new docs do not mention `PUT` at all. This does not change the design, because the
  endpoint is still documented. It is a risk to keep in mind (Q2).
  - ✏️ Correction (2026-10-13): the new docs **do** mention `/api/v5`, but only as a read API.
    "Calls API v5 … accepts the same parameters and returns the same fields as v4, plus one:
    `inbound_leg_recording_url`" `[PROVEN-docs]` (https://learn.retreaver.com/api/calls). There is
    still no `PUT` there.
- 🔎 **`fired_pixels_count` also counts pixels that have not fired yet** `[INFERRED]` from the docs'
  own example. The V2 example shows `"fired_pixels_count": 5`. One of its 5 `fired_pixel` rows has
  `"status": "new", "fired_at": null`. So the count includes queued pixel records. The Phase 3 rule
  ("must stay unchanged") is still the right test, because a new record, even a queued one, means a
  pixel was scheduled. The new docs' wording: "How many pixels (webhooks) were fired for this call."

### 2.3 Design impact

None. No dedicated conversion endpoint, no different body, no documented permission. **Convert = PUT
revenue=<amount>, Void = PUT revenue=0** stands.

---

## 3. Phase 1 — design and files

### 3.1 Design

**Per-platform registry.** This replaces the Ringba-only guard, the same way R1 replaced the sync
guard with `RangeFetcher`.
- platform-sync, `writeback.Writer`:
  - methods: `LookupCall` · `Convert` · `Void` · `ReadBack` · `IsConverted` · `ConvertSettled` ·
    `VoidSettled` · `SettleWindow` · `Revenue` · `CheckCredentials`;
  - plus an optional `RequestValidator`.
- `Service` keeps every shared rule and hands only the platform steps to the writer for
  `integration.platform`. The shared rules are: kill switch, tenant flag, Live-only, credentials,
  the 409 lock, and an audit row for every attempt.
- `ErrNotRingba` is now an alias of `ErrPlatformUnsupported`, so `errors.Is` keeps matching. It still
  returns 403 and is audited.
- `NewService(…, ringba, …)` registers Ringba as before. `main` registers Retreaver.
- API: `WRITEBACK_PLATFORMS = ['ringba','retreaver']`, with `supports_writeback` on every integration
  record and in `/integrations/platform-options`.
- Web: the local `WRITEBACK_PLATFORMS = ['ringba']` is gone. The card asks the backend.

Note on the spec's list of three guards: at `14c3068` the API had no `platform !== 'ringba'` check
left in `integrations.service.ts` `[PROVEN]` (grep). It already forwarded every platform to
platform-sync. The API part of the registry is therefore the capability list the UI reads. Refusal
and auditing stay in platform-sync.

**Ringba keeps its exact behaviour** `[PROVEN]`.
- `convert_test.go` is unchanged and green.
- New `registry_test.go` pins what the refactor could have moved:
  - a webhook-list error is still returned bare, with no audit row;
  - void still sends the call's current revenue and payout;
  - the response and audit-row shape is unchanged (no `platform` or `read_back` keys; the original
    11-argument audit insert);
  - the unsettled outcome is still `pending`.

**Retreaver transport** (`adapter.RetreaverCallClient`):
- `SetRevenue` → `PUT /api/v5/calls/{uuid}.json` with exactly `{"call":{"revenue":x}}`. The body
  type has no other field, so `payout` and `session_notes` cannot be sent `[PROVEN]` (the tests
  inspect every PUT body).
- `GetCall` → `GET /api/v2/calls/{uuid}.json`. Parses the nine read-back fields and accepts a number
  or a numeric string.
- Uses the **same per-company limiter instance** as Retreaver sync (`retreaverLimiters`, 2/s and
  60/min) `[PROVEN]`.
- Retries **only 429 and 5xx**, at most 3 attempts. Backoff is 0.5 s × n², honours `Retry-After`,
  capped at 10 s. It resends the same body.
- **Never retries a 4xx** `[PROVEN]`. It also does not retry a transport error. A PUT may have landed
  before the connection dropped, so the read-back decides. This is stricter than the spec.
- The UUID is checked before any request is built. A non-UUID never reaches the URL path.
- Read-backs use `mapRetreaverCallOpts(raw, false)`, so they do not move the sync C3 counters
  `[PROVEN]`. The sync path is unchanged.

**Retreaver writer** (`writeback.retreaverWriter`):

| Step | Rule |
|---|---|
| Before the lock | `api_key` and `company_id` must be present. The call id must be a UUID (else 400). The convert amount is rounded to cents. An amount that rounds to $0.00 is refused (400), because Retreaver would treat it as a void. |
| Lookup | `GET v2`. If the API returns a call whose `company_id` is not the tenant's, it is refused as an account mismatch (403, audited). |
| Precondition | Converted means revenue > 0 and `converted` true (or absent). Convert of a converted call → 409. Void of an unconverted call → 409. Same as Ringba. |
| Convert | `PUT revenue=<amount>` |
| Void | `PUT revenue=0` |
| Read-back | Polls `GET v2` every 2 s within `WRITEBACK_SETTLE_SECONDS_RETREAVER` (default 20). |
| Convert settled (provisional) | `revenue == amount`, and `converted == true` if present |
| Void settled (provisional) | `revenue` is `0` or `null` |
| Never settles | The shared unsettled rule below (owner decision 2): `pending` unless the read-back contradicts the write. |
| Audit | Every row gets `platform='retreaver'` and `read_back` JSON: before and after values for all nine fields, `settle_seconds`, `payout_changed`, `fired_pixels_delta`. Messages and `read_back` go through the redaction helper. Lookup failures are audited too. Ringba keeps its old behaviour here: no row. |

**Unsettled rule — the same for Ringba and Retreaver** (owner decision 2, `f1df7e3`). It lives
once in `Service` (`contradicts`, `pendingMessage`); each writer only supplies `Revenue(state)`.

| What happened | Outcome | HTTP | Audit | Lock |
|---|---|---|---|---|
| The write (Ringba fire/void, Retreaver PUT) errors after retries | `failed` | 502 | `failed` | released |
| Read-back matched within the settle window | `confirmed` | 200 | `confirmed` | released |
| Read-back still shows the old value, or no read-back GET succeeded | `pending` (unconfirmed) | 202 | `pending` | held as `pending` (15 min), so a second click cannot re-send |
| Read-back shows a **contradicting** value: a revenue > 0 that is neither what was written nor what the call had before | `failed` (`read-back contradicts the write`) | 502 | `failed` | released |

- Ringba keeps its pending messages word for word ("fire accepted; not settled within 20s",
  "void accepted; not settled within 20s") and its settle rules. Its one new outcome is the
  contradiction row, e.g. a void whose read-back shows a different non-zero amount; before this change
  that was `pending` `[PROVEN]` (`TestRingbaUnsettledRule`).
- Retreaver's pending message: "write accepted; read-back not yet matching after 20s (unconfirmed)".
- A `converted=false` flag with the right revenue is treated as not-yet-matching (pending), not as a
  contradiction. The flag may lag the revenue; Phase 3 will show whether it does `[UNKNOWN]`.

**Audit storage.** Migration `000032_writeback_audit_readback` adds `platform TEXT` and
`read_back JSONB`, plus `insert_writeback_audit_v2(… , p_platform, p_read_back)` with EXECUTE granted
to `callloom_sync`. Ringba rows still use the original function, so their grants and rows are
unchanged. Verified on Postgres 16:
- all migrations applied (000012 skipped: pgvector is not installed in the sandbox; that migration is
  unrelated);
- `store/audit_test.go` passes;
- 000032 down then up re-applies cleanly.

### 3.2 UI (call detail)

- The Conversion card shows only for an **admin**, on a platform where the **backend** reports
  `supports_writeback`, with a **Live** API integration that has `writeback_enabled`. Otherwise it
  shows the same explanations as Ringba today.
- **Retreaver convert popup:** "RETREAVER · CONVERT", the call UUID, the amount large ("Convert this
  call for $1.00?"), and the red line **"Retreaver may fire this call's conversion pixels/postbacks to
  the buyer or publisher."**
- **Retreaver void:** the button reads **"Void conversion (sets revenue to $0)"**. The popup says:
  "This sets the call's revenue to $0 in Retreaver. Pixels or postbacks that already fired are not
  recalled. It is reversible — the call can be converted again afterwards."
- **Ringba wording is unchanged.** `writeback-support.test.ts` pins every string.
- **Toast fix** (owner decision 3, `25412c8`). `ConversionActions` used to test
  `result.outcome === 'settled'`, a value platform-sync never sends (it sends `confirmed`, or
  `pending` with 202). Every confirmed Ringba or Retreaver write therefore toasted "had not confirmed
  yet". The text now comes from `writebackToast()` in `web/src/lib/writeback-toast.ts`:
  - `confirmed` (and legacy `settled`) → "Confirmed by Ringba — now $18.40";
  - anything else → the old "Sent. Ringba had not confirmed yet (pending) — it may take a moment."
  - Titles ("Converted in …", "Cancelled in …", "Voided in …") are unchanged.
- Screenshots were taken from a **harness page against a mocked API**. They are *not* Phase 4 evidence:
  - `docs/evidence/stage4b/retreaver-convert-popup.png`
  - `docs/evidence/stage4b/retreaver-void-popup.png`
  - `docs/evidence/stage4b/ringba-void-popup.png` (unchanged wording)
  - `docs/evidence/stage4b/retreaver-writeback-off.png` (tenant flag off: "Write-back is turned off for this account.")

### 3.3 Commits (one per item)

| Commit | Item |
|---|---|
| `9927b16` | Spec saved as `docs/retreaver-writeback-spec.md` |
| `e92889f` | Audit `platform` + `read_back` columns, migration 000032, store test |
| `3efb5c3` | Per-platform write-back registry in platform-sync (Ringba unchanged) |
| `e80a342` | Retreaver single-call transport (PUT v5 revenue, GET v2 read-back, limiter, retry, redaction) |
| `392893d` | Retreaver writer: Convert / Void / ReadBack, settle, audit read-back, server wiring, env |
| `f1c0daa` | API write-back registry: `supports_writeback` per integration |
| `0e54c0d` | `platform-options` reports `supports_writeback` |
| `11f5d31` | Web: Convert / Void for Retreaver driven by the backend, popup, void wording |
| `b442f33` | Report (Phase 0–1) |
| `f1df7e3` | Owner decision 2: unsettled read-back is pending for every platform; contradiction is failed |
| `25412c8` | Owner decision 3: toast fix (confirmed write no longer says "not confirmed yet") |
| *(this file's update)* | Report updated with the decisions |

Files changed: 33 including this report, all inside `services/platform-sync`, `services/api`, `web/src`, `db/postgres`,
`deploy/docker-compose.yml`, `.env.example` and `docs/`. No other service is touched.

---

## 4. Test list

No existing test file was modified (`git diff --name-status 14c3068..HEAD`: test files are `A` only)
`[PROVEN]`.

### platform-sync — `internal/adapter/retreaver_writeback_test.go` (transport, fake Retreaver over httptest)
- `TestRetreaverSetRevenuePutsOnlyRevenueToV5`: PUT, v5 path, query carries the key and company,
  JSON content type, body exactly `{"call":{"revenue":1}}`, no payout / session_notes / receivable / payable
- `TestRetreaverVoidSendsRevenueZero`: `{"call":{"revenue":0}}`; 0 is sent, not omitted
- `TestRetreaverGetCallReadsV2AndParsesReadBack`: v2 path, all nine fields, null payout stays nil,
  numeric-string revenue
- `TestRetreaverReadBackDoesNotMoveSyncC3Counters`
- `TestRetreaver429ThenSuccess`: Retry-After honoured; the retry resends the same body
- `TestRetreaver5xxRetriedWithBackoffThenSuccess`
- `TestRetreaver4xxNeverRetried`: 400, 401, 403, 404, 405, 406, 410 and 422 each send exactly 1 request
- `TestRetreaverFailedPutNeverLeaksKey`: 400 / 401 / 422 bodies that echo the URL, 500 ×3 with a
  `Location` header that echoes the key, 429 ×3, and a network error (PUT and GET). No key in any
  error; the redacted URL is still present.
- `TestRetreaverNetworkErrorNotRetried`
- `TestRetreaverInvalidUUIDSendsNothing`: includes a path-traversal attempt
- `TestRetreaverMissingCredsSendsNothing`
- `TestRetreaverWritebackSharesSyncLimiter`: same limiter instance as sync; defaults 2/s and 60/min

### platform-sync — `internal/writeback/retreaver_writer_test.go` (Service → writer → real transport → stateful fake Retreaver)
- `TestRetreaverConvertConfirmed`: audit row is `confirmed`, `platform=retreaver`, with the before
  and after `read_back`, `fired_pixels_delta=0`, `payout_changed=false`
- `TestRetreaverVoidConfirmedNullAndZero`: void leaves revenue `null` in one case and `0` in the other;
  both confirm
- `TestRetreaverReconvertAfterVoid`: convert → void → convert $0.75; three PUTs, all confirmed
- `TestRetreaverConvertRefusedWhenConverted` / `TestRetreaverVoidRefusedWhenNotConverted`: 409, no PUT
- `TestRetreaverUnsettledReadBackIsPendingNotFailed`: convert PUT accepted but read-back still old;
  void PUT accepted but read-back still converted; every read-back GET fails after the PUT. Each is
  `pending` (no error), audited `pending`, the lock is held, exactly one PUT.
- `TestRetreaverContradictingReadBackIsAuditedFailure`: convert $1 stored as $2; void stored as $0.50.
  Each is an audited `failed`, 502, "read-back contradicts the write", and the lock is released.
- `TestRetreaver429ThenSuccessThroughService`
- `TestRetreaver4xxPutNotRetriedAndAudited`: 422 → 1 PUT, audited failed, the message names Retreaver
- `TestRetreaverRedactionOnEveryFailurePath`: PUT 400 / 401 / 403 echo, 500 ×3 echo, 429 ×3,
  transport error, void PUT 500, GET 500, GET 404. The key is in none of: the returned error, the
  Result message, the Result JSON, any audit `message` or `read_back`.
- `TestRetreaverSharedGuards`: each case is audited and sends no Retreaver request unless it says otherwise.
  - kill switch → 403
  - tenant flag off → 403
  - not Live → 403
  - 409 lock held
  - double click → exactly one PUT
  - amount 0, −1 and 0.004 → 400; $1000 is accepted (no cap)
  - invalid UUID → 400
  - call of another company → 403
  - call not found → 404, and the message names Retreaver
  - missing `company_id`
- `TestRetreaverPixelAndPayoutWatchRecorded`: a pixel firing on convert is recorded as `fired_pixels_delta=1`
- `TestRetreaverSettleWindowFromEnv`: default 20 s, override, per-platform window

### platform-sync — `internal/writeback/registry_test.go`
- `TestRegistryRingbaRegisteredByNewService`
- `TestRegistryUnsupportedPlatformRejectedAndAudited`: trackdrive, callgrid, and Retreaver when not
  registered → 403 and audited, no platform call
- `TestRingbaWebhookListErrorStaysBareAndUnaudited`
- `TestRingbaVoidPassesCurrentRevenueAndPayout`
- `TestRingbaResponseAndAuditShapeUnchanged`
- `TestRingbaUnsettledRule`: void still converted at the old amount → `pending` with the pre-4b message
  and the lock held; convert never settles → `pending` with the pre-4b message; void read-back at a
  different non-zero amount → `failed` (502); fire error → `failed` (unchanged)
- `TestContradictsRule`: the rule's table (no read-back, old value, written value, cleared, different
  amount on convert, different non-zero amount on void)

### platform-sync — `internal/store/audit_test.go` (needs `DATABASE_URL`; passed on Postgres 16 here, skips otherwise)
- `TestInsertWritebackAuditRingbaAndRetreaver`: the Ringba row keeps `platform` and `read_back` NULL;
  the Retreaver row stores both

### API — `src/integrations/retreaver-writeback.spec.ts` (15 tests)
- registry list; `supportsWriteback` case-insensitive; `platform-options` and integration records carry `supports_writeback`
- 403 for non-admin roles; CSRF required on the Retreaver paths; amount 0 → 400, $1000 accepted
- forwarded convert body is exactly {integration_id, tenant_id, user_id, inbound_call_id, amount};
  void has no amount; neither has payout or session_notes
- kill switch and tenant flag → 403, and the request is still forwarded so platform-sync audits it
- 409 lock; a `pending` 202 is passed through as 202 with `outcome: pending` intact; a 502
  contradiction → 503 with the message; missing internal token → 503 with no forward

### Web — `src/lib/writeback-support.test.ts` (10 tests)
- support comes from the backend; hidden before the backend answers and for trackdrive / callgrid;
  hidden for non-admins
- ready only when supported **and** Live **and** `writeback_enabled`; Test/paused (mock) → "No live …"
- Ringba states unchanged
- Retreaver pixel line and void wording are exact; Ringba wording is byte-identical

### Web — `src/lib/writeback-toast.test.ts` (4 tests, owner decision 3)
- a confirmed Ringba convert says "Confirmed by Ringba — now $18.40", never "not confirmed"
- a confirmed Retreaver void says "Confirmed by Retreaver — now $0.00"
- `pending` keeps the old Ringba wording exactly
- legacy `settled` counts as confirmed; a missing amount shows $0.00

### Unchanged suites, still green `[PROVEN]`
- Ringba write-back: `writeback/convert_test.go`, 15 tests
- Retreaver sync: `adapter/retreaver_*_test.go`, 24 Retreaver test results, plus `syncer`
- every other platform-sync test from the baseline (146 → still 146 passing)
- API `writeback.routes.spec.ts`, `writeback.validation.spec.ts`
- web `writeback-role`, `writeback-amount`

**Mutation check `[PROVEN]`.** I removed `redact.SanitizeErr` from the transport's error path. Both
redaction tests failed and printed the leaked (fake) key. I then restored the code; the tree is clean.

---

## 5. Phase 2 — sandbox proof and plan

Run on 2026-10-13 on the owner's machine (local stack `calyx-4b`). **GET only.** No PUT, POST or
DELETE was sent to Retreaver or Ringba.

### 5.1 How it was read

- **Integration:** "My Test", added by the owner in Settings. Retreaver, `api_sync`, `sync_mode=mock`
  (Test), `is_active=false` (paused), `writeback_enabled=false` `[PROVEN]` (Postgres row). It is
  never Live on this machine.
- **Probe:** `services/platform-sync/cmd/probe-retreaver-writeback-get`. It is not in any image,
  because the Dockerfile builds only `./cmd/server`.
  - It reads the credentials with `get_api_sync_integration_any()`, the same lookup as the
    write-back path, and decrypts them in-process.
  - The encryption key and `DATABASE_URL` come from the running platform-sync container through
    **stdin**, never through a command line.
  - The probe runs in a `golang:1.26.5-alpine` container on the `calyx-4b_default` network.
  - Its HTTP transport **refuses every method except GET** and every host except
    `api.retreaver.com`.
  - Every URL and error goes through `internal/redact`.
  - Printed objects are masked: phones, `company_id`, SIP fields, object/show keys, postback key
    UUIDs, and URL paths. Only URL hosts are shown.
  - It refuses an integration that is Live.
- **Requests in the final run (5, all GET, 700 ms apart, so under the shared 2/s limit):**

| # | Request (key redacted) | HTTP |
|---|---|---|
| 1 | `GET /api/v2/calls/a82a7a80-f1fd-46a5-a018-379dd7893ea7.json?api_key=[REDACTED]&company_id=[REDACTED]` | 200 |
| 2 | `GET /campaigns/cid/3d4d4d1c.json?…` (from the call's `cid`) | 200 |
| 3 | `GET /targets/149596.json?…` (from the call's `target_id`) | 200 |
| 4 | `GET /api/v1/affiliates/afid/11111.json?…` (legacy afid lookup, from the call's `afid`) | **404** |
| 5 | `GET /api/v1/affiliates/71582.json?…` (id lookup from https://learn.retreaver.com/api/affiliates, from `system_affiliate_id`) | 200 |

  Before this run: a first run stopped at the database lookup before any request; a second run sent
  requests 1–4 above. The final run differs only in fixes to the masking and the affiliate lookup.
  In total, 9 GETs were sent to Retreaver. Request 4's 404 is `[UNKNOWN]`: the docs show the afid lookup, but this account
  answers 404 for afid `11111`, while the id lookup returns the same publisher (afid `11111`).

### 5.2 The call now (baseline for Phase 3) `[PROVEN]`

| Field | Value |
|---|---|
| `converted` | `false` |
| `revenue` | `null` |
| `payout` | `null` |
| `receivable` | `false` |
| `payable` | `false` |
| `fired_pixels_count` | **`1`** (see 🙋 B) |
| `conversions_determined_at` | `2026-10-12T17:48:38.314Z` |
| `postback_value` | `null` |
| `updated_at` | `2026-10-12T17:48:45.838Z` |
| other | `status=finished`, `total_duration=85`, `dialed_call_duration=74`, `connected=true`, `session_notes=null`, `duplicate=false`, all three `*_sale_timer_fired=null` |

### 5.3 Pixels, postbacks and terms

| Object | What GET shows | Verdict |
|---|---|---|
| Campaign `3d4d4d1c` "Calyx Sandox - Do not use" (id 54339) | `timers: []`, `postback_keys: []`, `conversion_propagation: false`. One conversion group, "Default Conversion Settings": `timer`, 90 s, `revenue 0`, `payout 0`, `postback: false`, `postback_uuid: null`. One target (149596). `record_calls: true`. | ✅ No campaign pixels or postbacks; $0 terms `[PROVEN]`. ⚠️ Recording is **on** (🙋 C). |
| Target 149596 "Calyx - Test buyer" | `conversion_groups: []`, `timers: []`, `tag_values: []`, caps all `null`, not paused | ✅ No target conversion terms, timers or postbacks `[PROVEN]` |
| Publisher 71582, afid `11111`, company name "Calyx- Test Sandbox" | `conversion_groups: []` | ✅ No publisher payout terms `[PROVEN]`. So a convert should not set `payout` `[INFERRED]`. |
| The call's `fired_pixels` | 1 entry: host `api.blacklistalliance.net`, `fire_order 0`, created 17:46:53 (**at call start**), `status: "error"`, `webhook_name: null` | ⚠️ A pixel **exists** that is not on the campaign, target or publisher (🙋 B) |

### 5.4 🙋 Open items: why Phase 2 stops here

**Update 2026-10-13, after the owner's answers.** The GET probe was re-run with the re-added
integration "My test" (`bb5b0fef…`, `mock`, paused, `writeback_enabled=false`): 5 GETs, same
endpoints, same HTTP codes. Total GETs so far: 14.

| Item | State |
|---|---|
| A. Company ID | ✅ **Resolved.** The owner re-entered it. `call.company_id == stored company_id` is now `true` `[PROVEN]`. |
| B. Company-level pixels | 🙋 **Still open.** The owner checked the campaign's conversion criteria (timer / attempt / postback). One type must always be set; it is set to `timer` at $0 with postback off, which §5.3 already proves. That is the campaign's own setting. It does not answer whether a **company-level** pixel or webhook (like the Blacklist Alliance one) fires on conversion. |
| C. Recording | ✅ **Resolved.** The owner turned it off. The campaign now shows `record_calls: false`, `updated_at 2026-10-13T16:25:38Z` `[PROVEN]`. |
| Campaign name | ✅ The owner confirmed that "Calyx Sandox - Do not use" is the sandbox campaign. |
| The call | Unchanged since §5.2: same nine fields, `fired_pixels_count=1` `[PROVEN]`. |

⚠️ **Incident while re-adding the integration (no data affected).**
- From API and platform-sync logs, "My test" ran a **real** (Live) sync for about a minute after
  it was created:
  - `POST /integrations` at 16:46:27Z;
  - a credential test that took 1.4 s (the mock test is instant);
  - a `POST …/sync` from 16:46:28 to 16:47:45 that ended in 503: "network error (transient, will
    retry): context deadline exceeded";
  - then two `PATCH …/sync-mode` (16:47:38, 16:48:33) and a pause at 16:49:03.
- Only the real adapter makes HTTP calls; the mock adapter makes none. So the integration was in
  Live mode during that sync `[INFERRED]`.
- The 30-day fetch timed out before anything was published. ClickHouse has no Retreaver ping
  after 16:04:21Z, which was the old integration's single mock ping `[PROVEN]`.
- The final state is `mock` and paused `[PROVEN]`.
- The red "error" badge in Settings is the stale status of that failed sync. It stays until a
  successful sync and is harmless while the integration is paused.

The original findings follow.

**A. The stored Company ID does not match the call's company `[PROVEN]`.**
- The call's `company_id` is a 6-digit number.
- The `company_id` saved in "My Test" is 14 characters long and **not all digits**. It is not the
  API key, and it has no surrounding whitespace. Neither value is printed here.
- Retreaver still answered every GET with 200, so it seems to resolve the company from the API key
  and not check `company_id` on these reads `[INFERRED]`.
- The Calyx writer compares the two values, so Phase 4 would refuse the call as an account mismatch
  (403, audited) `[PROVEN]` (`retreaverWriter.LookupCall`). It is also `[UNKNOWN]` what a PUT
  with a wrong `company_id` would do.
- 🙋 **Owner:** in Settings, re-enter "My Test" with the numeric Company ID from Retreaver (Test
  mode, then pause again). I re-run the GET probe to confirm `== true` before any write.

**B. Company-level pixels cannot be proven absent with GET.**
- The documented API has no endpoint that lists company-level pixels or webhooks. Both the legacy
  reference and learn.retreaver.com/api were checked `[PROVEN-docs]`.
- This call already shows one: a Blacklist Alliance lookup that fired at **call start**, not at
  conversion. It is on none of the campaign, target or publisher, so it is configured at the
  company level `[INFERRED]`.
- Whether any company-level pixel or webhook fires on **conversion** is `[UNKNOWN]`.
- 🙋 **Owner:** in the Retreaver UI, check every pixel/webhook in this company. Confirm that none
  fires on a conversion (or on revenue/receivable) for campaign "Calyx Sandox - Do not use". If one
  does, pause it or scope it away from this campaign first.
- In Phase 3, `fired_pixels_count` must then stay at **1** after every step.

**C. Recording is on for the sandbox campaign** (`record_calls: true`). The spec asked for
recording off. A revenue edit does not touch recording `[INFERRED]`. 🙋 Accept as is, or turn
recording off in Retreaver.

Also for the owner to confirm: the campaign is named "Calyx Sandox - Do not use" in Retreaver. The
spec calls it `CALYX-SANDBOX-DO-NOT-USE`. Is this the same sandbox campaign?

### 5.5 Plan for Phase 3 (needs A–C settled, then the owner's written approval)

- **Only call:** `a82a7a80-f1fd-46a5-a018-379dd7893ea7`.
- **Amounts:** $1 at most.
- **Body:** only `revenue`. Never `payout` or `session_notes`.
- **Pacing:** one step at a time, all through the shared limiter.
- The probe (`cmd/probe-retreaver-writeback`, not in any image) uses the stored "My Test"
  credentials.
- **Read-back fields after every step:** `converted`, `revenue`, `payout`, `receivable`,
  `payable`, `fired_pixels_count`, `conversions_determined_at`, `postback_value`, `updated_at`.
- **Stop at once if:** `fired_pixels_count` rises above 1, `payout` changes, the company check is
  not `true`, any PUT returns 4xx/5xx, or anything else differs from "Expected".

| Step | Request (key redacted) | Expected | Read-back fields | Undo |
|---|---|---|---|---|
| a. read | `GET /api/v2/calls/a82a7a80-…7ea7.json?api_key=[REDACTED]&company_id=[REDACTED]` | 200; the §5.2 baseline; company check `true` | all nine | — |
| b. convert $1 | `PUT /api/v5/calls/a82a7a80-…7ea7.json?api_key=[REDACTED]&company_id=[REDACTED]`, body `{"call":{"revenue":1}}` | 200 | (see c) | step d |
| c. read until settled (timed) | `GET /api/v2/calls/…json` every 2 s, at most 60 s | `revenue=1`, `receivable=true`, `converted=true` `[UNKNOWN]`, `payout=null`, `payable=false`, `fired_pixels_count=1`, `updated_at` moves | all nine + seconds to settle | step d |
| d. void | `PUT /api/v5/calls/…json`, body `{"call":{"revenue":0}}` | 200 | (see e) | step f restores the convert, if wanted |
| e. read | `GET /api/v2/calls/…json` (poll as in c) | `revenue` `0` or `null` `[UNKNOWN]`, `receivable=false`, `converted=false` `[UNKNOWN]`, `payout=null`, `fired_pixels_count=1` | all nine + seconds | — |
| f. convert $1 again | `PUT …`, body `{"call":{"revenue":1}}`, then read as in c | as c; `fired_pixels_count` still 1, so a re-convert does not re-fire | all nine + seconds | step h |
| g. convert $1 once more (proves absolute) | `PUT …`, body `{"call":{"revenue":1}}`, then read as in c. This is the probe only: the Calyx UI would answer 409 here, by design (Q4). | `revenue=1`, **not 2**; `fired_pixels_count=1` | all nine | step h |
| h. void | `PUT …`, body `{"call":{"revenue":0}}`, then read as in e | as e | all nine + seconds | — |
| i. final read | `GET /api/v2/calls/…json` | **End state:** not converted, `revenue` `0`/`null`, `receivable=false`, `payout=null`, `payable=false`, `fired_pixels_count=1` | all nine | — |

In total: **5 PUTs** (b, d, f, g, h), each followed by at most 30 GETs. Every edit is also saved in
Retreaver's version history `[PROVEN-docs]`.

## 6. Phase 3 — sandbox run

⏸ Not started.

## 7. Final rules

⏸ Phase 3 sets these. Provisional rules: §3.1.

## 8. Phase 4 — local end-to-end

⏸ Not started.

---

## 9. Server deploy note (for when the owner decides; nothing is deployed by this branch)

1. **Images to rebuild:** `platform-sync`, `api`, `web`. No other service changed.
2. **Migration:** `000032_writeback_audit_readback` runs with the normal `migrate` step. It is
   additive: two nullable columns and one new function. Rolling back the images does not need the down
   migration.
3. **Env vars:**
   - New: `WRITEBACK_SETTLE_SECONDS_RETREAVER` (default 20, set in compose). Change it only after
     Phase 3 measures the real settle time.
   - Test-only: `RETREAVER_API_ORIGIN` (unset in production; tests pass an origin directly).
   - Unchanged: `WRITEBACK_ENABLED` stays `false`. It is `${WRITEBACK_ENABLED:-false}` in both compose
     services and empty in `deploy/.env.example`.
   - Every integration's `writeback_enabled` stays `false` until the owner turns it on.
4. **Secret scan:** every commit `9927b16..HEAD` was scanned (targeted greps plus detect-secrets).
   - No keys, tokens or `.env` files.
   - detect-secrets flagged only existing dev defaults in `.env.example` (my diff there is two comment
     lines) and test fakes (`SECRETKEY-wb-…`, the existing test `JWT_SECRET`).

---

## 10. Open questions for the owner

1. ~~Masking style~~ — **decided:** keep `[REDACTED]` (§0).
2. **The update endpoint is "Legacy".** Only the legacy reference documents `PUT /api/v5/calls/{uuid}`;
   the new site does not mention it. Ask Retreaver support whether v5 PUT is supported long-term?
3. **Read-back version.** The new docs call v4 "recommended". The spec and sync use v2, so I kept v2.
   Move write-back read-back to v4 later?
4. **Re-convert a converted call with a new amount.** The PUT is absolute, so this is technically
   possible. The design refuses it (409, same as Ringba): void first, then convert. Keep it that way?
5. ~~Unsettled read-back~~ — **decided:** pending, same rule for both platforms (§0, §3.1).
6. ~~Toast wording~~ — **fixed** in `25412c8` (§3.2).
7. **Call id in the URL.** For Retreaver calls the call-detail route id must be the Retreaver UUID; the
   D8b notes say `external_call_id` = UUID. If any route passes a different id, write-back refuses it
   with 400 (UUID check) and sends nothing. Phase 4 will confirm.
8. **Gates.** The full `pwsh-preview -V1` gate needs Docker, so it waits for Phase 4 on your machine.
9. **Pending lock and Retreaver.** A `pending` result holds the call's lock for 15 minutes (Ringba's
   existing `pendingTTL`). For Retreaver that means a second convert or void on the same call is
   refused (409) for up to 15 minutes after an unconfirmed write. Phase 3 timings will show whether
   Retreaver needs a shorter hold.
