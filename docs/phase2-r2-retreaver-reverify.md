# R2 — Retreaver re-verify after content-hash

**Date:** 2026-07-14  
**Verdict:** Retreaver remains **CERTIFIED**

## Automated

| Check | Result |
|-------|--------|
| `go test ./internal/adapter/ -run Conformance` | PASS (retreaver still in `certifiedAdapters`) |
| `go test ./internal/adapter/ -run RegistryMembership` | PASS |
| Content-hash syncer tests | PASS |

## Cert matrix

`docs/platform-certification.md` — Retreaver row unchanged: **YES** CERTIFIED.

Post-2.0 notes:

- Unchanged re-syncs no longer emit duplicate Kafka/CH rows (`is_duplicate` presence-only path retired for new traffic).
- Deep-page limitation mitigated by page=1 refresh + content-hash.
- Privacy follow-ups (aniHash URL, hide apisync) shipped in 2.0b — no impact on Retreaver DoD.

## Manual / live scripts

Full live Test-mode sync + presentation/smoke optional when stack is up. Conformance + shared machinery green is sufficient to keep CERTIFIED after the shared dedup change.

**No `api_key=` leaks** — existing redact conformance still required for certified adapters.
