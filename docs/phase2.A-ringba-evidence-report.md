# Phase 2.A — Ringba official-docs evidence + decision gate

**Date:** 2026-07-14  
**Gate:** **GO** — proceed to Phase 2.B / 2.C

## Evidence table

| ID | Check | Finding | Sources |
|----|--------|---------|---------|
| **A1** | `POST /{accountId}/CallLogs/Date` + auth + rate limits | Path confirmed: `POST /{accountid}/CallLogs/Date` (`callLogGetLogsByDate`). Auth: **API access token** in `Authorization` header; **Bearer deprecated** effective **2026-06-01** (MFA + access tokens). Rate: reporting/call-log style **5/sec or 80/min**; httpclient already retries **429**. Adapter spaces ~220ms. | [CallLogApi.md](https://github.com/diablomedia/ringba-php-sdk/blob/master/docs/Api/CallLogApi.md); [May 30 2025 release notes](https://updates.ringba.com/release-notes-for-may-30-2025-316338); [Rate limits](https://support.ringba.com/hc/en-us/articles/23785949612695); [Export limits](https://support.ringba.com/hc/en-us/articles/23787558523159) |
| **A2** | columns/events/tags → mapped fields + **occurred_at** | Response shape: `callLog.data[]` with `columns` / `tags` (name/value). IDs: `inboundCallId` (primary) / `callId`. Geo: `state` / `geo:state`. Money: `revenue` / `payout`. Campaign/publisher/target: `campaignId` / `publisherId` / `targetId`. **Call timestamp → `occurred_at`:** column **`callDt`** (also used in community `valueColumns`). | CallLogApi + prior research; StackOverflow / Plura call-logs examples using `callDt`; [Call Details Report](https://support.ringba.com/hc/en-us/articles/17993830584855-Call-Details-Report) |
| **A3** | Last-modified / updated filter? | **NO** documented last-mod incremental. Endpoint is **date-window** (`reportStart` / `reportEnd` / CallLogsStatRequest date range). Filters are attribute/tag filters, not “modified since”. | CallLogApi.md; [Reporting Filters](https://support.ringba.com/hc/en-us/articles/17994137377815-Reporting-Filters); research `docs/apisync-platform-research.md` |
| **A4** | Deep-link + UI Call ID vs API id | **Case A:** UI Inbound Call ID = API `inboundCallId` = Calyx `external_call_id`. Public shareable call URL **not** published in official KB → catalog pattern + `NEXT_PUBLIC_RINGBA_CALL_URL_PATTERN` override; treat deep link as **best-effort**. | Annotations API uses `inboundCallId` UUID shape; Call Details “Record” tab shows inbound call ID |

## Pre-committed decisions (confirmed)

| Decision | Outcome |
|----------|---------|
| A3 NO last-mod | Implement **overlap window + content-hash** (Phase 2.0 already shipped). Do **not** use `updated_at`-style cursor. |
| `platform_duplicate` rule (b) | Only if payload carries an explicit duplicate flag. Column probe in mapper; leave unset unless present. |
| Backfill | Same **30-day** default + Settings lookback UI as Retreaver (`initial_lookback_days`). |

## Gate contradiction?

**None.** Evidence matches the plan defaults. Proceed to Phase 2.B.
