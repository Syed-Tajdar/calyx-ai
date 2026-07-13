# Platform Go-Live Checklist

Use this checklist for every external pull adapter (Retreaver → Ringba → TrackDrive → CallGrid).
A platform is **CERTIFIED** only when every item is evidenced in that phase’s report.

Rollback: flip the integration back to **Test** (one click, no rebuild).

## Definition of Done

| ID | Check | Evidence |
|----|--------|----------|
| D1 | Research current: endpoint, auth, response shape, timestamp field, state field, revenue/payout type, pagination, incremental/cursor, rate limits — each with a doc citation in `docs/apisync-platform-research.md` | |
| D2 | Golden fixture committed under `services/platform-sync/internal/adapter/testdata/{platform}_calls.json`, marked `source: docs` or `source: real-account (scrubbed)` | |
| D3 | Adapter passes conformance suite; moved `pending` → `certified` in `conformance_test.go` | |
| D4 | `occurred_at` mapped from the platform’s call timestamp (or documented as absent) | |
| D5 | `NormalizeState` wired; a real call’s state lands 2-letter on the coverage map | |
| D6 | Credentials never leak: transport-error test shows `[REDACTED]`, key count 0 in logs and `last_sync_error`. Header-auth platforms: no request/header object logged | |
| D7 | Empty-id guard: malformed page → loud error + zero published rows (no silent empties / fake ids) | |
| D8 | **A1 proof**: take one stored `external_call_id`, fetch that call from the platform’s own API by id → HTTP 200 with matching fields | |
| D9 | Update semantics: re-sync after revenue changes → dashboard shows **new** revenue; call counted once | |
| D10 | Volume sanity: our distinct call count for a window is same order of magnitude as the platform’s count; gaps explained | |
| D11 | Verify scripts green: `verify-presentation` (20/20), `smoke-test-all-platforms` (4/4), `verify-api-sync`, `verify-ingestion-slice`, `ws-smoke-test` | |
| D12 | Phase report written with **raw** output; credentials never printed | |

## Per-platform A1 detail endpoints

Record / confirm in `docs/apisync-platform-research.md`:

| Platform | Detail-by-id endpoint |
|----------|----------------------|
| Retreaver | `GET /api/v2/calls/{uuid}.json` |
| Ringba | (confirm in Phase 2 research) |
| TrackDrive | (confirm in Phase 3 research) |
| CallGrid | (confirm in Phase 4 research) |

## Rules

- Only **one** platform Live at a time; others stay on Test.
- Shared fixes → re-run conformance for every already-certified adapter.
- Never print `api_key` / `company_id` / tokens in reports or chat.

## Phase 2 first task (do not forget)

**Content-hash Redis dedup is a prerequisite for any `overlap` strategy** (Ringba / TrackDrive / CallGrid likely lack last-modified filters).

Naive overlap explodes ClickHouse rows. Fix before enabling overlap:

```
Redis key holds a CONTENT HASH of the mapped ping, not just presence:
  key absent       → new call    → publish, is_duplicate = 0
  hash unchanged   → no-op       → DO NOT publish
  hash changed     → an UPDATE   → publish, is_duplicate = 0
```

See Phase 1.6 plan / `docs/phase1.6-official-evidence-plan.md`.
