# Ringba Pre-Live Checks Report

**Date:** 2026-07-15  
**Scope:** Approved fixes (1→3) then CHECK 1 + CHECK 2 from the pre-live checklist.  
**CHECKs 3/4/5:** Already answered previously — re-verified after approved fixes; raw proof included under Implemented fixes.

Nothing below used real Ringba credentials. `BOGUS_RINGBA_TOKEN_XYZ` is safe to print.

---

## Implemented (approved order)

### (1) Redis named volume + AOF — hashes survive `--force-recreate`

`deploy/docker-compose.yml`:

```
redis:
  command: redis-server --appendonly yes --appendfsync everysec
  volumes:
    - redis_data:/data
volumes:
  redis_data:
```

**Survive-recreate proof (raw):**

```
$ docker compose up -d redis
$ docker exec deploy-redis-1 redis-cli CONFIG GET appendonly
appendonly
yes
$ docker exec deploy-redis-1 redis-cli SET "proof:vol:survive" "hash-alive" EX 3600
OK
$ docker exec deploy-redis-1 redis-cli GET "proof:vol:survive"
hash-alive
$ docker inspect deploy-redis-1 --format '{{json .Mounts}}'
[{"Type":"volume","Name":"deploy_redis_data","Source":"/var/lib/docker/volumes/deploy_redis_data/_data","Destination":"/data","Driver":"local","Mode":"rw","RW":true,"Propagation":""}]
$ docker compose up -d --force-recreate redis
$ docker exec deploy-redis-1 redis-cli GET "proof:vol:survive"
hash-alive
$ docker exec deploy-redis-1 redis-cli CONFIG GET appendonly
appendonly
yes
```

**`maxmemory` — OPS ITEM (before any large backfill):**

```
$ docker exec deploy-redis-1 redis-cli CONFIG GET maxmemory
maxmemory
0
```

Hashes **do** survive recreate (volume + AOF).

### OPS ITEM — Redis maxmemory + noeviction

**Before any large production / multi-page Ringba backfill:** set Redis `maxmemory` (size TBD for env) with `maxmemory-policy noeviction`. Today `maxmemory=0` means the policy never binds and Redis can grow until host OOM. Prefer compose/`redis-server` flags (or managed Redis equivalent), then verify:

```
CONFIG GET maxmemory
CONFIG GET maxmemory-policy   # expect noeviction
```

Not a credential-entry blocker for CHECK 1/2; required before large backfill.

### (2) pingdedup fail-OPEN

`packages/pingdedup/dedup.go`: on any Redis GET/SET error → publish (`PublishNew`/`PublishChanged`) with **nil** error + loud `slog.Error`.

**Raw tests:**

```
=== RUN   TestEvaluateFailOpenOnRedisUnreachable
ERROR pingdedup fail-OPEN: redis GET failed — publishing ...
--- PASS: TestEvaluateFailOpenOnRedisUnreachable (0.10s)
=== RUN   TestEvaluateFailOpenOnSetAfterClose
--- PASS: TestEvaluateFailOpenOnSetAfterClose (0.09s)
ok  packages/pingdedup

=== RUN   TestPublishSyncCallsFailOpenStillPublishes
--- PASS: TestPublishSyncCallsFailOpenStillPublishes (0.10s)
ok  services/platform-sync/internal/syncer

=== RUN   TestWebhookPathFailOpenStillPublishes
--- PASS: TestWebhookPathFailOpenStillPublishes (0.09s)
ok  services/ingestion/internal/handlers
```

### (3) `call_key` / `ani_hash` NOT in hash; read-path `argMaxIf`

Documented blindness in `packages/pingdedup/content.go` (INTENTIONALLY EXCLUDED).

Deep query read path (`services/api/src/native-integration/deep-query.service.ts`):

```sql
argMaxIf(ani_hash, received_at, ani_hash != '') AS ani_hash,
argMaxIf(call_key, received_at, call_key != '') AS call_key
```

---

