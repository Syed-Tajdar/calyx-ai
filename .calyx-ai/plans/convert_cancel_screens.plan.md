---
name: Convert / Cancel Screens (Ringba)
overview: "Canonical plan lives in docs/convert-cancel-screens-plan.md — UI for the already-proven Ringba write-back path. Status column, 'should have converted' filter, call detail Convert/Cancel with confirmation. Ringba only; Retreaver out of scope."
todos:
  - id: b1-has-converted-on-pings
    content: Add has_converted / duration_seconds / required_duration_seconds to PING_SELECT + PingRecord
    status: pending
  - id: b2-conversion-filter
    content: conversion filter (converted / unconverted / should_convert) on /pings and /pings/export
    status: pending
  - id: b3-writeback-enabled-flag
    content: Expose writeback_enabled on GET /integrations so the UI can explain unavailability
    status: pending
  - id: b4-api-client
    content: convertCall / voidCall + extended PingRecord in web/src/lib/api.ts
    status: pending
  - id: b5-confirm-dialog
    content: In-repo ConfirmDialog component (no new npm dependency)
    status: pending
  - id: b6-status-column
    content: Status column in PingsTable — Converted / Review / —
    status: pending
  - id: b7-filter-control
    content: Conversion segmented control in FilterBar + date-filter context
    status: pending
  - id: b8-convert-cancel-buttons
    content: Convert / Cancel buttons on /calls/[callId] with integration resolution
    status: pending
  - id: report
    content: docs/convert-cancel-screens-report.md with browser proof
    status: pending
isProject: false
---

# Convert / Cancel Screens pointer

Full plan: [docs/convert-cancel-screens-plan.md](../../docs/convert-cancel-screens-plan.md)

Locked decisions recorded there: D1 "either condition" filter rule, D2 amount pre-fill,
D3 integration resolution, D4 amount semantics shown to the operator.

Do not edit this pointer for content changes — update the docs canonical copy.
