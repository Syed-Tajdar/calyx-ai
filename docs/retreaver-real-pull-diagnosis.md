# Retreaver Real Pull — Diagnosis

**Date:** 2026-07-09  
**Integration:** `test acc` (`b904b7e7-a8d8-4dda-9963-5c54e8db5a8d`) — flipped back to **Test** (`sync_mode=mock`) before diagnosis completed.

---

## STEP 1 — Sync error + platform-sync logs

### Postgres (`integrations` — retreaver, latest 5)

| name | connection_mode | sync_mode | sync_status | last_sync_error |
|------|-----------------|-----------|-------------|-----------------|
| test acc | api_sync | **mock** | ok | *(cleared after mock syncs)* |
| Retreaver Dev Postback | webhook | mock | idle | — |
| api-sync-verify-* | api_sync | mock | ok | — |

At diagnosis time `last_sync_error` was empty because the row was flipped to Test and subsequent mock syncs succeeded.

### platform-sync logs (credentials redacted)

**Successful real pulls (Live mode):**

```
api sync completed integration_id=b904b7e7-… platform=retreaver calls=308
api sync completed … calls=8
api sync completed … calls=7
api sync completed … calls=6
```

HTTP auth succeeded — not an auth failure. Retreaver returned paginated data.

**Intermittent sync failures (separate from mapping):**

```
sync failed … platform=retreaver error=Get "https://api.retreaver.com/api/v2/calls.json?api_key=[REDACTED]&company_id=[REDACTED]&…": dial tcp: lookup api.retreaver.com on 127.0.0.11:53: server misbehaving

sync failed … error=… lookup api.retreaver.com … no such host
```

Transient Docker DNS failures — explains UI **sync_status=error** on some cycles, independent of field mapping.

**Security note:** Error logs currently include `api_key` in the request URL query string. Logs should redact query params (follow-up hardening).

---

## STEP 2 — Retreaver response shape vs adapter

### Official Retreaver API V2 (`GET /api/v2/calls.json`)

Per [Retreaver Core API docs](https://retreaver.github.io/core-api-docs/) — **top-level shape is a bare JSON array**, not `{"calls":[...]}`:

```json
[
  {
    "call": {
      "uuid": "f1abfb78-ab8a-4146-9946-8169fbcc6d6c",
      "caller": "+13015236555",
      "caller_state": "NC",
      "revenue": 5.0,
      "payout": 5.0,
      "system_campaign_id": 9872,
      "system_target_id": 27449,
      "afid": null,
      "status": "finished",
      "connected": true,
      ...
    }
  }
]
```

**Key names on the call object (documented):** `uuid`, `caller`, `caller_state`, `revenue`, `payout`, `status`, `connected`, `system_campaign_id`, `system_target_id`, `afid`, `cid`, …

### What `mapRetreaverCall` expected (before fix)

| Field | Adapter expected | Actual V2 location |
|-------|------------------|-------------------|
| `external_call_id` | top-level `uuid` | nested under **`call.uuid`** |
| `state` | top-level `caller_state` | nested under **`call.caller_state`** |
| `revenue` / `payout` | top-level | nested under `call.*` |
| `publisher_id` | `client_afid` | often **`afid`** |
| `campaign_id` | `campaign_id` | often **`system_campaign_id`** |
| `target_id` | `target_id` | often **`system_target_id`** |

### Parser path

`fetchCallsWithPerPage` correctly falls back to unmarshaling a **bare array** into `[]json.RawMessage`, but each element is `{"call":{...}}`. `mapRetreaverCall` unmarshaled that wrapper directly → **all mapped fields empty** while sync still reported `calls=308`.

---

## STEP 3 — ClickHouse counts

```sql
-- retreaver pings
empty_id=371  demo_rows(apisync-*)=1825  real_looking=21
empty_state=371  has_state=1846
```

**Empty-id sample (2026-07-09 16:14 UTC):** `external_call_id=''`, `state=''`, `revenue=0`, `is_duplicate=1` — matches bad real-pull mapping window.

**Real-looking sample:** UUIDs with `state=CA`, revenue populated — mostly **webhook smoke / demo** rows (dates 2026-07-03–08), not the bad API-sync batch.

---

## STEP 4 — Ping feed sources

| Source | Active? | Contributes retreaver rows? |
|--------|---------|----------------------------|
| ClickHouse `/pings` API | Yes | Yes — includes empty-id bad rows + apisync-* demo rows |
| Mock live emitter (`MOCK_MODE=true`) | Yes (`mock-live-emitter.service.ts`) | Yes — WS events merged in `useHybridPingFeed`; synthetic `call_id` UUIDs, empty state, $0 revenue |
| Kafka live consumer | No (disabled when `MOCK_MODE=true`) | — |

**Conclusion:** Feed shows **both** ClickHouse rows and mock WebSocket events. Empty call-id / DUP=Yes rows are primarily from **bad API-sync mapping** in ClickHouse; live mock emitter adds additional synthetic retreaver pings with UUID call_ids but empty state.

---

## STEP 5 — Empty external_call_id guard (added)

In `services/platform-sync/internal/syncer/syncer.go`:

- Skip any mapped call with blank `ExternalCallID`
- Count skips; **do not publish** to Kafka
- Set `sync_status=error` with message `N calls skipped: missing external id`

---

## STEP 6 — Cleanup (recommended)

After rebuilding platform-sync with the mapper fix:

```sql
ALTER TABLE pings DELETE WHERE platform='retreaver' AND external_call_id='';
```

Leave `apisync-*` demo rows unless a full reset is desired.

---

## Fix applied

**Files:**

- `services/platform-sync/internal/adapter/retreaver_adapter.go` — unwrap `{"call":{...}}`; map `afid`, `system_campaign_id`, `system_target_id`
- `services/platform-sync/internal/adapter/retreaver_adapter_test.go` — nested V2 list item test
- `services/platform-sync/internal/syncer/syncer.go` — empty-id guard

**Deploy:**

```powershell
cd deploy
docker compose build platform-sync
docker compose up -d platform-sync
```

Then flip integration to **Live** again and trigger sync; verify ClickHouse rows have UUID + state.

---

## Coverage map (not a bug)

Heatmap aggregates `WHERE state != '' AND is_duplicate = 0`. Bad rows had **empty state** and **is_duplicate=1** (shared empty dedup key) — correctly excluded until mapping is fixed.

---

**Root cause:** Retreaver V2 list returns `[{"call":{uuid,...}}]` but `mapRetreaverCall` read flat top-level fields → empty `external_call_id`/state/revenue; empty dedup key flagged nearly all rows duplicate. **Fix:** unwrap nested `call` object, map V2 field names (`afid`, `system_campaign_id`, `system_target_id`), and skip empty external IDs at publish time.

---

## Dedup path (API-sync vs webhook)

Full investigation with file:line refs, ClickHouse/Redis queries, and CASE A vs “409 drop” reconciliation: **[dedup-path-answer.md](./dedup-path-answer.md)**

**Short answer:** API-sync uses the **same Redis dedup** as webhook (shared key `{tenant}:dedup:{platform}:{external_call_id}`), but **bypasses the ingestion HTTP service** and publishes straight to Kafka. Both paths **store** duplicates with `is_duplicate=1` — they are **not** dropped with 409. The 370 empty-id rows were flagged by platform-sync Redis dedup on the shared empty key, not by the consumer.
