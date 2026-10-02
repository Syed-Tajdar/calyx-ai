# Calyx — Stage 1–3 fix round 2 (owner review of `stage1-3-fix-report.md`) + merge gate

**Branch:** `stage4/retreaver` · **Operator:** Talha · **Owner:** Syed
**Output:** append a "Round 2" section to `docs/stage1-3-fix-report.md`

The first fix round is **accepted**: A1–A6, B1–B8, C1–C4, with gates and console clean. The
items below close what the report itself flagged plus three things the review found. After
this round the branch merges to `main`.

Same rules as `docs/stage1-3-fix-spec.md`:
- verify before changing, and prove every item
- one commit per item, prefix `fix(stage1-3-r2)`
- no Live sync, no live Convert/Cancel
- gates run with `pwsh-preview`
- push the branch only, never `main`
- stop at every 🙋

---

## R2-1 🔴 Kill switch: prove the server path, not just the code

The report says the write-back routes are now mounted when `WRITEBACK_INTERNAL_TOKEN` is set,
and `SetGlobalEnabled(false)` makes the service refuse. The audit said the routes were mounted
only when `WRITEBACK_ENABLED=true`. This changed. On the server `WRITEBACK_ENABLED=false` while
the token **is** set, so this is the exact production case.
1. Show with file:line where `WRITEBACK_ENABLED` is read in **api** and in **platform-sync**,
   and where it drives `SetGlobalEnabled`.
2. Run the stack locally with `WRITEBACK_ENABLED=false` **and** the token set (the server's
   config). Send a convert and a void as an admin, with a valid CSRF token and a fake call id.
   Prove three things:
   - The response is a refusal.
   - **No** request reaches Ringba. Show the platform-sync log or a counter.
   - An audit row says `rejected` / globally disabled.
3. Add a test for exactly this combination. Restore the local compose afterwards.

## R2-2 🟡 Pause/resume and old soft-deleted rows

- Show that `list_api_sync_integrations()` (and the sweeps) skip both **paused**
  (`is_active=false`) and **deleted** (`deleted_at` set) integrations. Use the same rolled-back
  SQL test style as Stage 4 P2.
- Show that a deleted integration cannot be resumed (API + UI).
- Pause and resume must write an audit or settings-history row (who, when, which
  integration, which action). The spec asked for this, but the report does not show it. Add
  it if it is missing.
- **Server risk:** before migration `000029`, every integration that was deleted in the past
  looks exactly like a paused one: `is_active=false` with no `deleted_at`. After deploy they
  would **reappear in Settings as "Paused"**, and someone could resume one with stale
  credentials. Add to the server deploy note a **read-only query** for Aryan to run **before**
  `000029`. It lists every row with `is_active=false`: tenant, platform, name and
  `updated_at` only, no secrets. Then 🙋 the owner decides which ones get `deleted_at`. Do
  **not** write a migration that guesses.

## R2-3 🟡 Dashboard ignores the platform filter

"Found, not fixed": Total Pings, revenue KPIs and the ping chart do not apply the platform
dropdown (`/analytics/metrics` and `/analytics/ping-series` take no `platform`). With Ringba
and Retreaver both live, choosing "Retreaver" would still show both. Fix:
- Add `platform` to both endpoints and pass it from the dashboard.
- Coverage should also follow the platform, if it does not already.
- Proof: on a database with rows from both platforms (create a few `smoke-` Ringba rows if
  needed, then delete them), All = Ringba + Retreaver for calls and revenue, and each
  platform matches ClickHouse.

## R2-4 🟡 Transcript and Audience Enrichment show scripted sample data

"Found, not fixed": with the scoring container running, a Retreaver call showed **scripted
transcript lines**. That is the same problem as B4: mock data that looks real. Hide the
Transcript and Audience Enrichment panels behind the same v1 flag. Screenshot the call
detail afterwards.

## R2-5 🟡 Rate limiter: 81 requests against an 80/min cap

During this pass `TestDualWindowLimiterHonoursPerMinuteOnSustainedWalk` saw **81** requests in
a rolling 60 s against a cap of 80. It was called a load flake, but this limiter is what keeps
the live Ringba account under its per-account limit.
1. Find out whether the test's measurement window and the limiter's window differ (test
   artefact), or whether the limiter really allowed 81 (bug). Show the reasoning with
   timestamps.
2. If it is the limiter: fix it.
3. In both cases, give the real calls a safety margin. The default Ringba per-minute budget
   becomes **75** (configurable), so wall-clock jitter cannot reach Ringba's 80. Run the test
   10 times under load (build running) and report the max rolling count.

## R2-6 🟢 Gates must pass in the v1 configuration

`verify-system` failed until the six intelligence containers were started, and
`verify-presentation` checks `POST /ask`. Production v1 runs **without** those services, so
the gates must pass in the v1 setup.
- Add a v1 mode (for example `-V1`, or read `INTELLIGENCE_SCALE=0`) that skips the
  intelligence containers, their health checks and `/ask`.
- Run all three gates in v1 mode, with the intelligence services stopped, and show PASS.
- The full mode still exists for later.

## R2-7 🟢 Where do "Converted · $0" calls live?

A `converted_zero` call appears in neither the Converted nor the Not converted filter, so a
user cannot list them. Add a filter chip **"Converted · $0"**, or include them under Converted
with the muted badge. Pick one, state why, and show the counts adding up to the total.

---

## Merge gate (after R2-1 … R2-7)

- All three gates PASS in **v1 mode** and in **full mode**.
- Retreaver all-time unchanged: **3,652** calls / **$33,143.85**.
- Secret scan of every commit since `b5f3dc4`.
- Updated server deploy note:
  1. The R2-2 query, run **before** `000029`
  2. Migrations `000028` → `000029`, in order
  3. Rebuild `api`, `web` and `platform-sync`
  4. New env: `WRITEBACK_SETTLE_SECONDS`, plus the R2-5 limit
  5. `WRITEBACK_ENABLED` stays `false`
- Push the branch, give the hashes, **STOP**. The owner then says merge.

## Part E (live $1 Convert + Cancel): **not a merge blocker**

Write-back stays **off** on the server (`WRITEBACK_ENABLED=false`), so merging does not expose
the money path. Part E becomes the gate for **turning write-back on** for any tenant. It runs
later, on the Ringba sandbox call, with the owner's approval, and only from one place, so
Ringba is not live-synced from two machines (Rule 10).
