# Phase 1.6 — Official evidence → locked decisions → Retreaver CERTIFIED

**Canonical plan** (Cursor pointer: `.cursor/plans/phase1.6_official_evidence.plan.md`).

**Goal:** Replace assumptions with official/live evidence, lock three decisions, implement only what Retreaver needs, close D9 API-level (honest value-change) and D8b, then certify before Phase 2 Ringba.

**Constraints:** Retreaver stays **Test** except B5 Live syncs. No credentials in any output. Probes read-only. Only Retreaver adapter mapping changes in Part B. Stop at Decision Gate if Part A contradicts pre-committed conditions.

## Already in tree (verify + close gaps)

- `updated_at_start` + `sort_by=updated_at` in `services/platform-sync/internal/adapter/retreaver_adapter.go`
- Hardcoded `retreaverInitialLookback = 30 * 24 * time.Hour` (was 24h) — A4 documents current behaviour before B3
- End-of-window / `lastSyncAt` uses **5m** rewind, not Decision 1’s **15m** safety delta
- `maxPagesPerSync = 10` in `registry.go`
- Existing `docs/retreaver-api-d9-proof-report.md` proves **re-fetch MECHANISM only** (56.46 × 3) — **not** value-change; do not treat as Phase 1.6 D9 API YES
- Settings UI: Test/Live + Sync only — no lookback label

## PART A — Evidence only

See playbook sections A1–A5. **A4 (updated):** empty-cursor window is **30 days hardcoded**, not 24h. Report before changing. Checklist: (1) cursor advances on `updated_at` correctly; (2) rate limits vs `maxPagesPerSync=10`; (3) per-integration configurable + Settings UI.

Deliverable: `docs/phase1.6-evidence-report.md` → Decision Gate.

## DECISION GATE (pre-committed if evidence matches)

1. A2 YES → `updated_at` cursor + 15m safety delta; else overlap + content-hash prerequisite (not this phase). Webhooks OOS.
2. A3 populates → rule (b): include in ping counts, exclude from revenue, add `duplicate_rate`.
3. Backfill: explicit per-integration, default 30d, configurable, rate-limited, UI-surfaced. Explicit Backfill action.

## PART B — Retreaver only

- **B1** cursor_mode + safety delta; max seen `updated_at` − delta
- **B2** platform_duplicate aggregates if A3 YES
- **B3** `initial_lookback_days` + Backfill action + Settings “Syncing last N days”
- **B4** D8b per A5 Case A/B → **Case A selected** (see [phase1.6-b4-d8b-report.md](./phase1.6-b4-d8b-report.md)); Case B `platform_display_id` skipped pending Syed screenshot
- **B5** D9 value-change: docs on revenue→`updated_at`; live 0→nonzero if possible; else **PARTIAL** — “re-fetch proven, revenue-bump assumption UNVERIFIED”. Never claim API-level YES without observed revenue bump.

## PART C — Certify

Update `docs/platform-certification.md` honestly. Re-run verify/smoke/ws. Scoreboard includes D9 PARTIAL wording if applicable.

**Phase 2 note (record only):** content-hash Redis dedup before any `overlap` strategy.

## Out of scope

- Other platform adapter mapping changes
- Webhook conversion / call_key decrypt flow
- Content-hash dedup implementation
- Progress UI for backfill (follow-up if expensive)
