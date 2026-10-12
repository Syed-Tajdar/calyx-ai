# Calyx — Retreaver convert / void: fast track (R3 + Stage 4b in one)

**Branch:** `stage4b/retreaver-writeback`, created from `main` @ `14c3068`. It replaces
`stage4/r3`; leave that branch as it is.
**Operator:** Syed, in his own Claude Code on his own machine · **Owner:** Syed
**Report:** `docs/retreaver-writeback-report.md`
**Goal (owner):** Retreaver convert/void built, proven on a sandbox call, and deployed
**this week**. Write-back stays **off** for every tenant until the owner turns it on.

This runs **in parallel** with Talha's `v1/golive-prep`, on a separate machine.
- **Phase 0 and Phase 1** need no Docker and no credentials, only the Go and Node toolchains
  for unit tests.
- **Phase 2–4** need a local Calyx stack (Docker) with the owner's Retreaver integration
  saved in **Settings** as **Test/paused, never Live**. The server becomes the only Live sync
  (Rule 10).
- Do not run the Phase 3 probe while the server is doing its first 30-day Retreaver backfill.
  Both share the 60/min limit.
- Wherever this spec says "laptop", read "this machine's local stack".

---

## What the docs already say (Claude's research, verify it in Phase 0)

Source: `https://retreaver.github.io/core-api-docs/`, section "Calls" → "Update".

- **Update a call:** `PUT https://api.retreaver.com/api/v5/calls/{uuid}.json?api_key=…&company_id=…`
  - Body: `{"call": {...}}`, `Content-Type: application/json`
  - Fields: `revenue` (float), `payout` (float), `receivable` (bool), `payable` (bool),
    `session_notes` (string)
- `revenue` ≥ 0.01 marks the call **receivable**. `revenue: 0` clears revenue. Payout and
  `payable` work the same way. These are **absolute** values: the amount is set, not added.
- 🔴 **"If an edit newly marks the call as converted, its conversion pixels fire."** Pixels and
  postbacks that already fired are **not** re-sent. Every edit is saved to Retreaver's
  **version history**.
- There is **no** documented "convert", "void" or "unconvert" endpoint. Setting revenue to `0`
  is the only documented way to clear it.
- The docs mention no rate limits or permissions for PUT. The R1 limits stay: **2/s and 60/min
  per company_id, shared with sync**.
- Read-back stays `GET /api/v2/calls/{uuid}.json`.

So, for Calyx:
- **Convert** = `PUT revenue = <amount>`
- **Void** = `PUT revenue = 0`

Open questions for the sandbox:
- Does `converted` become true or false?
- Does a void leave `revenue` at `0` or `null`?
- Does `payout` change?
- How long until GET shows the new values?
- Does `fired_pixels_count` change?

## Owner decisions in this spec (🙋 if you disagree)

- **Convert sets `revenue` only.** `payout` is not sent, the same as Ringba, where Calyx sets the
  conversion amount. If the sandbox shows that payout changes on its own, **stop** and report.
- **Never send `session_notes`.** It could overwrite notes the team wrote in Retreaver.
  Retreaver's own version history plus the Calyx audit row are the audit trail.
- **Pixels:** on a real call, a Calyx convert can fire that campaign's conversion pixels to
  buyers or publishers, exactly like editing the call in the Retreaver UI. The confirmation
  popup must say this for Retreaver calls (see Phase 1).

## ⛔ Rules

1. 🔴 **Redaction.** The API key travels in the URL. Every log line, error, audit row and test
   output for PUT goes through the R1 redaction helper. Add a test proving a failed PUT
   (4xx/5xx and network error) never logs the key.
2. **No Retreaver write** until the owner approves the Phase 2 plan in writing. After that:
   - only on the **one sandbox call** the owner names
   - amounts **≤ $1**
   - stop at once if `fired_pixels_count` increases or anything differs from the plan
3. **No Live sync on the laptop.** The probe in Phase 3 reads the credentials from the stored
   "My test" integration, decrypted in-process, and does not sync. Credentials are never typed
   into chat.
4. `WRITEBACK_ENABLED` stays `false` in every compose default and on the server. The laptop may
   set it to `true` only for the Phase 4 local check, and back to `false` afterwards.
5. Do not change the laptop `.env` encryption key. Gates run with `pwsh-preview` `-V1`. One
   commit per item, prefix `stage4b:`. Push this branch only, **never `main`**.
6. Label every claim `[PROVEN]`, `[PROVEN-docs]` (with URL), `[INFERRED]` or `[UNKNOWN]`.

---

## Phase 0 — Verify the docs (no network writes)

Read the same docs section. Confirm or correct every bullet above, with quotes.

If the docs say anything that changes the convert/void design, **stop 🙋**. Examples: a
dedicated conversion endpoint, PUT being v5-only with a different body, or a permission the
key needs.

## Phase 1 — Build behind the flag (fake Retreaver API, unit tests only)

**1. Per-platform write-back.** Replace the three Ringba-only guards with one registry, the same
way R1 replaced the sync guard with `RangeFetcher`:
- API: `integrations.service.ts` (`platform !== 'ringba'`)
- platform-sync: `writeback/convert.go` (`ErrNotRingba`)
- web: `WRITEBACK_PLATFORMS`

The interface is something like `LookupCall / Convert / Void / ReadBack`. Ringba keeps its exact
current behaviour; prove it with its existing tests unchanged and green.

