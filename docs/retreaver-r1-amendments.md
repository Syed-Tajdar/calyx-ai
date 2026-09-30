# Calyx — Stage 4 R1: Owner's review of R0 + amendments to the R1 design

**Applies to:** `docs/retreaver-stage4-spec.md`, PHASE R1
**Reviews:** `docs/retreaver-stage4-report.md`, PHASE R0
**Status:** R0 accepted. **`GO R1`**, subject to every item below. Where this file and the
R0.5 design disagree, this file wins.

---

## A. Owner decisions on R0's open questions

| # | Question | Decision |
|---|---|---|
| 1 | Rotate the Retreaver key | **No — not now.** This is the running account; rotating could break the clients' existing setup. The key stays as it is. Because it can't be rotated, **B2 (no future leaks) is mandatory**. |
| 2 | Rule 7 without an account name | Last 4 of the company ID (`5270`) is accepted, **plus** one stronger check before R2 (C8). |
| 3 | Rule 10 | Stays open until R2. |
| 4 | Throttle 2/s · 60/min · concurrency 1 per `company_id` | **Approved** as the default, with 429 + `Retry-After` handling. |
| 5 | `pwsh` | **Must be installed before R1 is reported.** The operator types his own admin password: `brew install --cask powershell@preview`. The three real gates are the acceptance standard for code changes. The Node/Go substitutes were fine for R0 only. |
| 6 | Branch cut from `convert-cancel-screen` (`c4e3c5d`) | **Accepted**, on one condition: `stage4/retreaver` is **never merged before** `convert-cancel-screen` has been reviewed and merged to `main` on its own. Stages 1–3 have not been reviewed yet. |

---

## B. Corrections to R0

