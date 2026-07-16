# Platform Certification Status

Living status for the sequential rollout (Retreaver → Ringba → TrackDrive → CallGrid).
Updated after **Retreaver FINAL certification** (`docs/retreaver-final-certification-report.md`).

| Platform | Research | Fixture | Conformance | occurred_at | Redaction | A1 proof | D9 SQL | D9 API | Update mechanism | Initial backfill | D8b | Certified |
|----------|----------|---------|-------------|-------------|-----------|----------|--------|--------|------------------|------------------|-----|-----------|
| Retreaver | ✅ | real-account | ✅ | ✅ | ✅ | ✅ | ✅ | **YES** (0→13.51 live) | `updated_at` + 15m safety delta; revenue bumps `updated_at` **observed YES** | default **30 days**, per-integration + Settings UI + Backfill | ✅ Case A (truncated list / full detail+URL); exact+prefix search; deep link | **YES** |
| Ringba | ✅ Phase 2.A | docs | ✅ | ✅ `callDt` | ✅ header token | ⬜ pending live | ✅ (argMax shared) | **PARTIAL** (overlap+hash ready; no live 0→rev) | **overlap 2h + content-hash** (A3=NO last-mod) | default **30 days** + Settings lookback | ✅ Case A config (`inboundCallId`); deep link best-effort / env | **CONDITIONAL** |
| TrackDrive | ⬜ Phase 3 | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| CallGrid | ⬜ | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| Call Loom | n/a | — | — | — | — | — | — | — | n/a | n/a | — | ⬜ |

### D9 levels (Retreaver)

| Level | Meaning | Status |
|-------|---------|--------|
| **SQL-level** | Aggregates `argMax` without filtering `is_duplicate` | ✅ |
| **API-level** | Live revenue 0→nonzero re-fetched; metrics `argMax`; counted once | ✅ **YES** — call `c525ed11-…` 0→**13.51**; `updated_at` moved; see final certification report |

### Observed: does revenue bump `updated_at`?

**YES** (Retreaver, live call). Pattern validated for Retreaver only — other platforms need their own evidence.

### Product follow-ups (privacy / polish)

1. **Call-detail URL:** drop `aniHash` query param — **done** Phase 2.0b (`docs/phase2.0b-followups-report.md`).
2. **Ping Feed:** default-hide `apisync-%` + **Show demo** — **done** Phase 2.0b.

### Coverage — North America (US + Canada) — shipped 2026-07-14

NormalizeState allowlists US+CA; Coverage map + honest “regions · US + Canada” header; Ping Feed headline = distinct calls. See [`docs/north-america-coverage-report.md`](north-america-coverage-report.md) and [`docs/coverage-north-america-ui-report.md`](coverage-north-america-ui-report.md).

### Known limitation — deep-page backfill

During **initial backfill**, the sync cursor walks `page` forward on a wide window. Concurrent updates may be delayed until the walk catches up.

- **Mitigation (Phase 2.0):** content-hash Redis dedup + end-of-deep-walk **`page=1` refresh** (`docs/phase2.0-content-hash-dedup-report.md`).
- Unchanged hashes publish zero rows; changed content republishes as update rows.

### Phase 2 progress

| Slice | Status |
|-------|--------|
| 2.0 content-hash | **DONE** — deployed + CH row-level proven (`docs/phase2.0-deploy-verification-report.md`) |
| 2.0b follow-ups | **DONE** |
| R2 Retreaver re-verify | **DONE** on running system (CERTIFIED YES; `api_key=` log count 0) |
| 2.A Ringba evidence | **GO** |
| 2.B Ringba adapter | **DONE** |
| 2.C / Live Ringba DoD | **BLOCKED** — needs real Settings credentials (`docs/phase2-ringba-live-certified.md`) |
| Enterprise risks (PART 3) | **REPORTED** investigate-only (`docs/phase2-enterprise-risks-report.md`) |

Retreaver remains **CERTIFIED**. Ringba stays **CONDITIONAL** until Live D8–D8b. **TrackDrive blocked.**

### Phase 1.6 notes (historical)

- `platform_duplicate` rule **(b)** + `duplicate_rate` remain in effect.
- Prior PARTIAL D9 superseded by this final report.
