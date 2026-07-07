---
name: Platform REST Adapters
overview: Research all four webhook platforms' Calls APIs (documented first), refactor platform-sync behind a shared adapter interface, then implement real REST pull adapters one at a time (Retreaver → Ringba → TrackDrive → CallGrid) with per-platform verification. Call Loom stays a direct source; webhook path unchanged.
todos:
  - id: step0-research-doc
    content: "Write docs/apisync-platform-research.md with verified per-platform table (sources cited)"
    status: pending
  - id: shared-framework
    content: "Refactor platform-sync: PlatformAdapter interface, http client, cursor helpers, extend RemoteCall + migration 000020 sync_status/error"
    status: pending
  - id: dynamic-cred-ui
    content: "API + Web: per-platform credential fields, real testConnection via platform-sync TestAuth"
    status: pending
  - id: adapter-retreaver
    content: "Implement Retreaver adapter + unit tests + docs/apisync-retreaver-report.md + verify script"
    status: pending
  - id: adapter-ringba
    content: "Implement Ringba adapter (CallLogs/Date) + verify after Retreaver passes"
    status: pending
  - id: adapter-trackdrive
    content: "Implement TrackDrive adapter (subdomain + Basic auth) + verify"
    status: pending
  - id: adapter-callgrid
    content: "Implement CallGrid adapter (cursor pagination) + verify"
    status: pending
isProject: false
---

# Platform REST Adapters

**Canonical human copy for verify/cross-check:** [docs/apisync-platform-research-plan.md](../../docs/apisync-platform-research-plan.md)

When implementing, read the full plan in `docs/apisync-platform-research-plan.md` — it contains STEP 0 research table, shared framework, build order, and per-slice checklists.
