# Ringba Quota / Endpoint Research (recommend only)

**Date:** 2026-07-17  
**Scope:** Research + recommendations. No adapter / quota architecture implementation.  
**Input note:** `calyx_ringba_quota_architecture.md` was **not found** in the repo, Downloads, Desktop, or `.calyx-ai`. Sections §1 / §3 / §4 below are inferred from that prompt + Ringba’s own docs + our current adapter.

---

## Priority 1 — Full 301 body (blocked by our truncation)

### Finding
`checkHTTPStatus` truncates non-2xx bodies to **200 chars**:

```286:294:services/platform-sync/internal/adapter/retreaver_adapter.go
func checkHTTPStatus(status int, body []byte, platform string) error {
	...
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
```

Stored/logged error today:

```text
ringba HTTP 301: {"message":"This reporting endpoint has been moved. Please refer to the api documentation of the new endpoint to be able to accommodate this request on the following page: \r\nhttps://developers.ringb
```

The authoritative new URL is cut mid-host (`developers.ringb…`). Public search does **not** recover the exact destination; [developers.ringba.com](https://developers.ringba.com/) is a JS API portal that does not expose the path in static fetch.

### Recommendation (smallest first build)
1. Lift truncation for non-2xx bodies (or raise to ≥2–4KB / full `maxBodyBytes`).
2. Trigger **one** Live Ringba sync.
3. Paste the **full** 301 JSON (and any `Location` header if Ringba sends one — today `HTTPClient.Do` drops headers on the returned response).
4. Only then change the CallLogs path.

Do **not** guess `/v2/...` from third-party SDKs until that message is captured.

---

## Priority 2 — §1 endpoint / limit table vs Ringba docs

Reconstructed confirmation table (Ringba-owned URLs only where possible):

| Surface | Limit | Scope (Ringba wording) | Citation |
|--------|-------|------------------------|----------|
| Reporting Panel / Summary / Insights / **Exporting call details** / related **Get Logs** API | **5/sec or 80/min** | **Account-wide**: “all users in your account, across this screen and the related API calls” (UI + API share the bucket) | [Why am I being rate limited?](https://support.ringba.com/hc/en-us/articles/23785949612695); [Is there a limit to how many Call Details I can export?](https://support.ringba.com/hc/en-us/articles/23787558523159); [Reporting](https://support.ringba.com/hc/en-us/articles/17994272778519-Reporting-Video) |
| **Get Details about Specific Calls** (`…/calllogs/details` style) | **200/min** | Account-wide across UI + that API | Same rate-limit + export articles |
| **Call logs export** (UI CSV + API export ping) | **5/min, 20/hour, 200/day** | **Per user per account**; “includes any calls made with API tokens issued under the user's account” | [Why am I being rate limited? → Call logs export rate limit](https://support.ringba.com/hc/en-us/articles/23785949612695) |
| Export row/file size | **No** row/file size cap documented | N/A | [Export limits article](https://support.ringba.com/hc/en-us/articles/23787558523159) |
| Export **date range** | **≤ 1 year** per export | UI + API | [Oct 17 2025 release notes](https://updates.ringba.com/release-notes-for-october-17-2025-325270); [Call Details Report](https://support.ringba.com/hc/en-us/articles/17993830584855-Call-Details-Report) |
| Auth | `Authorization: Token {apiToken}` | N/A | [API Access Tokens](https://support.ringba.com/hc/en-us/articles/31661837696023-API-Access-Tokens) |
| Legacy path we hit | `POST /{accountId}/CallLogs/Date` | Now returns **301 moved** (after Token fix) | Live error 2026-07-17; third-party mirror: [CallLogApi.md](https://github.com/diablomedia/ringba-php-sdk/blob/master/docs/Api/CallLogApi.md) |
| CSV export API (exists) | `POST /{accountId}/CallLogs/Date/Export/{format}` + progress `GET …/Export/{jobId}` | Export rate limits above | Same PHP SDK CallLogApi; Plura examples name `calllogs_export_csv` |

**Our throttle today:** dual window 5/sec + 80/min per `account_id` — matches the **Get Logs / Reporting** bucket, not the export-only 20/hr·200/day bucket.

---

## Priority 3 — §3 account-wide vs per-user (dedicated integration user)

### Docs are not actually contradictory once endpoints are separated

| Bucket | Scope | Does a dedicated Ringba user help? |
|--------|-------|-------------------------------------|
| **5/sec · 80/min** (Reporting + Get Logs API — what `CallLogs/Date` used) | **Account-wide** (all admins + UI + API) | **No.** Same account shares one budget. Example in docs: five admins on Reporting + one Get Logs in the same second → fail. |
| **Export** 5/min · 20/hr · 200/day | **Per user per account** (tokens under that user) | **Yes, for export only.** A dedicated user/token gets its own export budget, isolated from humans clicking Export CSV. |

### Recommendation
1. Treat live paginated sync as **account-wide 80/min** — keep per-`account_id` throttle; do not assume a service user doubles capacity for Get Logs.
2. If/when we use CSV export jobs, mint the API token under a **dedicated Ringba user** so export quotas don’t fight humans.
3. Still email Ringba support with this exact question (docs stay easy to misread when both sections sit in one article):

> For `CallLogs/Date` (Get Logs / reporting API), is the 5/sec·80/min limit shared across all users and UI in the account? For `CallLogs/Date/Export/{format}`, is the 20/hr·200/day limit isolated per user (including that user’s API tokens)?

Until they answer, code to the **stricter published reading**: Get Logs = account-wide; Export = per-user.

---

## Priority 4 — §4 caller state: Tags vs Columns (D5 risk)

### Ringba’s own samples
[Ringba/ringba-api-samples README](https://github.com/Ringba/ringba-api-samples/blob/master/README.md):

- `Columns` — main call fields (length, inbound number, …)
- `Tags` — caller details **(country, state, number)** + campaign/target/publisher
- Explicit: *“queries columns to collect inboundPhoneNumber… and **the tags to pull the caller's state**.”*

### Our adapter today
`mapRingbaCallLogEntry` looks up **Columns first, then Tags**, for `state` / `geo:state`:

```169:191:services/platform-sync/internal/adapter/ringba_adapter.go
lookup := func(name string) string {
  // columns then tags
}
stateVal := lookup("state")
if stateVal == "" {
  stateVal = lookup("geo:state")
}
```

So we are **not** Columns-only. Residual D5 risks:
1. Live payloads may put state **only** in tags under a name we don’t try (e.g. `user:state`, `caller:state`).
2. Tag objects may carry `type` + `name` (UI “Tags” tab); we only unmarshal `name`/`value`.
3. Fixture `testdata/ringba_calls.json` puts state in **columns** with empty tags — unit tests won’t catch a tags-only live shape.

### Recommendation
After endpoint works: capture **one raw** `callLog.data[0]` (redacted) and confirm where `state` lives. If tags-only / typed tags → extend lookup (tags-first for geo, or `type:name`). Until then, treat D5 as **at risk**, not proven.

---

## Priority 5 — Max `size` + CSV export existence

| Question | Answer | Confidence | Citation |
|----------|--------|------------|----------|
| Does CallLogs CSV export API exist? | **Yes** — `POST /{accountId}/CallLogs/Date/Export/{format}` (e.g. `csv`) + `GET …/Export/{jobId}` for progress | High (SDK + Ringba articles pointing at API Call Logs page) | [CallLogApi.md](https://github.com/diablomedia/ringba-php-sdk/blob/master/docs/Api/CallLogApi.md); [Export limits](https://support.ringba.com/hc/en-us/articles/23787558523159); [Oct 2025 1-year export window](https://updates.ringba.com/release-notes-for-october-17-2025-325270) |
| Max rows/file for export? | **No documented row/file cap**; **date range ≤ 1 year** | High | Export article + Oct 2025 notes |
| Export request rate? | **5/min, 20/hr, 200/day** per user | High | Rate-limit article “Call logs export” |
| Max accepted `size` on paginated Get Logs? | **Not documented** in Ringba support articles reviewed. Community/Plura examples use `size: 10` (and StackOverflow historically 1000). Our adapter uses **50**. | **Low** — must probe live (try 50 → 100 → 500 → 1000; record first 4xx) | No official max found |

**Quota architecture implication:** large backfills via Get Logs burn the shared 80/min account budget; CSV export is fewer requests but separate per-user caps and async job UX. Prefer: lift 301 → confirm path → measure page `size` ceiling → then decide page-poll vs export-job.

---

## Recommended build order (for your decision)

1. **Lift 200-char truncation** (+ optionally preserve `Location`) → one Live sync → paste full 301 → lock new endpoint.
2. Point adapter at that endpoint; keep `Authorization: Token …`.
3. Keep **account-wide** 5/sec·80/min throttle for Get Logs; use dedicated user **only** if adopting export jobs.
4. Raw-sample state from tags vs columns before calling D5 PASS.
5. Probe max `size`; document; optionally add export path later under per-user export caps + 1-year windows.

## Support email (copy/paste)

Subject: Ringba API rate-limit scope — Get Logs vs Call Logs Export

Hi Ringba Support,

We’re integrating against `POST /v2/{accountId}/calllogs` (Get Call Logs) and may later use `POST /v2/{accountId}/calllogs/export/csv` plus the export progress GET.

Please confirm:

1. For **Get Call Logs** (`POST …/calllogs`), is the **5 requests/second or 80 requests/minute** limit shared across **all users in the account and the Reporting UI**, as described in your rate-limit / Call Details export articles?

2. For **Call logs export** (`POST …/calllogs/export/{format}` and its progress GET), is the **5/min · 20/hour · 200/day** limit applied **per user per account**, including API tokens issued under that user?

3. **Does an export request (and its progress GET) also count against the account-wide 5/sec·80/min Reporting/Get Logs bucket, or ONLY against the per-user export bucket (5/min, 20/hr, 200/day)?**  
   Your docs list “Exporting call details” under the account-wide Reporting limits **and** a separate “Call logs export rate limit” per user — we need to know whether export double-counts, because that changes what we can promise customers about isolation from human Reporting usage.

Thanks,
Calyx AI

---

## Queued mapper items (from RingbaApiRequester sample — applied additively)

| Item | Status |
|------|--------|
| Auth `Authorization: Token {apiToken}` | **DONE** (not sample OAuth/Bearer) |
| Base `/v2/` + `CallLogs/Date` | **DONE** → `POST https://api.ringba.com/v2/{accountId}/CallLogs/Date` |
| Full non-2xx body + headers (no 200-char truncation) | **DONE** |
| State from tags `InboundNumber:Region` (and State) | **DONE** (lookup + valueColumns) |
| `occurred_at` from `dtStamp` ms | **DONE** (additive with `callDt`) |
| Response `result → callLog → data` | **DONE** (additive with `report.records` / `callLog.data`) |
| `pageSize` up to 10000 | **QUEUED / NOT APPLIED** — live API returns 400 `"The maximum allowed Size is 1000."` for 1001; keep cap at 1000 until Ringba changes it |

---

## Build progress (2026-07-17)

### Priority 1 done — full 301 + headers
```
ringba HTTP 301: headers={Content-Type="application/json; charset=utf-8"} body={"message":"This reporting endpoint has been moved. ... https://developers.ringba.com/#cebe3d19-2f2d-48e0-b461-05e7f90837f2","messageCode":0,"errors":[]}
```
- **No `Location` header** on the 301 (body docs URL only).
- Client now uses `CheckRedirect: ErrUseLastResponse` (does **not** auto-follow). Go’s default would follow 301/302 (up to 10) and can rewrite POST→GET / strip Authorization on cross-host redirects.

### Priority 2–5 endpoint lock
From published Postman collection `2sAYBVhryW` (developers.ringba.com):
- New path: `POST https://api.ringba.com/v2/{accountId}/calllogs`
- Max `size`: **1000** (documented in Get Call Log)
- CSV export exists: `…/calllogs/export/csv` + progress GET — **not implemented** until Q3 answered
- Response: `report.records[]` flat (not `callLog.data[]` columns)

### Live proofs (after endpoint + valueColumns)
| Check | Result |
|--------|--------|
| Sync status | **ok** |
| Rows / distinct calls | **52 / 52** |
| `state != ''` | **52 / 52** (from `tag:InboundNumber:State`) |
| `revenue > 0` | **19** (via `conversionAmount` in valueColumns) |
| Max `size` | **1000** accepted; **1001** → HTTP 400 `"The maximum allowed Size is 1000."` |
| Redirects | Client **does not** follow (`ErrUseLastResponse`); 301 had **no Location** |
| Export path | **Not implemented** — waiting on support Q3 |

### Open blockers
- [x] Full 301 destination URL (docs fragment; path locked via published collection)
- [ ] Ringba support Q1–Q3 (especially export double-count)
- [x] Live max `size` = 1000 (400 on 1001)
- [x] Live state = `tag:InboundNumber:State` via `valueColumns`
