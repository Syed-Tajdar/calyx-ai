# Phase 2 — Ringba live certification

**Date:** 2026-07-15  
**Verdict: CONDITIONAL** — code + D11 deploy proof ready; **live Ringba credentials not provided in this session**.

## Implemented (code / deploy)

| Item | Status |
|------|--------|
| `RINGBA_BASE_URL` override | DONE ([`ringba_adapter.go`](../services/platform-sync/internal/adapter/ringba_adapter.go)) |
| Dual-window throttle 5/sec + 80/min per `account_id` | DONE ([`ratelimit.go`](../services/platform-sync/internal/adapter/ratelimit.go)); unit tests PASS |
| HTTPClient `Retry-After` on 429 | DONE |
| Content-hash deploy + CH prove | DONE (see [`phase2.0-deploy-verification-report.md`](phase2.0-deploy-verification-report.md)) |
| Conformance / occurred_at / deep-link catalog | DONE (prior Phase 2.B) |

## 2.1 Bogus-token redaction

- Code: bare `Authorization:` redaction unit-tested (`TestSanitizeErrRedactsBareAuthorizationToken`).
- Script ready: [`scripts/prove-ringba-bogus-redaction.ps1`](../scripts/prove-ringba-bogus-redaction.ps1) — set compose `RINGBA_BASE_URL` to unresolvable host, run script, remove override.
- **Operator must run before pasting real token** (blocked from autonomous redirect of sync traffic this session).

## 2.2–2.5 Live DoD

Blocked on Syed pasting real `api_token` + `account_id` in Settings (never print).

Pre-existing ringba CH rows: count observed **1** on this stack (not ~912) — wipe still recommended before Live if demos accumulate.

| Check | Status |
|-------|--------|
| D8 A1 | **NO** (pending live) |
| D9 value-change | **PARTIAL** (overlap+hash proven on Retreaver Test mock; Ringba live conversion not observed) |
| D10 / D5 / D8b live | **FAIL / pending** |

## 2.6 Throttle model

Dual sliding-window limiter keyed by Ringba `account_id` (shared across integrations). Replaces 220ms sleep. Multi-page **live** backfill metrics (req/min, 429 count) pending Live run.

### 2.6b Backfill duration (estimate)

- `maxPagesPerSync=10`, `size=50` ⇒ ≤500 calls/run (+ page1 refresh).
- Sustained ≤80 req/min ⇒ ≥750ms/request average.
- Example 100k calls / 30d ≈ 200 sync ticks; at `SYNC_INTERVAL_SEC=120` ⇒ ~6.7h wall if continuous — Settings progress is status/cursor only (no progress bar). Product: dedicated backfill worker or larger page budget.

## 2.7 Close-out

- Flip Ringba → **Test** after any Live window (ops).
- Matrix: **CONDITIONAL** (not YES).

## GATE

```
Ringba D8 A1: NO
Ringba D9 live value-change: PARTIAL (mechanism proven on shared content-hash; no Ringba conversion)
Ringba D10 / D5 / D8b: FAIL (pending live)
Ringba CERTIFIED: CONDITIONAL (credentials + live DoD incomplete)
Throttle honours 5/sec AND 80/min, scoped per account: YES (code + unit); live backfill prove: NO
Backfill at scale: ~hours for 100k @ 120s tick; progress bar: NO
Ready for Phase 3 (TrackDrive): NO (Syed rule — full live verification required)
```
