# Retreaver account clean reset — report

**Date:** 2026-07-13  
**Goal:** Total wipe of old-account Retreaver test data so a new account can be integrated without mixing rows.  
**Credentials:** never printed; new integration is UI-only (operator).

---

## Verdict

| Check | Result |
|-------|--------|
| ClickHouse `platform='retreaver'` after wipe | **0** |
| Redis `*:dedup:retreaver:*` after clear | **0** |
| Postgres `api_sync` Retreaver integrations after delete | **0** (22 removed) |
| Webhook row kept | **YES** — `Retreaver Dev Postback` |
| Stack restart | **api / platform-sync / web** restarted; retreaver count still **0** |
| STEP 6–7 (new account Live + D8b re-confirm) | **PENDING** — operator sets up in UI |

**D8b note:** Earlier “No matching records” was the **wrong Retreaver account**, not a prefix-search bug. Prefix match stands; re-confirm on the new account after Live sync (STEP 7).

---

## STEP 1 — Inventory (BEFORE)

### ClickHouse real-pull (non-demo, non-empty `external_call_id`)

| Metric | Value |
|--------|-------|
| rows | **57,805** |
| distinct calls (`uniqExact(external_call_id)`) | **4,452** |
| `min(received_at)` | 2026-06-25 05:27:03 |
| `max(received_at)` | 2026-07-13 17:24:31 |

### Demo / webhook-smoke (same platform, pre-wipe)

| Metric | Value |
|--------|-------|
| `apisync-%` demo rows | **2,736** |
| webhook-smoke (`occurred_at=0`, not apisync) | **35** |
| **total** `platform='retreaver'` | **60,541** |

### Redis dedup keys (before)

| Pattern | Count |
|---------|-------|
| `*:dedup:retreaver:*` | **4,786** |

### Integrations (before) — `platform='retreaver'`

**23 rows** total. Notable:

| id (prefix) | name | connection_mode | sync_mode | active |
|-------------|------|-----------------|-----------|--------|
| `886713af-…` | Ridgerise | api_sync | mock | f |
| `4ec62a84-…` | test rtt acc | api_sync | mock | f |
| … | (20 other api_sync verify/test rows) | api_sync | mock/real | f |
| `a9182df0-…` | Retreaver Dev Postback | **webhook** | mock | **t** |

(Full list was 22× `api_sync` + 1× `webhook`. Credentials not logged.)

---

## STEP 2 — ClickHouse wipe

```text
ALTER TABLE pings DELETE WHERE platform='retreaver'
```

| After | Value |
|-------|-------|
| `SELECT count() FROM pings WHERE platform='retreaver'` | **0** |
| Mutations pending | **0** (mutation finished) |

### Other platforms (untouched)

| platform | rows |
|----------|------|
| callgrid | 24 |
| callloom-native | 8 |
| ringba | 912 |
| trackdrive | 24 |

---

## STEP 3 — Redis dedup clear

| | Count |
|--|-------|
| Before `*:dedup:retreaver:*` | **4,786** |
| After delete (`scan` + `DEL`) | **0** |

---

## STEP 4 — Remove old `api_sync` integrations

```text
DELETE FROM integrations WHERE platform='retreaver' AND connection_mode='api_sync';
→ DELETE 22
```

### Remaining Retreaver integrations

| id | name | connection_mode | sync_mode | is_active |
|----|------|-----------------|-----------|-----------|
| `a9182df0-085c-48a4-a86c-ef8582d65e46` | Retreaver Dev Postback | webhook | mock | t |

Webhook kept for smoke/dev postback. No api_sync cursor/credentials left.

---

## STEP 5 — Restart + clean feed

```text
cd deploy
docker compose restart platform-sync api web
```

| Service | Status |
|---------|--------|
| deploy-api-1 | Up (`:3000`) |
| deploy-platform-sync-1 | Up (`:8088`) |
| deploy-web-1 | Up (`:3001`) |

Post-restart:

| Check | Result |
|-------|--------|
| CH retreaver rows | **0** |
| Redis retreaver dedup keys | **0** |
| api_sync retreaver integrations | **0** |

**UI:** Hard-refresh (Ctrl+Shift+R) Ping Feed, Coverage, and Settings so caches do not show stale Retreaver rows.

---

## STEP 6 — Fresh integration (operator / UI)

**Not executed by agent** (credentials stay in UI only).

Operator checklist:

1. Settings → Integrations → Retreaver, name e.g. `Retreaver (new acct)`.
2. Connection: API key; Data source: **Test** first.
3. Paste **new** account `api_key` + `company_id` (account Syed can log into).
4. Save → mock sync ok.
5. Do **not** flip Live until STEP 7 verification is ready.

---

## STEP 7 — Live + new-account verify (PENDING)

After operator creates the integration and flips Live + one sync:

1. Confirm only new-account UUIDs in CH (no old-account mix — wipe ensures this for prior rows):

```bash
docker exec deploy-clickhouse-1 clickhouse-client --query "SELECT external_call_id, state, revenue, occurred_at FROM pings WHERE platform='retreaver' AND external_call_id NOT LIKE 'apisync-%' AND external_call_id != '' ORDER BY received_at DESC LIMIT 10"
```

2. Logs: no plaintext `api_key=` (redacted only).
3. Cross-verify with **new** Retreaver login: Calyx UUID / 8-char prefix → found; deep link “Open in Retreaver” opens matching call.
4. Flip back to Test when done testing, if desired.

| Item | Status |
|------|--------|
| New-account rows visible | **PENDING** (UI) |
| Deep link + full-uuid cross-check | **PENDING** (UI) |
| D8b prefix both ways on new account | **PENDING** (re-confirm; prior cert claim stands) |

---

## D8b — no code change

The earlier prefix-search failure was caused by searching an id from the **old** test account while logged into a **different** account. Retreaver prefix search (e.g. first 8 chars) works. Certification D8b claim remains valid; re-confirm once new-account Live data exists.

---

## One-line

Total-wipe Retreaver CH (**60,541 → 0**) + Redis dedup (**4,786 → 0**) + **22** api_sync integrations removed (webhook kept); stack restarted clean — operator creates the new-account integration in the UI next.
