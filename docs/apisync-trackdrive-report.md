# API Sync — TrackDrive adapter report

**Status:** Implemented (code complete; live verify requires real credentials + `MOCK_MODE=false`)

## Checklist

- [x] Adapter: `services/platform-sync/internal/adapter/trackdrive/`
- [x] Basic auth + subdomain base URL
- [x] Cursor pagination per TrackDrive API
- [x] Settings: `subdomain`, `public_key`, `private_key`
- [ ] Live verify (`scripts/verify-apisync-trackdrive-real.ps1`)

## Verify (real)

```powershell
$env:TRACKDRIVE_SUBDOMAIN = "..."
$env:TRACKDRIVE_PUBLIC_KEY = "..."
$env:TRACKDRIVE_PRIVATE_KEY = "..."
powershell -ExecutionPolicy Bypass -File scripts/verify-apisync-trackdrive-real.ps1
```