**2. Retreaver implementation:**
- `Convert(amount)` → `PUT /api/v5/calls/{uuid}.json` `{"call":{"revenue":amount}}`
- `Void()` → `PUT … {"call":{"revenue":0}}`
- `ReadBack()` → `GET /api/v2/calls/{uuid}.json`. Return `converted`, `revenue`, `payout`,
  `receivable`, `payable`, `conversions_determined_at`, `fired_pixels_count`,
  `postback_value` and `updated_at`.
- Use the **same per-company limiter** as Retreaver sync (2/s, 60/min).
- Retry only on 429/5xx, with backoff. Never retry a 4xx.

**3. Settle and read-back:**
- Per-platform settle window: `WRITEBACK_SETTLE_SECONDS_RETREAVER`, default 20 until Phase 3
  measures the real value.
- Provisional rule:
  - Convert succeeded = `revenue == amount`, and `converted == true` if the field is present.
  - Void succeeded = `revenue` is `0` or `null`.

  Phase 3 finalises it.
- Store the read-back fields in the audit row. No secrets.

**4. Everything shared stays as it is, now for Retreaver too:**
- kill switch, per-tenant flag, admin-only (403), CSRF
- idempotency lock (409)
- audit on every attempt including rejections
- amount must be finite and > 0, no cap

**5. UI (call detail, Retreaver call):**
- Convert/Cancel appear only when the backend reports Retreaver write-back as supported **and**
  write-back is enabled for the tenant, exactly like Ringba today.
- The confirmation popup says: platform **Retreaver**, call id, the amount written large, and
  this line: **"Retreaver may fire this call's conversion pixels/postbacks to the buyer or
  publisher."**
- Cancel wording for Retreaver: **"Void conversion (sets revenue to $0)"**.

**6. Tests**, with a fake Retreaver API:
- convert, void, re-convert
- read-back mismatch → failure audited
- 429 then success
- 4xx not retried
- redaction on every failure path
- 403 non-admin, CSRF, kill switch, tenant flag off, 409 lock
- Ringba suite unchanged

## Phase 2 — Sandbox proof and plan (GET only) 🙋 STOP

1. 🙋 The owner gives the **UUID of a test call** on the sandbox campaign
   `CALYX-SANDBOX-DO-NOT-USE`: internal buyer and publisher, $0 terms, no pixels or postbacks,
   recording off.
2. With GET only, read the call, its campaign, target and affiliate (publisher). **Prove** that
   no pixel or postback is configured and that the terms are $0. If this cannot be proven,
   **stop 🙋**.
3. Write the plan table:

| Step | Request (key redacted) | Expected | Read-back fields | Undo |
|---|---|---|---|---|

   It runs in this order:
   - a. read
   - b. convert $1
   - c. read until settled (time it)
   - d. void
   - e. read
   - f. convert $1 again
   - g. convert $1 once more, to prove absolute (expect 1, not 2)
   - h. void
   - i. final read

4. Commit the report, push, give the hash, **STOP** for the owner's written approval.

## Phase 3 — Sandbox run with a probe tool (after approval)

- `cmd/probe-retreaver-writeback`: not in any image, reads the stored credentials, runs the
  approved steps **exactly**, one at a time.
- After each step, record every read-back field and the seconds until settled.
- **`fired_pixels_count` must stay unchanged after every step. If it rises: stop at once 🙋.**
- End state: the call is **not converted, revenue 0 or null**. Prove it.
- Then state the **final rules**, and update the Phase 1 code and tests if they differ from the
  provisional ones:
  - convert success
  - void success
  - the settle window
  - what Calyx shows after a void
  - whether the next sync sees the change (`updated_at` moved)

## Phase 4 — Local end-to-end through Calyx

The point is to exercise the real code path, not the probe.
1. On the laptop, set `WRITEBACK_ENABLED=true` temporarily.
2. Turn on the tenant flag for the admin only.
3. Get the sandbox call into the laptop Calyx with a **single-call fetch only**: the existing
   single-call verify/ingest path, or a minimal one-call import. **No Live sync.**
4. Run Convert $1 → read-back → audit row, then Void → read-back → audit row, from the call
   detail page.
5. Take screenshots of the popup, before and after, and the audit rows.
6. Set `WRITEBACK_ENABLED=false` again.
7. If no single-call import exists and building one is not small, **skip Phase 4 🙋**. The same
   check then happens on the server on the owner's "write-back on" day.

## Output and gates

`docs/retreaver-writeback-report.md`:
1. Scoreboard: Phase 0–4
2. Phase 0 doc quotes with URLs
3. The design and the files changed
4. The test list
5. The Phase 2 proof and plan
6. Phase 3 step-by-step read-backs, timings and `fired_pixels_count`
7. The final rules
8. Phase 4 screenshots
9. A **server deploy note**: images, new env vars, `WRITEBACK_ENABLED` stays `false`
10. Open questions for the owner

Requirements:
- v1 gates pass in `-V1`.
- On this machine there is no certified Retreaver data, so the 3,652 / $33,143.85 check is
  Talha's. Instead, prove that the Ringba write-back tests and the Retreaver sync tests are
  unchanged and green.
- Secret scan of every commit, with extra care for probe output.

Two stops:
- **after Phase 2** (plan approval)
- **at the end**: push, hashes, STOP; the owner decides the merge
