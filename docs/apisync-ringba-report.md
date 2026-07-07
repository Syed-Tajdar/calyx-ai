# API Sync — Ringba adapter report

**Status:** Implemented (code complete; live verify requires real credentials + `MOCK_MODE=false`)

## Checklist

- [x] Adapter: `services/platform-sync/internal/adapter/ringba/`
- [x] `POST /{accountId}/CallLogs/Date` with offset pagination
- [x] Rate limit spacing 220ms between requests
- [x] Settings: `api_token` + `account_id`
- [ ] Live verify (`scripts/verify-apisync-ringba-real.ps1`)

## Verify (real)

```powershell
$env:RINGBA_API_TOKEN = "..."
$env:RINGBA_ACCOUNT_ID = "..."
powershell -ExecutionPolicy Bypass -File scripts/verify-apisync-ringba-real.ps1
```
