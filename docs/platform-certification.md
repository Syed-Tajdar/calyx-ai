# Platform Certification Status

Living status for the sequential rollout (Retreaver → Ringba → TrackDrive → CallGrid).
Updated after **Retreaver FINAL certification** (`docs/retreaver-final-certification-report.md`).

| Platform | Research | Fixture | Conformance | occurred_at | Redaction | A1 proof | D9 SQL | D9 API | Update mechanism | Initial backfill | D8b | Certified |
|----------|----------|---------|-------------|-------------|-----------|----------|--------|--------|------------------|------------------|-----|-----------|
| Retreaver | ✅ | real-account | ✅ | ✅ | ✅ | ✅ | ✅ | **YES** (0→13.51 live) | `updated_at` + 15m safety delta; revenue bumps `updated_at` **observed YES** | default **30 days**, per-integration + Settings UI + Backfill | ✅ Case A (truncated list / full detail+URL); exact+prefix search; deep link | **YES** |
| Ringba | ⬜ Phase 2 | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (own last-mod answer; likely overlap + content-hash) | pending | ⬜ | ⬜ |
| TrackDrive | ⬜ | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| CallGrid | ⬜ | docs | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | pending (likely overlap) | pending | ⬜ | ⬜ |
| Call Loom | n/a | — | — | — | — | — | — | — | n/a | n/a | — | ⬜ |

### D9 levels (Retreaver)

| Level | Meaning | Status |
|-------|---------|--------|
| **SQL-level** | Aggregates `argMax` without filtering `is_duplicate` | ✅ |
| **API-level** | Live revenue 0→nonzero re-fetched; metrics `argMax`; counted once | ✅ **YES** — call `c525ed11-…` 0→**13.51**; `updated_at` moved; see final certification report |

### Observed: does revenue bump `updated_at`?

**YES** (Retreaver, live call). Pattern validated for Retreaver only — other platforms need their own evidence.

### Known limitation (not a blocker) — deep-page backfill

During **initial backfill**, the sync cursor walks `page` forward on a wide `updated_at_start` window (`maxPagesPerSync` batches). Concurrent updates to calls that already fall behind the active deep page may be **delayed until backfill completes** (or until `page` resets to a window that includes them again).

- **Not data loss** — the change remains on the platform and is picked up once the walk catches up / a recent `page=1` window is used.
- **Does not block** Retreaver CERTIFIED (D9 value-change was proven by resetting to a recent window).
- **Phase 2 follow-up (do not implement now):** address with **content-hash Redis dedup** + periodic **`page=1` refresh** (or equivalent) so concurrent updates are not starved by a deep page walk.

### Phase 2 (Ringba)

**Ready: YES.** First tasks: **content-hash Redis dedup** (prerequisite for any `overlap` strategy) and the deep-page backfill refresh above. Retreaver stays on **Test** when not actively verifying.

### Phase 1.6 notes (historical)

- `platform_duplicate` rule **(b)** + `duplicate_rate` remain in effect.
- Prior PARTIAL D9 superseded by this final report.
