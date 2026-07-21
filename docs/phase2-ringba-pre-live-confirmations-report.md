# Phase 2 Ringba — Pre-Live Confirmations Report

**Date:** 2026-07-17  
**Commit verified:** `b719ec7b94d6c7da74f2460df7e80d83e89764a5`  
**Scope:** Deep-view ClickHouse correction, deployed API/web image proof, Ringba test-row provenance, and `withAuthContext` revert verification.

## Executive result

| Check | Result |
|---|---|
| Deep-view fix documented and inspected | **PASS** |
| Non-empty `call_key` preservation retained | **PASS** |
| API running the Phase 2 artifact | **PASS** |
| Web running the Phase 2.0b artifact | **PASS** |
| “1 real Ringba row” identified | **PASS — local smoke row, not live Ringba** |
| `withAuthContext` absent from deployed binary | **PASS — count 0** |
| Ringba D10 volume baseline clean | **NOT YET — test rows must be excluded or wiped** |

## 1. Deep-view ClickHouse fix

### Why presentation initially failed

The deep-view query was changed from selecting only the newest raw ping row to aggregating all rows for the call. This is required so a later update with an empty `call_key` does not erase a previously captured non-empty value.

An intermediate timestamp expression used:

```sql
max(received_at) AS received_at
```

The alias reused the source column name. ClickHouse alias substitution then caused `received_at` references inside the same aggregate projection to resolve to `max(received_at)`, producing a nested aggregate error:

```text
Aggregate function max(received_at) AS received_at is found inside another aggregate function in query.
```

### Exact corrective diff

The timestamp aggregate was given a distinct alias and formatted inside the aggregate subquery:

```diff
-          formatDateTime(p.received_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS ping_received_at,
+          p.ping_received_at,
...
-            max(received_at) AS received_at,
+            formatDateTime(max(received_at), '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS ping_received_at,
```

The committed aggregation is:

```sql
argMaxIf(ani_hash, received_at, ani_hash != '') AS ani_hash,
formatDateTime(max(received_at), '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS ping_received_at,
argMaxIf(call_key, received_at, call_key != '') AS call_key
```

Source: `services/api/src/native-integration/deep-query.service.ts`.

### Semantic assessment

The correction did **not** revert or weaken `argMaxIf(call_key, received_at, call_key != '')`.

`argMaxIf` considers only rows where `call_key != ''`. Therefore:

- a later empty `call_key` is excluded;
- an earlier non-empty `call_key` remains available;
- the newest non-empty `call_key` wins when more than one exists.

The presentation test reached **20/20** after removing the timestamp alias collision. It did not pass by sacrificing the non-empty `call_key` requirement.

## 2. Running API and web image proof

### Compose status

Relevant `docker compose ps` result:

```text
deploy-api-1   deploy-api   Up   0.0.0.0:3000->3000/tcp
deploy-web-1   deploy-web   Up   0.0.0.0:3001->3000/tcp
```

### API image

```text
container:  deploy-api-1
created:    2026-07-15T17:25:59.226584737Z
status:     running
image ID:   sha256:e4159e3232e82ae58a2cd97c5b0d418be0043ae318ee0478327657e7b5a50de3
image ref:  deploy-api

image created: 2026-07-15T17:25:56.550598544Z
image ID:      sha256:e4159e3232e82ae58a2cd97c5b0d418be0043ae318ee0478327657e7b5a50de3
```

The container and local image IDs match. Inspection of `/app/dist` confirmed:

```text
/app/dist/native-integration/deep-query.service.js:
  argMaxIf(call_key, received_at, call_key != '') AS call_key

/app/dist/pings/pings.filters.js:
  if (!filters.show_demo)
  external_call_id NOT LIKE 'apisync-%'

/app/dist/pings/pings.controller.js:
  show_demo
```

This proves the running API includes both the deep-view correction and default demo-row hiding.

### Web image

```text
container:  deploy-web-1
created:    2026-07-15T13:20:15.196489287Z
status:     running
image ID:   sha256:8115723afc26c709f158885dfade8bfd3af4943f5484256e69445596dc32faab
image ref:  deploy-web

image created: 2026-07-15T12:34:44.431867347Z
image ID:      sha256:8115723afc26c709f158885dfade8bfd3af4943f5484256e69445596dc32faab
```

The container and local image IDs match. Inspection of the compiled `.next` output returned:

```text
aniHash URL pattern (?aniHash=): 0
platform URL pattern (?platform=): 1
show_demo occurrences: 10
```

This proves the running web image contains the Phase 2.0b behavior:

- `aniHash` is no longer placed in call-detail URLs;
- call details obtain ANI hash from the authenticated deep-view response;
- demo-row visibility is controlled through `show_demo`.

### Interrupted build interpretation

The earlier combined API/web build command was interrupted, but that does not mean both artifacts failed:

- the web image had already completed and was later used to create `deploy-web-1`;
- the API was rebuilt again after the final ClickHouse correction;
- current container image IDs match the inspected local images;
- compiled artifact inspection confirms the required Phase 2 code.

## 3. Ringba row provenance and D10 handling

The row previously described as “1 real Ringba row” was:

```text
external_call_id: 16ba2f64-efaa-4245-b4b2-19b98ee72ad8
received_at:      2026-07-15 17:05:04 UTC
event_type:       incoming
state:            TX
```

It came from `scripts/smoke-test-all-platforms.ps1`, which invoked `scripts/smoke-test.ps1 -Platform ringba`:

```text
[ringba] Sending test ping (call_id=16ba2f64-efaa-4245-b4b2-19b98ee72ad8)
Ingestion response: 200
E2E PASS (ringba)
```

The script sends a signed request to the local seeded Ringba development webhook:

```text
http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me
```

It is a local Ringba Dev Pixel/webhook smoke event. It did **not** come from Ringba live API sync and must not count toward D10.

Subsequent ingestion and WebSocket verification generated additional local Ringba test rows. Before D10, use one of these safe controls:

1. wipe known Ringba development/smoke rows and record the zero baseline; or
2. use a strictly bounded live-sync time window and explicitly exclude known test IDs.

Preferred certification procedure: wipe the test rows, capture `0` baseline, run only the live Ringba sync window, and calculate D10 from that bounded window.

## 4. `withAuthContext` revert proof

The unsafe `withAuthContext` helper had appended authorization context to errors before sanitization. It was reverted because Ringba authentication is sent in headers and Go transport errors do not need the credential copied into their error text.

Deployed binary command:

```sh
strings /app/platform-sync | grep -c withAuthContext
```

Result:

```text
0
```

The revert is therefore present in the deployed Phase 2 platform-sync binary and survived commit `b719ec7`.

## Ringba live gate

- [x] Deep-view timestamp fix documented
- [x] Non-empty `call_key` preservation confirmed
- [x] API Phase 2 artifact confirmed
- [x] Web Phase 2.0b artifact confirmed
- [x] `withAuthContext` absent from deployed binary
- [x] Existing Ringba row identified as test traffic
- [ ] Remove/exclude all Ringba development rows and capture D10 zero baseline
- [ ] Run bounded live D8/D9/D10/D5/D8b certification window
- [ ] Return the Ringba integration to Test mode after certification

**Verdict:** technical pre-live confirmations pass. Ringba D10 remains open until test rows are excluded and a clean live-sync volume window is measured.
