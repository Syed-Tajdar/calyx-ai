# Retreaver API D9 — Plan (canonical)

See also: `.cursor/plans/retreaver_api_d9_54092ada.plan.md`

## Goal

API-only: `updated_at` cursor so late revenue re-fetches; empty-cursor lookback **30 days**; prove API-level D9. No webhook in this slice.

## Implement

1. Live-probe `updated_at_start` (gate).
2. Cursor: `updated_at_start` + `page`; migrate old `created_at_start`.
3. Query: `updated_at_start`, `sort_by=updated_at`.
4. Empty cursor: `now − 30d`.
5. Unit tests + conformance.
6. Deploy platform-sync; Live proof; flip Test; report + certification.

## Out of scope

Webhook, `platform_duplicate` rule, other platforms.
