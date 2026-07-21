# Call-detail parity audit — Calyx vs Ringba

**Date:** 2026-07-21  
**Scope:** What `/calls/[callId]` shows today vs Ringba Call Details (Record tab + Call Details report columns).  
**Goal:** Calyx should show everything a buyer/ops user needs so **Open in Ringba** is a rare fallback, not a necessity.

Sources: Calyx UI (`CallDeepViewPanel`, `ScoreCard`, `TranscriptPanel`, call-detail page), deep-view API shape, Ringba [Call Details Report](https://support.ringba.com/hc/en-us/articles/17993830584855-Call-Details-Report) / Record tab (campaign, publisher, target, buyer, inbound call ID, dialed number, tags, recording, etc.).

Legend for **in Calyx?**:

- **Yes** — visible on the call-detail page today  
- **Partial** — in API / ClickHouse / another panel but **not** shown on call-detail (or only as a weak proxy)  
- **No** — not surfaced on call-detail

---

## Parity table

| Field | in Calyx? | in Ringba? | Gap |
|-------|-----------|------------|-----|
| Inbound Call ID | **Yes** (header + copy) | **Yes** (Record / column) | — |
| Revenue (conversion) | **Yes** (deep-view Ping) | **Yes** | — |
| Payout | **Partial** — in `deepView.ping.payout` / CH; **not rendered** in `CallDeepViewPanel` | **Yes** | **UI gap** — store yes, show no |
| RPC (per-call) | **No** — dashboard/analytics aggregate RPC only | **Yes** (report metrics / derived) | **Gap** — no per-call RPC on detail |
| Campaign ID / name | **Partial** — `campaign_id` in deep-view API; **not shown** in panel | **Yes** (name + ID on Record) | **UI gap** (+ Ringba often shows **name**, we only store ID) |
| Publisher ID / name | **Partial** — ID shown; no display name | **Yes** (name + ID) | **Name gap** |
| Target ID / name | **Partial** — in API; **not shown** in panel | **Yes** (name + ID) | **UI + name gap** |
| Buyer | **No** | **Yes** (Record) | **Gap** — not ingested/mapped today |
| State (caller geo) | **Yes** (2-letter) | **Yes** (tags / geo) | — |
| Duration | **Partial** — on `TranscriptPanel` / scoring when transcript exists; not always on ping-only calls | **Yes** (Call Length) | **Gap** when no transcript yet; should come from Ringba call length column |
| Timestamp (call time) | **Partial** — `received_at` / `occurred_at` in API & ping feed; **not shown** on call-detail deep-view panel | **Yes** (`callDt` / Call Date) | **UI gap** — show `occurred_at` (callDt) prominently |
| Recording URL | **No** | **Yes** (when recording enabled) | **Gap** — no recording fetch/store/play on detail |
| Transcript | **Yes** (`TranscriptPanel`) when scoring pipeline ran | **Sometimes** (Ringba transcription product / notes — not always) | Calyx often **ahead** here; still empty if score/transcript job didn’t run |
| Tags (custom / user / SIP / annotations) | **No** (enrichment attrs ≠ Ringba call tags) | **Yes** (Record categories + filterable tags) | **Gap** — not synced into call-detail |

---

## Summary — what still forces Ringba open

Customers still need Ringba (or will feel incomplete in Calyx) for:

1. **Recording playback / URL**  
2. **Buyer** identity  
3. **Full tag / annotation set** from the call  
4. **Campaign / publisher / target display names** (we have IDs only, and target/campaign not even on the panel)  
5. **Payout + call timestamp + target** on the detail UI (data often present, UI incomplete)  
6. **Per-call RPC** and reliable **call duration** without waiting on transcript  

Calyx differentiators already on the page (competitors often send users back to the dialer): **TCPA / compliance risk**, **call-quality score + sentiment**, **enrichment attribute count**, **ad attribution** — keep these; close the table gaps above so Open in Ringba is optional.

---

## Suggested close-the-gap order (not implemented here)

1. **Surface existing ping fields** on `CallDeepViewPanel`: payout, campaign_id, target_id, `received_at` / prefer `occurred_at` when available.  
2. **Map Ringba call length** → duration on ping (don’t depend on transcript).  
3. **Recording URL** via Ringba API/column when present (presigned / link-out with TTL).  
4. **Buyer + tag bag** from Get Call Log `valueColumns` / tags.  
5. **Name resolution** (campaign/publisher/target/buyer) via Ringba entity APIs or report columns that return names.

---

## Related UX (done this slice)

**Open in Ringba** now: copy `inboundCallId` → open call-logs report → toast  
`"Call ID copied — paste into the Inbound Call ID filter in Ringba."`  
so the report page is not an unfiltered dead-end.
