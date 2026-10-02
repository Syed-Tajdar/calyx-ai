# Platform Certification Status

Living status for the sequential rollout (Retreaver → Ringba → TrackDrive → CallGrid).
Updated after **Ringba live certification** (`docs/phase2-ringba-live-certified.md`, 2026-07-20).

| Platform | Research | Fixture | Conformance | occurred_at | Redaction | A1 proof | D9 SQL | D9 API | Update mechanism | Initial backfill | D8b | Certified |
|----------|----------|---------|-------------|-------------|-----------|----------|--------|--------|------------------|------------------|-----|-----------|
| Retreaver | ✅ | real-account | ✅ | ✅ `created_at` (UI clock is `start_time`) | ✅ | ✅ | ✅ | **YES** — earlier `c525ed11-…` 0→13.51; Stage 4 R2 `1c037735…` 0→**34.13**, `sync_count` 2 | `updated_at` + 15m safety delta; revenue bumps `updated_at` **observed YES**; hourly 7-day sweep **proven live** 2026-10-02 | default **30 days** (Stage 4: 30 days UTC, 3,493 then incremental) | ✅ Case A (truncated list / full detail+URL); exact+prefix search; deep link | **YES** — Stage 4 R2 re-proven, then returned to Test |
| Ringba | ✅ Phase 2.A | docs + live | ✅ | ✅ `callDt` | ✅ `Token` header | ✅ live Get Call Log exact id (`…/calllogs/details` **404** on host) | ✅ | **YES** (0→**34.75** live) | **overlap 2h + content-hash** (A3=NO last-mod) | default **30 days** + Settings lookback | ✅ Case A (`inboundCallId`); two-way ID **operator-confirmed**; deep link = **report-level** (no per-call URL) | **YES** |
| TrackDrive | ⬜ Phase 3 | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| CallGrid | ⬜ | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| Call Loom | n/a | — | — | — | — | — | — | — | n/a | n/a | — | ⬜ |

### D9 levels

| Platform | Level | Status |
|----------|-------|--------|
| Retreaver | API-level 0→nonzero | ✅ **YES** — Stage 4 R2 `1c037735-…` 0→**34.13**, collapsed `/pings` `syncs=2`. Earlier `c525ed11-…` 0→**13.51** stands |
| Ringba | API-level 0→nonzero | ✅ **YES** — `RGB6044D6A54…RGB9B02` 0→**34.75**; `argMax` + distinct=1 |

### Observed: does revenue bump last-mod?

| Platform | Result |
|----------|--------|
| Retreaver | **YES** — revenue bumps `updated_at` |
| Ringba | **N/A (A3=NO)** — no last-mod filter; overlap + content-hash re-fetch caught 0→revenue |

### Product follow-ups (privacy / polish)

1. **Call-detail URL:** drop `aniHash` query param — **done** Phase 2.0b (`docs/phase2.0b-followups-report.md`).
2. **Ping Feed:** default-hide `apisync-%` + **Show demo** — **done** Phase 2.0b.

### Coverage — North America (US + Canada) — shipped 2026-07-14

NormalizeState allowlists US+CA; Coverage map + honest “regions · US + Canada” header; Ping Feed headline = distinct calls. Live Ringba states **WA / TX / LA** confirmed 2-letter on `GET /coverage/heatmap` (D5).

### Known limitation — deep-page backfill

During **initial backfill**, the sync cursor walks pages forward on a wide window. Concurrent updates may be delayed until the walk catches up.

- **Mitigation (Phase 2.0):** content-hash Redis dedup + end-of-deep-walk **`page=1` refresh** (`docs/phase2.0-content-hash-dedup-report.md`).
- Unchanged hashes publish zero rows; changed content republishes as update rows.
- Ringba live page size **1000** (official max) reduces page count vs older size=50.

### Phase 2 progress

| Slice | Status |
|-------|--------|
| 2.0 content-hash | **DONE** |
| 2.0b follow-ups | **DONE** |
| R2 Retreaver re-verify | **DONE** — CERTIFIED YES |
| 2.A Ringba evidence | **GO** |
| 2.B Ringba adapter | **DONE** — official `/v2/…/calllogs` |
| 2.C / Live Ringba DoD | **DONE** — **CERTIFIED YES** (`docs/phase2-ringba-live-certified.md`) |
| Enterprise risks (PART 3) | **REPORTED** investigate-only |

Retreaver **CERTIFIED** (Stage 4 R2, 2026-10-02, then returned to Test and paused). Ringba **CERTIFIED YES**. **TrackDrive Phase 3 may start.**

### Stage 4 R2 close (2026-10-02)

- Backfill window `[2026-08-31T23:38:52Z, 2026-09-30T23:38:52Z)` re-read: API `Total` 3,477, revenue sum **$31,722.09**, ClickHouse latest on the same UUIDs **$31,722.09** (sub-cent float residue $0.0001).
- Dashboard All time / Retreaver: **$33,143.85** and **3,652** calls. That is latest revenue with `platform_duplicate=0`. Duplicate-lead revenue **$420.53** is excluded. The rest of the gap vs the frozen window is 175 calls whose `updated_at` is outside that window (the D9/void/adjustment rows plus calls ingested after the backfill).
- Hourly sweep proven: `reason=sweep`, window `2026-09-25T15:27:58Z` → `2026-10-02T15:27:58Z`, 471 calls, 470 unchanged, 1 published (`2b73929f…`, status `forwarded`).
- Seven voids: API `converted=false` and `revenue=null`. ClickHouse latest `has_converted=0`, revenue 0. Not a C3 violation. R3 must not assume Retreaver keeps `converted=true` after a void; the earning signal is `converted AND revenue > 0`.
- Integration returned to **Test** (`sync_mode=mock`) and **paused** (`is_active=false`). Staging has no Retreaver API integration for company `…5270`.

### Ringba live notes (2026-07-20)

- Auth: `Authorization: Token {apiToken}` (not Bearer / bare token).
- Endpoint: `POST /v2/{accountId}/calllogs` — legacy `CallLogs/Date` and `calllogs/details` both **404/301** on current host; use Get Call Log.
- Deep link: **report-level** `https://app.ringba.com/#/dashboard/call-logs/report/new` — no per-call URL / no inboundCallId query param (operator-confirmed).
- Throttle live: ~31 Get Logs req/hour at 120s tick, **0** true 429s, rolling-60s peak ≪ 80.
- Flipped back to **Test** (`sync_mode=mock`) after cert.
