# Stage 4b handoff — Retreaver convert / void (cloud session → local Claude Code)

Written 2026-10-13 by the cloud session that did Phase 0–1. Read this, then
`docs/retreaver-writeback-spec.md` (source of truth) and `docs/retreaver-writeback-report.md`
(Phase 0–1 results, design, test list, owner decisions) before doing anything.

## Where things stand

- **Branch:** `stage4b/retreaver-writeback` @ `9c3716c` ("stage4b: merge main 835b9f7"), pushed.
  Built from `main` @ `14c3068`, then merged `main` @ `835b9f7` (v1 go-live prep). No conflicts.
- **Done:** Phase 0 (docs verified, quoted), Phase 1 (registry + Retreaver Convert / Void /
  ReadBack + UI + tests), owner decisions 1–3, merge of main.
- **Tests on `9c3716c`:** platform-sync `go test ./...` all ok; API jest 64/64; web
  `node --test` 24/24; `next build` ok. Existing Ringba write-back and Retreaver sync test files
  are unchanged.
- **Not done:** Phase 2 (GET only, plan), Phase 3 (sandbox run), Phase 4 (local end-to-end),
  `pwsh-preview -V1` gates.

## Owner decisions already taken (do not re-ask)

| Topic | Decision |
|---|---|
| Redaction output | Keep the R1 helper's `[REDACTED]`. `***` is only for chat/report display. |
| Unsettled read-back | Same rule for Ringba and Retreaver: write errors after retries → `failed`; read-back not matching yet → `pending` (202, lock held); contradicting value → `failed`. Implemented in `f1df7e3`. |
| UI toast | Fixed in `25412c8` (`confirmed` no longer shows "not confirmed yet"). |
| Read-back version (Q3) | Keep `GET /api/v2/calls/{uuid}.json`. |
| Re-convert a converted call (Q4) | Keep 409: void first, then convert. |
| Pending lock (Q9) | Keep the 15-minute pending lock. |

## Local stack on the owner's Windows machine

- Repo: `C:\calyx-4b` (branch above). Compose project **`calyx-4b`**:
  `docker compose -p calyx-4b --env-file .env -f deploy/docker-compose.yml …`
  Always pass `--env-file .env`: `deploy/.env` is a symlink in git and is a plain text file on
  Windows.
- `C:\calyx-4b\.env` is local only, git-ignored, made from `deploy/.env.example` with
  `APP_ENV=development`, `NEXT_PUBLIC_API_URL=http://localhost:3100`,
  `NEXT_PUBLIC_WS_URL=ws://localhost:3100`, `WRITEBACK_ENABLED=false`, `COMPOSE_PROFILES=`; other
  secrets empty = compose dev defaults. **Never change it** (the credential encryption key
  must not change) and never commit it.
- Verified 2026-10-13 04:31: v1 services Up, migration version 32 dirty=false, healthz
  8080/8081/3100/8088 = 200, login `admin@calyx.ai` / `calyx_dev` = 201 at http://localhost:3101.
- Integrations at that time: 4 dev webhook seeds from migrations (no API keys).
- **Old stack:** compose project `deploy` (`C:\Users\DELL\calyx-AI\deploy\docker-compose.yml`),
  18 containers, all **stopped, none removed**. Volumes kept: `deploy_postgres_data`,
  `deploy_clickhouse_data`, `deploy_redis_data`, `calyx-go-cache`, `calyx-go-mod`, plus 3
  anonymous volumes. The owner decides deletion later. Its containers have a restart policy, so
  Docker Desktop may start them again on restart — check `docker compose ls` and stop (never rm)
  them if they come back, because they use the same ports.
- Helper scripts used: `C:\Projectscalyx-stage4b\stage4b-BCD-v3.ps1` (inventory, stop old
  containers, build one image at a time with retries, up, checks), log in
  `C:\Projectscalyx-stage4b\logs\stage4b-BCD.log`.

## Next step: Phase 2 (GET only) — exactly per the spec

**Precondition — 🙋 owner action.** The owner adds the Retreaver integration in local
Settings: API key mode, **Test** (never Live), name "My test", API key + Company ID entered in
the UI only. Then the owner **pauses** it and says "done". Never ask for the key in chat.

**Sandbox call:** `a82a7a80-f1fd-46a5-a018-379dd7893ea7`, campaign `CALYX-SANDBOX-DO-NOT-USE`.

Then:
1. Credentials come only from that stored integration, decrypted in-process. Run the probe in a
   `golang` container on the `calyx-4b` Docker network, reading Postgres there; use
   `CREDENTIAL_ENCRYPTION_KEY` from the running platform-sync container's environment. Do not
   print the key or the company id. Route all output through `internal/redact`.
2. With **GET only**, read the call (`/api/v2/calls/{uuid}.json`), its campaign, target and
   affiliate (publisher). **Prove** that no pixel or postback is configured and that the terms
   are $0. If this cannot be proven, **stop 🙋**.
3. Record the call's current `converted`, `revenue`, `payout`, `receivable`, `payable`,
   `fired_pixels_count`, `conversions_determined_at`, `postback_value` and `updated_at`.
4. Write the a–i plan table (spec, Phase 2) into `docs/retreaver-writeback-report.md`, with
   the key redacted.
5. Secret-scan, commit `stage4b: …`, push **this branch only**, give the hash, **STOP**.
   No PUT until the owner approves the plan in writing.

## Hard rules (from the project)

- Never print, log or commit secrets. The Retreaver key travels in the URL, so every
  log/error/audit/test output goes through the redaction helper.
- No Retreaver or Ringba write (PUT/POST) until the owner approves the written Phase 2 plan.
  After approval: only the one sandbox call above, amounts ≤ $1, stop at once if
  `fired_pixels_count` increases.
- Never set any integration to Live on this machine. The server is the only Live sync.
- `WRITEBACK_ENABLED` stays false in every default (Phase 4 may set it true locally, then back).
- Never send `session_notes` or `payout` to Retreaver.
- Never run `docker compose down -v`, `docker rm`, `docker volume rm` or prune.
- Label claims `[PROVEN]` / `[PROVEN-docs]` (with URL) / `[INFERRED]` / `[UNKNOWN]`.
- One commit per item, prefix `stage4b:`. Secret-scan before every push. Never push to `main`,
  never merge; the owner decides merges.
- Stop at every 🙋 and at the end of each phase with commit hashes and a short summary.
- Talk to the owner in simple Roman Urdu/English; code, commands and reports in English.
