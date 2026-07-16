# URL privacy + real/mock feed mix — check report

**Date:** 2026-07-14  
**Scope:** Read-only verification on new-account Retreaver test.  
**Credentials / raw ANI:** never printed.

---

## Verdict

| Item | Result |
|------|--------|
| PART 1 — detail link uuid = `external_call_id` | **PASS** |
| PART 2 — `aniHash` in URL | **Privacy smell** — needed today only for EnrichmentPanel; droppable via server/`deepView` (recommend drop; **do not change in this pass**) |
| PART 3 — `apisync-*` after wipe | **1 mock row** from first Test/mock sync of integration `Test 1`; recommend **default-hide** `apisync-%` |
| PART 4 — cross-verify + no `api_key` leak | **Ready** (3 real uuids) · `api_key=` log count **0** |
| Open in Retreaver deep link | Uses path uuid (= `external_call_id`); pattern `https://retreaver.com/calls/{id}` |

**Blocker?** No.

### Product follow-ups (do not implement now)

Privacy / polish only — not certification blockers. Tracked 2026-07-14:

1. **Drop `aniHash` from the call-detail URL** — resolve enrichment **server-side by `callId`** (API loads stored hash; no caller-derived hash in history / access logs / `Referer`). Keep `platform=` if needed.
2. **Default-hide `apisync-%` demo rows** in the Ping Feed, with an optional **“Show demo”** toggle for mock/verify testing.

Evidence and rationale: this report (PART 2 / PART 3). Status: **deferred**.

---

## PART 1 — Call id correct?

Query:
```text
SELECT external_call_id, state, revenue, occurred_at, platform
FROM pings WHERE external_call_id = '78b653b3-04c2-4119-94d5-70d63589df23'
```

| Field | Observed |
|-------|----------|
| `external_call_id` | `78b653b3-04c2-4119-94d5-70d63589df23` |
| `platform` | `retreaver` |
| `state` | `WA` |
| `revenue` | `0` |
| `occurred_at` | `2026-07-13 22:40:28` |

**Expect match:** PASS — URL path `/calls/{uuid}` is the real `external_call_id` (Ping Feed truncates display only).  
**Cross-check prefix in new Retreaver account:** `78b653b3`.

Link build site (`web/src/components/dashboard/PingsTable.tsx`):
```text
/calls/${encodeURIComponent(p.external_call_id)}?aniHash=...&platform=...
```

---

## PART 2 — Why is `aniHash` in the URL?

### What it is

`ani_hash` = SHA-256 (or FixedString(64) hash) of caller ANI — stored on the ping row. **Not** the raw phone number. Client APIs expose `ani_hash` only; ClickHouse has `ani_hash`, no plaintext ANI column on `pings`.

### Where the link is built

| Location | Role |
|----------|------|
| `web/src/components/dashboard/PingsTable.tsx` | Appends `?aniHash=${p.ani_hash}&platform=...` on Call ID href |
| `web/src/app/(app)/calls/[callId]/page.tsx` | Reads `searchParams.get('aniHash')` |
| `web/src/components/enrichment/EnrichmentPanel.tsx` | `GET /enrichment/{aniHash}` via `fetchEnrichment` |
| `services/api/src/enrichment/enrichment.controller.ts` | Lookup by `ani_hash` |
| `services/api/src/native-integration/deep-query.service.ts` | Already loads ping + enrichment summary **server-side** by `callId` using stored `ani_hash` |

### What the detail page needs it for

Primary call data (score, transcript, deep view) loads by **`callId` only** — `aniHash` is **not** required for those fetches:

```text
fetchScores(token, { callId })
fetchTranscript(token, callId)
fetchCallDeepView(token, callId)
```

`aniHash` is used only to:

1. Gate / feed **`EnrichmentPanel`** (full attribute list).
2. Set journey step `hasEnrichment: Boolean(aniHash)`.

Without the query param, the page still works but shows: *“No ANI hash provided…”* — even though `fetchCallDeepView` already returns `ping.ani_hash` and a server-side enrichment summary.

### Raw ANI on client?

