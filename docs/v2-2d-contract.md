# Phase 2D — internal contract (API ↔ platform-sync) and build rules

Source of truth for both halves of Phase 2D. Spec: `docs/v2-intelligence-spec.md` Phase 2D; facts:
`docs/v2-intelligence-report.md` §5; owner decisions Q10, Q11 and 2a. Schema: migration `000034_workflows`.

## Switches (all off by default)

| Switch | Where | Meaning |
|---|---|---|
| `FEATURE_WORKFLOWS` / `NEXT_PUBLIC_FEATURE_WORKFLOWS` | API (runtime) / web (build) | Alerts tab, alert centre and `/workflows/*` routes exist only when on (global `@FeatureGate('workflows')` → 404 otherwise) |
| `WORKFLOW_ACTIONS_ENABLED` | API **and** platform-sync | Kill switch. Off → rules can alert and suggest, but nothing is ever written to Ringba or Retreaver |
| `integrations.allow_publisher_pause` | Postgres, per integration | Admin toggle "Allow Calyx to pause publishers". Live (`sync_mode='real'`, active) integrations only |
| `WORKFLOW_RINGBA_PAUSE_VERIFIED` | API and platform-sync | Off (default) until the sandbox proof shows Ringba `PATCH /Publishers/{id}` `{"enabled":false}` equals the UI pause. While off: Ringba = Suggest **with a link to the publisher in Ringba**, no write, Auto disabled for Ringba |
| `WORKFLOW_MAX_AUTO_PAUSES_DAILY` | API | Default 3 per tenant per Eastern day; after that Suggest only + an alert |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`, `ALERT_EMAIL_TO` | API | Email is off while `SMTP_HOST` is empty. The bell works without email |

## Internal endpoints (platform-sync, Bearer `WRITEBACK_INTERNAL_TOKEN`, same as write-back)

`POST /publisher/pause` and `POST /publisher/unpause`

Request (JSON, strict):
```json
{ "tenant_id": "uuid", "integration_id": "uuid", "publisher_id": "string",
  "user_id": "uuid|null", "mode": "manual|auto|timer", "rule_id": "uuid|null",
  "alert_event_id": "uuid|null", "numbers": {} }
```

Response 200:
```json
{ "outcome": "confirmed|confirmed_by_response", "paused": true, "read_back": "ok|none",
  "message": "…", "audit_id": "uuid" }
```

Errors (body `{"message": "...", "outcome": "rejected|failed", "audit_id": "uuid|null"}`):
- **403** kill switch off · integration not Live / paused / Test · `allow_publisher_pause` false ·
  Ringba while `WORKFLOW_RINGBA_PAUSE_VERIFIED` is off · unsupported platform
- **404** publisher not found on the platform
- **409** another pause/unpause for the same tenant+platform+publisher is in progress (Redis lock)
- **502** platform error after retries, or read-back contradicts the write

Every attempt, including each rejection above, writes one `publisher_action_audit` row through
`insert_publisher_action_audit(...)`. Messages and `request_redacted` go through `internal/redact`; no key,
token or platform URL query is ever stored or logged.

## Platform calls

| | Ringba | Retreaver |
|---|---|---|
| Read | `GET https://api.ringba.com/v2/{accountId}/Publishers/{publisherId}` → `enabled` | `GET /api/v1/affiliates/afid/{afid}.json` (Phase 0 saw **404** for this form on one account; on 404 fall back to listing `GET /api/v1/affiliates.json` pages to find the system id for that `afid`, then `GET /api/v1/affiliates/{id}.json`) |
| Pause | `PATCH …/Publishers/{publisherId}` body `{"enabled": false}` | `PUT /api/v1/affiliates/{id or afid/{afid}}.json` body `{"affiliate":{"paused":true}}` |
| Unpause | same with `true` | same with `false` |
| Read-back | `enabled` must equal the written value → `read_back: ok`, `outcome: confirmed`; contradiction → 502 | If the GET body has a `paused` key: compare (`ok` / 502). If not (documented response has none): `read_back: none`, `outcome: confirmed_by_response` (owner Q10) |
| Auth | `Authorization: Token {apiToken}` (as the sync adapter) | `api_key` + `company_id` query (as the sync adapter) |
| Limiter | shared Ringba limiter | shared Retreaver per-company limiter (2/s, 60/min) |

Retry only 429 and 5xx (max 3, backoff, `Retry-After`), never a 4xx, never a transport error. Calyx never
deletes a publisher and never changes payouts, campaigns, targets or routing.

## API side guardrails (before calling platform-sync)

1. `FEATURE_WORKFLOWS` on (else 404), admin only (403), CSRF (global guard).
2. Kill switch off → no call; alert records `action = pause_blocked`, audit row `rejected` (mode auto/manual).
3. Minimum sample: no rule fires on a window with fewer than `min_calls` calls (default 20).
4. Auto only if the rule has been in Suggest for ≥ 7 days (`suggest_since`); the API refuses to save
   `action_mode = 'auto'` earlier, and the evaluator re-checks.
5. Daily limit `WORKFLOW_MAX_AUTO_PAUSES_DAILY` auto pauses per tenant per Eastern day.
6. Protected publishers: never auto-paused (Suggest only).
7. Cooldown: one alert per rule × entity per `cooldown_hours` (default 6).
8. A pause is global for the publisher: the confirm popup shows that in bold with the publisher's calls and
   revenue for the last 7 days.
9. Unpause is manual (admin) or the rule's `pause_hours` timer (evaluator tick, `mode: timer`).
10. Every blocked attempt is audited too (the API inserts `rejected` rows itself for guardrail blocks).
