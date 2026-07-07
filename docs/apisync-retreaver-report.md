# API Sync — Retreaver adapter report

**Status:** Implemented (code complete; live verify requires real credentials + `MOCK_MODE=false`)

## Checklist

- [x] API researched in `docs/apisync-platform-research.md`
- [x] Adapter: `services/platform-sync/internal/adapter/retreaver/`
- [x] Unit test: `adapter_test.go` (mapCall + cursor)
- [x] Shared framework: PlatformAdapter, sync_status migration
- [x] Settings: dynamic `api_key` + `company_id` fields
- [ ] Live verify with real Retreaver key (`scripts/verify-apisync-retreaver-real.ps1`)

## Verify (mock)

```powershell
make verify-api-sync
```

## Verify (real)

Set `MOCK_MODE=false` on `api` and `platform-sync`, then:

```powershell
$env:RETREAVER_API_KEY = "..."
$env:RETREAVER_COMPANY_ID = "..."
powershell -ExecutionPolicy Bypass -File scripts/verify-apisync-retreaver-real.ps1
```