**No.** Grep of `web/src` shows no raw ANI/phone number fields in ping UI — only `ani_hash` / “caller” copy. Deep link Open in Retreaver uses **external_call_id** only (`buildPlatformCallUrl(platform, callId)`).

### Recommendation (privacy item — not a blocker; **no code change this pass**)

| Preferred | Drop `aniHash` from the URL. Resolve enrichment via: |
|-----------|------------------------------------------------------|
| A | Use `deepView.ping.ani_hash` in component state after deep-view load (still a hash in memory, not in history/`Referer`), **or** |
| B | Prefer server: extend deep-view / enrichment API keyed by `callId` only (API loads `ani_hash` from encrypted/hashed storage). |

Keep `platform=` if useful (not sensitive).  
Do **not** put caller-derived hashes in GET query strings long-term — same principle as log redaction (history, access logs, Referer).

---

## PART 3 — Real / mock mix after wipe

### Counts (now)

| | Count |
|--|-------|
| `apisync-%` mock rows (retreaver) | **1** |
| Real non-empty `external_call_id` | **9,841** |

Newest mock:
```text
apisync-retreaver-20260714-708697334b59107f   received_at=2026-07-14 10:56:39
```

### Source

**Mock adapter** (`services/platform-sync/internal/adapter/mock.go`) mint ids as:
```text
apisync-{platform}-{YYYYMMDD}-{rand}
```

Integrations now:

| name | mode | sync_mode | status |
|------|------|-----------|--------|
| **Test 1** (`269ba4dd-…`) | `api_sync` | **real** (Live) | ok |
| Retreaver Dev Postback | webhook | mock | idle |

platform-sync log at **same second** as the mock row:
```text
2026-07-14T10:56:40Z  api sync completed  integration_id=269ba4dd-…  calls=1
```
Then later syncs: `calls=1000` / `436` / … (real Live pulls).

**Conclusion:** After the wipe, Syed created **Test 1** — first sync ran while still on **Test/mock** (1 synthetic `apisync-retreaver-*` ping), then flipped **Live** and real UUIDs poured in. Not a second integration and not wipe failure. Webhook Dev Postback did not create this row (`apisync-` prefix is mock **API sync** only).

### Options (Syed decides)

| | Option | Notes |
|--|--------|-------|
| **(a) Recommend** | Default-hide `apisync-%` in Ping Feed (optional “Show demo” toggle) | Cleanest for Live cross-verification |
| (b) | Delete the one demo row + avoid Test sync / `verify-api-sync` during Live | Manual; demo returns if Test sync runs again |
| (c) | Leave visible | Harmless if operators know the prefix; muddies real testing |

**Recommend (a).**

---

## PART 4 — Cross-verification readiness (new account)

### Sample real uuids (newest first)

| external_call_id | state | occurred_at | 8-char prefix |
|------------------|-------|-------------|---------------|
| `57e6eb1b-0680-4d42-906e-c926282523a0` | NM | 2026-07-13 17:00:18 | `57e6eb1b` |
| `58bbf84b-3a1d-4237-986e-d0431c9bab71` | FL | 2026-07-09 17:15:19 | `58bbf84b` |
| `5831ed64-26c9-485d-aa96-18af7578e870` | CA | 2026-07-09 18:49:59 | `5831ed64` |

Plus PART 1 example: `78b653b3-04c2-4119-94d5-70d63589df23` → prefix `78b653b3`.

**D8b check (operator):** prefix search in **new** Retreaver account → found; Calyx → Open in Retreaver → `https://retreaver.com/calls/{full-uuid}`.

### Credential leak check

```text
docker logs deploy-platform-sync-1 | grep -c "api_key="
→ 0
```

No `api_key=` lines in current platform-sync logs (redaction path; no raw key printed).

---

## One-line

Detail link uses real `external_call_id`; `aniHash` is caller-hash for EnrichmentPanel only (privacy: drop from URL / resolve via deep-view); one `apisync-*` row came from Test 1’s first mock sync before Live — recommend default-hide demo rows; real uuids ready for new-account D8b with no `api_key` leak.
