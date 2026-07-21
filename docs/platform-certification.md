# Platform Certification Status

Living status for the sequential rollout (Retreaver → Ringba → TrackDrive → CallGrid).
Updated after **Ringba live certification** (`docs/phase2-ringba-live-certified.md`, 2026-07-20).

| Platform | Research | Fixture | Conformance | occurred_at | Redaction | A1 proof | D9 SQL | D9 API | Update mechanism | Initial backfill | D8b | Certified |
|----------|----------|---------|-------------|-------------|-----------|----------|--------|--------|------------------|------------------|-----|-----------|
| Retreaver | ✅ | real-account | ✅ | ✅ | ✅ | ✅ | ✅ | **YES** (0→13.51 live) | `updated_at` + 15m safety delta; revenue bumps `updated_at` **observed YES** | default **30 days**, per-integration + Settings UI + Backfill | ✅ Case A (truncated list / full detail+URL); exact+prefix search; deep link | **YES** |
| Ringba | ✅ Phase 2.A | docs + live | ✅ | ✅ `callDt` | ✅ `Token` header | ✅ live Get Call Log exact id (`…/calllogs/details` **404** on host) | ✅ | **YES** (0→**34.75** live) | **overlap 2h + content-hash** (A3=NO last-mod) | default **30 days** + Settings lookback | ✅ Case A (`inboundCallId`); two-way ID **operator-confirmed**; deep link = **report-level** (no per-call URL) | **YES** |
| TrackDrive | ⬜ Phase 3 | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| CallGrid | ⬜ | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| Call Loom | n/a | — | — | — | — | — | — | — | n/a | n/a | — | ⬜ |

### D9 levels

| Platform | Level | Status |
|----------|-------|--------|
| Retreaver | API-level 0→nonzero | ✅ **YES** — `c525ed11-…` 0→**13.51** |
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

Retreaver **CERTIFIED**. Ringba **CERTIFIED YES**. **TrackDrive Phase 3 may start.**

### Ringba live notes (2026-07-20)

- Auth: `Authorization: Token {apiToken}` (not Bearer / bare token).
- Endpoint: `POST /v2/{accountId}/calllogs` — legacy `CallLogs/Date` and `calllogs/details` both **404/301** on current host; use Get Call Log.
- Deep link: **report-level** `https://app.ringba.com/#/dashboard/call-logs/report/new` — no per-call URL / no inboundCallId query param (operator-confirmed).
- Throttle live: ~31 Get Logs req/hour at 120s tick, **0** true 429s, rolling-60s peak ≪ 80.
- Flipped back to **Test** (`sync_mode=mock`) after cert.
