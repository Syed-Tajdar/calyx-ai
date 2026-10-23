# Aryan — run order for 2026-10-23

**Two hashes only:**
- **`6a8ad8a`** for item 1;
- **`6a34fd1`** for items 3–6. This is the code-complete main. GitHub `main` is one **docs-only** commit ahead (the one
  that wrote this hash into the packs). The server does not need that commit, and `git merge --ff-only 6a34fd1` is
  correct.

Never a plain `git pull`: main may move on after these docs. Each pack moves the checkout with
`git fetch origin && git merge --ff-only <its hash>`. **Read every pack on GitHub `main`**, not from the server's checkout,
which is pinned to an older commit.

**Rules for all six:**
- **Never** `docker compose down`, never `-v`, always `-p deploy`. One terminal; each block sets its own variables.
- No secret in chat. Plane and trackitin untouched.
- **Not between 08:30 and 10:00 PKT** for item 1 (its recon before and after must be on the same Eastern day).
- Any `STOP`, non-200, dirty migration or `permission denied` → stop, send the output to Syed, do not retry.
- Items run **in this order**, each only after Syed's OK on the one before.

| # | What | Hash | Expected time | Send to Syed | Stop rules |
|---|---|---|---|---|---|
| 1 | `docs/deploy-v2-2d-2b.md` **Steps 0–6(c)**: 2D + 2B, migrations 35 + 36; then Ask AI, alerts (actions off), transcription with the **button only**. **Not 6(d).** | `6a8ad8a` | ~3 h (builds ~15 min, recon wait 30 min, 6(a)–6(c) ~1.5 h with Syed) | Step 0, Step 1 counts + recon, Step 3, Step 4 "set/empty" lines, Step 5 (+ `7-day: IDENTICAL`), 6(a) 5 answers + queries, 6(b) dry run + query, 6(c) job/recording/transcript tables + 0 URL lines | Checkout must show **0** `000037` files; migration must be `36 \| f`; platform-sync `restarting=true` at 6(c) → Rollback 3; Ask AI mismatch → Rollback 1 |
| 2 | **Pause** for Syed's management demo. Change nothing on the server. | — | Syed's call | — | No restarts, no env edits, no builds |
| 3 | `docs/deploy-37-grants.md`: backup, move to the final main, migrate to 37, checks. **No rebuild, no restart.** | `6a8ad8a` → `6a34fd1` | ~30 min | Step 0 (`36 \| f`, count 13), Step 2 (`37 \| f`, count 0, the 26-row role table), Step 3 (sync tick, pages, Syed's 5 browser checks, **0** `permission denied`) | Any `permission denied` line → its Rollback at once (back to `36 \| f` on `6a8ad8a`) |
| 4 | `docs/ops-daily-backups.md`: **§1** manual run → **§3** restore test (throwaway containers) → **§2** cron (03:30 PKT) | `6a34fd1` (no pull) | ~45 min | §1 output (`BACKUP OK`, `exit=0`, `du -sh`), §3 output (`RESTORE TEST: PASS`), §2 `crontab -l` | §1 without `BACKUP OK` / `exit=0`; `checksums: FAILED`; §3 without `RESTORE TEST: PASS`; free space under 10 GB (the script stops itself) → stop, no cron |
| 5 | Report §2D.6 **Step 0** probe, **GET only**. Create the temporary worktree at `6a34fd1` instead of `origin/v2/2d-on-main`: same probe code, only `go.mod` differs by 2B's archive dependency. At migration 37 it prints real `is_active` / `allow_publisher_pause` instead of `unknown`. | `6a34fd1` (worktree; the server checkout stays) | ~20 min | `~/calyx-golive/probe-2d-step0.txt` | **No pause, no unpause, no write of any kind.** Remove the worktree at the end |
| 6 | ~~6(d) global automatic transcription~~ — **NOT today (owner decision 2026-10-23).** `TRANSCRIBE_AUTO` stays `false`. It is replaced by the **Transcription Rules** phase (`docs/v2-2b2-transcription-rules-spec.md`): scoped campaigns, sampling and triggers, each with its own cap. Do **not** run `docs/deploy-v2-2d-2b.md` 6(d). | — | — | — | If anyone asks to switch `TRANSCRIBE_AUTO` on today: no, send them to Syed |

**Notes**
- After item 3, the server checkout stays on `6a34fd1` for items 4 and 5. Nothing pulls again.
- Item 1's full rollback is in that pack. If item 3 has already run, run the 37 pack's Rollback first.
- **Backfill is never automatic.** It is only manual: per selected campaign and date range, with the cost estimate
  first (Transcription Rules phase). `TRANSCRIBE_BACKFILL` stays `false`.
- `TRANSCRIBE_AUTO` stays `false` (item 6 is not today). Transcription today is the admin **Transcribe** button only.
- Publisher actions (`WORKFLOW_ACTIONS_ENABLED`) stay `false`.
