# API Sync — CallGrid adapter report

**Status:** Implemented (code complete; live verify requires real credentials + `MOCK_MODE=false`)

## Checklist

- [x] Adapter: `services/platform-sync/internal/adapter/callgrid/`
- [x] Cursor pagination (`useCursor`, `searchAfter`)
- [x] 500ms inter-page delay
- [x] Settings: `api_key` (Bearer)
- [ ] Live verify (`scripts/verify-apisync-callgrid-real.ps1`)

## Verify (real)

```powershell
$env:CALLGRID_API_KEY = "..."
powershell -ExecutionPolicy Bypass -File scripts/verify-apisync-callgrid-real.ps1
```
