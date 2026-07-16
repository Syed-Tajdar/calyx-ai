# Phase 2.C — Ringba certification

**Date:** 2026-07-14  
**Platform:** Ringba  
**Mode after cert window:** flip integration back to **Test** (ops; one click).  
**Live credentials this run:** absent (`RINGBA_API_TOKEN` / `RINGBA_ACCOUNT_ID` not set) — live D8/D9/D10 deferred honestly.

Prerequisite shared work: Phase 2.0 content-hash + page=1 refresh; Phase 2.A gate **GO**.

---

## Definition of Done

| ID | Check | Status | Evidence |
|----|--------|--------|----------|
| D1 | Research + citations | **PASS** | `docs/apisync-platform-research.md` Ringba; `docs/phase2.A-ringba-evidence-report.md` |
| D2 | Golden fixture | **PASS** | `testdata/ringba_calls.json` (`source: docs`) |
| D3 | Conformance certified | **PASS** | `ringba` in `certifiedAdapters`; `go test ./internal/adapter/ -run Conformance` PASS |
| D4 | `occurred_at` from `callDt` | **PASS** | `mapRingbaCallLogEntry` + unit test |
| D5 | NormalizeState US+CA | **PASS** (code) | Shared allowlist; fixture Texas→`TX`. Live map pin pending real sync |
| D6 | Header-auth redaction | **PASS** | `Authorization: <token>` → `[REDACTED]` (Bearer/Basic/bare); conformance + `redact` tests |
| D7 | Empty-id guard | **PASS** | Conformance missing-id skipped; syncer skips empty |
| D8 | A1 id proof (live) | **PENDING** | Needs Live creds + fetch by `inboundCallId` / InboundCallIds detail. Script: `verify-apisync-ringba-real.ps1` |
| D8b | Deep link + Case A | **PASS** (config) | Case A: UI/API id = `inboundCallId`. Catalog `app.ringba.com/#/call/{id}` + env override. Live browser confirm pending |
| D9 | Value-change live | **PARTIAL** | Mechanism ready: **overlap 2h + content-hash**. Live 0→nonzero conversion **not** exercised this run (no creds) |
| D10 | Volume sanity | **PENDING** | Requires Live sync window |
| D11 | Verify scripts | **PARTIAL** | Adapter/unit green. Full presentation/smoke/api-sync/ingestion/ws: run when stack rebuilt with Phase 2 images |
| D12 | Phase report | **PASS** | This file + 2.0 / 2.0b / R2 / 2.A reports |

## Automated raw output

```
go test ./packages/pingdedup/...                         PASS
go test ./internal/adapter/... ./internal/redact/...     PASS (ringba + retreaver certified)
go test ./internal/syncer/...                            PASS
```

(No secrets printed.)

## Update mechanism (recorded)

| Item | Choice |
|------|--------|
| Last-mod filter | **NO** (A3) |
| Strategy | Overlap window (`last_sync − 2h`) + content-hash |
| Backfill | Default **30 days** via `initial_lookback_days` |
| `platform_duplicate` | Not mapped (no proven Ringba duplicate flag in CallLogs columns) |

## Certification verdict

**CONDITIONAL CERTIFIED** — adapter + shared machinery + docs DoD complete; **live A1/D9/D10** remain open until Ringba Live sync with real tokens.

Matrix: certify **CONDITIONAL**; flip any Live Ringba integration back to **Test** after the cert window.  
**TrackDrive Phase 3:** may start (conditional gate satisfied). Do **not** treat D9 as YES until a live conversion proof lands.

## Ops next (when creds available)

1. Settings → Ringba API Sync → **Live** (only Ringba Live).
2. `verify-apisync-ringba-real.ps1` (env tokens — never print).
3. Capture one `inboundCallId` → detail fetch **200** (D8).
4. If a conversion updates revenue in-window → re-sync → `argMax` + distinct once (D9 → YES).
5. Flip back to **Test**.