## CHECK 1 — Bogus-token HEADER redaction on DEPLOYED stack

### 1. Phase 2 image

```
$ docker compose -f deploy/docker-compose.yml ps platform-sync
NAME                     IMAGE                  COMMAND             SERVICE         CREATED         STATUS         PORTS
deploy-platform-sync-1   deploy-platform-sync   "./platform-sync"   platform-sync   …               Up             0.0.0.0:8088->8088/tcp

$ docker exec deploy-platform-sync-1 sh -c "ls -l /app"
total 17068
-rwxr-xr-x    1 root     root      17475950 Jul 15 15:41 platform-sync
```

Binary contains `dedup_mode=content_hash`, `Authorization: [REDACTED]`, `pingdedup fail-OPEN`.

### 2–4. Transport path (unresolvable host)

Temporary env (removed after):

```
RINGBA_BASE_URL=https://this-host-does-not-exist-xyz.invalid/
```

Integration: `bogus-ringba-header-gate` / `api_token=BOGUS_RINGBA_TOKEN_XYZ` / `account_id=999999` / Live.

**Header-auth correctness (do not inject token into errors):** Go transport errors carry the **URL**, not headers — a header token cannot appear there. `REDACTED >= 1` is the positive control for **query-string** auth (Retreaver only). An earlier `withAuthContext` that appended `Authorization: <token>` before `SanitizeErr` was **reverted** — it created a pre-sanitize leak surface that did not exist.

**Correct header-auth proof (four items) — after revert, re-run raw:**

```
BOGUS_sync=0
REDACTED_sync=0
BOGUS_api=0

# REDACTED=0 — expected and correct for header auth (headers absent from Go transport errors)

SELECT name, sync_status, last_sync_error FROM integrations WHERE name ILIKE '%bogus%';
 bogus-ringba-header-gate | error | network error (transient, will retry): Post "https://this-host-does-not-exist-xyz.invalid/999999/CallLogs/Date": dial tcp: lookup this-host-does-not-exist-xyz.invalid on 127.0.0.11:53: no such host
```

Log excerpt (no Authorization, no token):

```
{"msg":"request failed","error":"Post \"https://this-host-does-not-exist-xyz.invalid/999999/CallLogs/Date\": dial tcp: lookup this-host-does-not-exist-xyz.invalid on 127.0.0.11:53: no such host"}
```

Binary confirmation after revert: `strings /app/platform-sync | grep -c withAuthContext` → `0`

**SanitizeErr would-redact proof (unit — if Authorization ever appeared):**

```
=== RUN   TestSanitizeErrRedactsBareAuthorizationToken
--- PASS: TestSanitizeErrRedactsBareAuthorizationToken (0.00s)
```

Also: `TestHTTPClientDoTransportErrorOmitsAuthorizationHeader` asserts dial errors do not contain the token and do not inject `Authorization:`.

### 5. AUTH 401 path (no base-URL override)

`RINGBA_BASE_URL` removed; container recreated; sync hit real `api.ringba.com` with bogus token.

```
BOGUS_sync=0
REDACTED_sync=0
BOGUS_api=0

name | sync_status | err
bogus-ringba-header-gate | error | authentication error: ringba HTTP 401: {"message":"Authorization has been denied for this request."}
```

(REDACTED=0 on 401 is expected — response body has no token; same class as Retreaver 401.)

**Header / `%+v` logging scan:**

```
$ rg -n "%\+v|%#v" services/platform-sync/ --glob "!**/*_test.go"
(no matches)

Authorization only set/read via req.Header.Set/Get; errors go through SanitizeErr / ClassifySyncError.
```

### 6. Cleanup

```
DELETE FROM integrations WHERE name ILIKE '%bogus%';
bogus_remaining=0
RINGBA_BASE_URL removed from deploy/docker-compose.yml
```

**CHECK 1: PASS** — header-auth proof: BOGUS=0 on transport, BOGUS=0 on 401, `%+v`/header-object logging grep clean, SanitizeErr unit covers Authorization if it ever appeared. `REDACTED=0` on transport is **expected and correct** for header auth.

