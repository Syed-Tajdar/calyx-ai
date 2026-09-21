# Write-back contain report

**Date:** 2026-09-19  
**Spec:** user-supplied CONTAIN steps (no `docs/writeback-contain.md` in repo; this report follows that text).  
**Mode:** Contain only. Handler code kept. Migrations and read/mapping path not changed.

---

## C1 — platform-sync convert route not registered

**Change:** `WRITEBACK_CONVERT_ENABLED` loaded in [`config.go`](../services/platform-sync/internal/config/config.go); default **off** (only `true` or `1` enables). [`cmd/server/main.go`](../services/platform-sync/cmd/server/main.go) registers `POST /writeback/convert` **only** when that flag is on. `handleWritebackConvert` remains in the file. Flag **not** set in compose (unset).

| Probe | Before | After |
|-------|--------|-------|
| `POST :8088/writeback/convert {}` | **400** (this pass, before rebuild) | **404** empty body |

Compose does not set `WRITEBACK_CONVERT_ENABLED`. Container env after recreate: `[]` (unset).

---

## C2 — Nest convert route not registered

**Change:** same flag. [`integrations.controller.ts`](../services/api/src/integrations/integrations.controller.ts) applies `@Post(':id/writeback/convert')` only when `WRITEBACK_CONVERT_ENABLED` is `true` or `1`. Method `convertWriteback` remains. Flag unset on `api` container (`[]`).

| Probe | Before | After |
|-------|--------|-------|
| `POST :3000/integrations/{Ringba Live id}/writeback/convert` + JWT | **404** — **not** a missing route. platform-sync log `writeback convert failed` `ringba: call not found in 7d window` (LookupCall read; webhook **not** fired) | **404** — no new platform-sync convert log |
| same without JWT | **401** | **404** |

After-contain Nest 404s had empty bodies in this client. platform-sync received **no** convert request after recreate.

---

## C3 — fire conditions

### C3.1 Ringba Live → Test (`sync_mode=mock`)

`ce6fd338-af48-47d8-850a-bd10b855e2f4`

| | `is_active` | `sync_mode` |
|--|-------------|-------------|
| Before | `f` | `real` |
| After | `f` | `mock` |

`UPDATE 1`. Ticker still paused.

### C3.2 Allowlist emptied

[`deploy/docker-compose.yml`](../deploy/docker-compose.yml) `platform-sync`:

| | `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS` |
|--|----------------------------------------|
| Before | `CA98abf9d92be446e786f298680a3bbee6` |
| After | `""` (container env `[]` / empty) |

Boot log after recreate:

```
writeback_allowlist_configured":false
```

Before-contain boot (still in older log history): `writeback_allowlist_configured":true`.

---

## C4 — smokes after contain

| Probe | Expected | Got |
|-------|----------|-----|
| `POST :8088/writeback/convert {}` | 404 | **404** |
| `POST :8088/writeback/convert` valid-looking body | 404 | **404** |
| `POST :3000/.../writeback/convert` + JWT | 404 | **404** |
| Nest without JWT | 404 (was 401) | **404** |

No `writeback convert failed` lines after platform-sync recreate.

**Verdict: convert write reachable today — NO.** Routes are not registered (`WRITEBACK_CONVERT_ENABLED` unset/default off). Even if they were re-enabled, `sync_mode=mock` and empty allowlist refuse a fire. UI still has no convert control (untouched).

---

## C5 — left untouched

- ClickHouse `has_converted`, adapter mapping, deep-view Converted field
- Migrations `000024`, `000012` (not reverted)
- `cmd/probe-ringba-*`
- Ringba webhooks

---

## Differs from the audit (reported, not fixed)

1. **`docs/writeback-contain.md` is not in the repo.** Steps were taken from the user message.
2. **Nest + JWT before contain was already HTTP 404** on this id + fake call id, because LookupCall returned not-found — **not** because the Nest route was missing. Proof: platform-sync `ringba: call not found in 7d window`. After contain, the same 404 is a **missing route** (no convert log).
3. That before-contain JWT probe performed a Ringba **Get Call Log read** (7d scan). It did **not** call `FireInboundConversion`.
4. Go 404 bodies were empty (`net/http` default). Audit smokes quoted status codes only.
5. `.env.example` still comments the sandbox allowlist ID. Compose is empty; example file was not in C1–C5.
