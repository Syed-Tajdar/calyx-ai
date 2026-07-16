# Phase 2.B — Ringba adapter report

**Date:** 2026-07-14  
**Status:** PASS (adapter + conformance)

| Item | Detail |
|------|--------|
| `OccurredAt` | Mapped from column/tag `callDt` (fallbacks: `callDateTime`, `callDate`) |
| Cursor | Overlap `last_sync − 2h`; empty cursor lookback **30d** (or `initial_lookback_days`); offset pagination; ~220ms spacing + 429 retries |
| Auth | `Authorization: {api_access_token}` (no Bearer) |
| Fixture | `testdata/ringba_calls.json` (`source: docs`) |
| Conformance | `ringba` → `certifiedAdapters`; `occurredAtRequired["ringba"]=true` |
| Deep link | `shared/providers/platform-deep-links.json` + web default `https://app.ringba.com/#/call/{id}` |
| Redaction | Bare `Authorization:` token redacted |