**B1 — "Terminal only" was not accurate.** Claude Code writes session transcripts to disk
(`~/.claude/projects/…` on the operator's Mac). The key almost certainly reached that file.
The grep only covered the repo and the scratchpad.
- Report whether the transcript files contain the key: a yes/no count only, never print the
  match.
- The key will not be rotated (A1). The operator keeps those transcript files private to his
  machine: not shared, not uploaded, not attached to any report.

**B2 — The API key travels in the URL query string, so every error can leak it.
Mandatory: the key cannot be rotated, so it must not leak again.** Go's
`*url.Error` includes the full request URL. We hit this exact class of bug in the
`withAuthContext` incident: Go transport errors carry the URL. Any timeout, DNS failure or
TLS error on a Retreaver request can therefore write `api_key=…` into platform-sync logs,
the integration's `last_error` column, or a report. The `Link` header is the same problem.
R1 must:
1. **Check the official docs for header-based auth** (e.g. `Authorization` or an `X-…`
   header). If Retreaver accepts it, move the key out of the query string. Prove it with one
   live GET. Cite the doc URL.
2. Whether or not (1) works: redact `api_key` and `company_id` (raw and URL-encoded) in
   **every** error string and log line the Retreaver adapter can produce. Make it one shared
   helper, not ad-hoc code.
3. Add a test that forces a transport error (for example an unreachable
   `RETREAVER_BASE_URL`) and asserts the resulting error string, log output and stored
   `last_error` contain no key.
4. Check what is **already** stored: search the Postgres integration error fields and the
   current platform-sync logs for the key pattern. Report a count only.

**B3 — "Commit hash: recorded below after commit" was never filled in.** State the R0 commit
hash in the R1 report. Do not make a commit just to record a hash.

---

## C. Amendments to the R1 design (R0.5)

**C1 — Page cap must not skip the rest of the window. 🔴**
R0.5 says "log and carry the remainder into the next window". With `order=desc` the rows
left over after the cap are the **oldest** rows of the window. The next window starts at
`end − 15m`, so those rows would **never be fetched**. That is K1 again, created by the fix.
Required instead:
- Use the `Total` header on page 1. If `Total` > `per_page × MAX_PAGES_PER_WINDOW`, **split
  the window** (halve it, recursively) before paging. Nothing is ever cut off.
- If the cap is still hit for any reason, do **not** advance the window. The cursor keeps
  `{window_start, window_end, page}` and the next tick resumes the same window.
- Test: a window whose `Total` exceeds the cap is split, and every row arrives.

**C2 — A `Total` mismatch must block, not only warn.** When rows collected ≠ `Total` for a
frozen window, do **not** advance the cursor. Retry the window, with a bounded number of
attempts and backoff. If it still mismatches, keep the cursor, mark the integration's sync
status as degraded with a clear message, and log it. Test it.

**C3 — `null` revenue/payout without a silent 0.**
- State exactly how "not reported" is stored with the current schema: ClickHouse column
  types, the content hash, and what the dashboard sums.
- If the current schema cannot represent it without a migration, **stop at that point (🙋)**
  and propose the migration. Do not invent one inside R1.
- The hash must treat `null` and `0` differently.

**C4 — Field-meaning table against Ringba.** Before mapping, write a table: Calyx field →
Ringba source → Retreaver source → meaning. It must cover at least `DurationSeconds`,
`CallLengthSeconds`, `HasConverted`, `ProfitGross`, `PlatformDuplicate`, and target/buyer
names. The dashboard shows both platforms side by side, so the same column must mean the
same thing for both. If a Retreaver field only roughly matches, say so and pick the closer
one with a reason.

**C5 — The hash test covers every newly mapped field.** Not only `converted`. For each newly
mapped hashed field: flip only that field → the hash changes → the row republishes. And the
reverse: a field **not** in the hash does not cause republishing.

**C6 — `reports_conversion_state`: keep one source of truth.**
- Define it once: per platform, in a single place that both the API filter
  (`pings.filters.ts`) and the web helper (`shouldHaveConverted`) read.
- Before this change, the "should have converted" counts for Ringba must be identical to
  the counts after it. Prove it on the local data.
- TrackDrive/CallGrid rows (demo only) must no longer be flagged.

**C7 — The downtime-gap and long-call claims move from `[INFERRED]` to tested.** Add both
tests:
1. The sync is down for several hours, then recovers → no call in the gap is missed.
2. A call whose `updated_at` moves after it ends is picked up by the next window.

**C8 — Stronger account check before R2 (Rule 7).** Before R2, the owner picks one recent
call in the Retreaver web UI and gives its UUID. The adapter fetches it through the stored
integration (`/api/v2/calls/{uuid}.json`) and reports `status`, `created_at` and
`revenue`. The owner compares them with the UI. Match = account confirmed. Report it in R1's
open questions so it is ready for R2.

**C9 — Throttle proof.** A test shows two Retreaver integrations on the **same**
`company_id` share one limiter, and different `company_id`s do not. A 429 with and without
`Retry-After` backs off correctly, and the page is retried, not skipped.

**C10 — Go version parity.** Local tests ran on Go 1.27.1, while the images build with
`golang:1.25.14`. Run the platform-sync test suite once inside `golang:1.25.14-alpine`
(`docker run … go test ./...`) and report the result.

---

## D. Unchanged from R0.5 (approved as designed)

- R1.1 frozen window `[start, end)`, cursor `{window_start, window_end, page}`, old-cursor
  migration, 15-minute overlap, `per_page` 100 with the comment about the silent fall back
  to 25
- R1.2 mapping `converted`, durations, profit, duplicate, names, status, connected
- R1.4 capability method instead of the hardcoded `"ringba"` guard in `RunWindow` and in
  the sweeps. Ringba behaviour identical; TrackDrive/CallGrid unaffected.
- Settings table, plus any setting the items above add, with its default
- Dedup-TTL vs the 30-day sweep: proof of one logical row and a single dashboard count
  (already an R1 item in the spec)

## E. Gate R1

- All three PowerShell gates exit 0.
- Every test from C1, C2, C5, C7 and C9, plus the spec's own R1 tests, passes.
- Ringba: no regression (Rule 8).
- Report in `docs/retreaver-stage4-report.md` → PHASE R1 section, including B1, B2(4) and C8.
- Commit on `stage4/retreaver`, push the branch only, give the hash. **STOP** and wait for
  `GO R2`.
