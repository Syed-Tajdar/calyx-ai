# Calyx — Pre-Phase-2b: Source Integrity + Approved Cleanup

**Scope:** confirm the repo source is complete and buildable, rebuild every app image from
it, and execute the two deletions the operator has approved. No Phase 2. No sync live.

**Deliverable:** `docs/pre-phase2b-report.md`

## ⛔ RULE
Do exactly this. If a file is found truncated or anything differs from this document,
STOP and report — do not reconstruct code from memory. Do not start Phase 2.

---

## Why

Pre-Phase-2 found two services whose **source did not compile**:
- `services/platform-sync/internal/config/config.go` — `Load()` was missing `}, nil` and
  its closing brace. Restored by hand.
- API — `CallDeepViewPing.has_converted` declared but unmapped. Fixed by hand.

The running containers were built from code that no longer matched the repo. A missing
function tail suggests truncation, not a typo — other lines may be gone too.

---

## 1 — Git integrity check (report before rebuilding)

1. `git status` — list every modified, deleted and untracked file. Report them all.
2. For `config.go`: `git log -p` / `git diff` against the last commit where it compiled.
   Report the full diff. **Question to answer:** besides the restored `}, nil` and brace,
   are any other lines missing — config fields, env reads (`WRITEBACK_ENABLED`,
   `WRITEBACK_INTERNAL_TOKEN`, DB/Redis settings), or defaults?
3. Same diff check for the API deep-view file that needed `has_converted`.
4. Report whether uncommitted work exists that the running images depended on.

**If any other lines are missing, STOP.** Do not re-type them. Report the diff and wait.

---

## 2 — Rebuild every app image from current source

```
cd deploy
docker compose build api platform-sync web consumer ingestion
docker compose up -d --force-recreate api platform-sync web consumer ingestion
```

Report that each image built. A build failure is a finding — report it, don't patch
around it without saying so.

## 3 — Re-prove the write path on the rebuilt platform-sync

Same smokes as Phase 1.1 / 1.2 — **no Ringba write**:

| Probe | Expected |
|---|---|
| `POST :8088/writeback/convert`, no Authorization | 401 |
| wrong Bearer | 401 |
| valid Bearer, empty JSON | 400 (handler reached) |
| `printenv WRITEBACK_ENABLED` in platform-sync | `true` |

These confirm the rebuilt binary — from the repaired source — still enforces auth and
reads its config.

## 4 — Web live-feed filter

With the rebuilt web image, confirm the browser live-feed filter
(`web/src/lib/use-hybrid-ping-feed.ts`) excludes `apisync-`, `demo-`, `smoke-`.

## 5 — Execute the approved deletions (operator-approved)

Run **exactly** these two statements and nothing else:

```sql
ALTER TABLE default.pings DELETE WHERE external_call_id IN (
  '0e92eece-73fb-4d06-b9c7-858862aca994',
  'ea023ec0-927b-43b9-af44-8befd5afe113',
  'ba472c0e-3212-4013-a760-c197b3877617',
  'cb51ea8b-6234-41cc-8697-3aad6729381a',
  'dbbdef75-8c22-4b08-bd82-5bdbb1bd08fe',
  '128fb18f-3738-4ddc-9b51-dd367b302778',
  '1906f701-2bdb-4b56-9f9c-f5dca66334af',
  '195c1e62-d264-4d6f-a9e1-a28347152e3f',
  '7afabab6-90d8-4a14-a255-4b648ded24b1',
  'cb45766e-04d7-4510-b117-075bbcb25305',
  'db04ac18-cce8-44de-ae41-1b9b896c8856',
  'f2b1d903-b545-4b14-8df6-dc6154060154',
  '1f149b5f-b772-4216-b397-fd7d8e65f920',
  'b9ff1104-aa1f-4abd-9a7a-00f69f68e240'
);

ALTER TABLE default.pings DELETE WHERE external_call_id LIKE 'apisync-%';
```

**Before running**, re-count each set and confirm it still matches: 15 rows and 3 rows.
If either count differs, STOP — do not run.

ClickHouse mutations are async. After running, poll until complete and report:

| Family | Before | After |
|---|---|---|
| old smoke (14 ids) | 15 | 0 |
| `apisync-%` | 3 | 0 |
| `demo-%` | 53 | 53 (untouched) |
| `smoke-%` | | (untouched) |
| Real Ringba `RGB%` | 4100 | 4100 (untouched) |

And default KPIs before/after — `total_pings` should drop by the 14 old-smoke calls;
revenue by the old-smoke `argMax` amount (~$195.00).

## 6 — Gates

```
scripts/verify-system.ps1            → exit 0, Docker 15/15, Healthz 11/11
scripts/verify-ingestion-slice.ps1   → exit 0
scripts/verify-presentation.ps1      → exit 0
```

## 7 — Commit

Once green, commit all changes from Pre-Phase-2 and this file, so the repo and running
images match. Report the commit hash and confirm `git status` is clean.

## Out of scope
Phase 2 · turning sync live · Retreaver re-sync · any write-back behaviour change ·
any deletion other than the two statements above