---

## CHECK 2 — Dual-window throttle (5/sec AND 80/min)

### Code greps (raw)

```
internal/adapter/httpclient.go:54:	ra := strings.TrimSpace(h.Get("Retry-After"))
internal/adapter/httpclient.go:70:// Do … 429 retry (honours Retry-After)
internal/adapter/ratelimit.go:9:// Scope: one limiter instance per external account key
internal/adapter/ratelimit.go:71:// RingbaAccountLimiters … 5/sec AND 80/min
internal/adapter/ringba_adapter.go:130:	// Dual-window throttle: 5/sec AND 80/min, scoped per Ringba account_id
internal/adapter/ringba_adapter.go:131:	ringbaLimiters.ForAccount(accountID).Wait()
```

No fixed `220ms` sleep on the Ringba path.

### Plain answers

| Question | Answer |
|----------|--------|
| Spacing/limiter? | `DualWindowLimiter{perSec:5, perMin:80}` sliding windows |
| Sustained multi-page req/min? | Max **80** in any rolling 60s (5/sec burst allowed inside that) |
| Scope? | **Per Ringba `account_id`** (`RingbaAccountLimiters.ForAccount`) |
| Retry-After on 429? | **Yes** — `parseRetryAfter` + backoff in `HTTPClient.Do` |

### Sustained-walk proof (raw)

```
=== RUN   TestDualWindowLimiterHonoursPerMinuteOnSustainedWalk
    ratelimit_test.go:53: n=160 elapsed=1m15.1887292s naive_avg_req_per_min=127.68
    ratelimit_test.go:68: max_hits_in_any_rolling_60s=80
--- PASS: TestDualWindowLimiterHonoursPerMinuteOnSustainedWalk (75.19s)
=== RUN   TestRingbaAccountLimitersKeyedSeparately
--- PASS
=== RUN   TestParseRetryAfterSeconds
--- PASS
```

Note: naive average over wall time looks >80 because of front-loaded bursts after each minute refill; the rolling-window max is the binding Ringba quota metric (**80**).

**CHECK 2: PASS**

---

## Wipe ringba test rows

```
$ docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "SELECT count() FROM pings WHERE platform='ringba'"
931
$ docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "ALTER TABLE pings DELETE WHERE platform='ringba'"
$ docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "SELECT count() FROM pings WHERE platform='ringba'"
0
```

---

## CHECKs 3 / 4 / 5 (answered previously — post-fix status)

| Check | Prior | Now |
|-------|-------|-----|
| 3 Redis volume / hashes survive recreate | NO volume | **YES** (`deploy_redis_data`, AOF=yes; maxmemory still **0**) |
| 4 Evaluate Redis error | FAIL-CLOSED | **FAIL-OPEN** (tests above) |
| 5 webhook vs api-sync fields / hash membership | call_key mismatch risk | **Excluded from hash + documented**; read uses `argMaxIf` |

---

## Gate lines

```
CHECK 1 bogus header redaction on deployed stack: PASS  (BOGUS=0 transport+401; REDACTED=0 expected for header auth; %+v clean; SanitizeErr unit OK)
CHECK 2 dual-window throttle (5/sec + 80/min, per-account): PASS  (rolling max=80; Retry-After yes; per account_id)
CHECK 3 redis volume present / hashes survive recreate: YES  (maxmemory: 0 — OPS ITEM before large backfill)
CHECK 4 Evaluate() on Redis error: FAIL-OPEN  (must be FAIL-OPEN)
CHECK 5 webhook vs api-sync field sets identical: NO  (call_key/ani_hash excluded from hash; clobbering mitigated via argMaxIf; clobbering observed historically: YES class)
Safe to enter real Ringba credentials: YES
```

**OPS ITEM:** set Redis `maxmemory` + `maxmemory-policy noeviction` before any large backfill (see §1 above).
