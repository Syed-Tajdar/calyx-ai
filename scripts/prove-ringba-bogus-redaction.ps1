# Prove Ringba header-token redaction (bogus token) BEFORE real credentials

Requires rebuilt platform-sync with RINGBA_BASE_URL support.

```powershell
# 1) Point Ringba at an unresolvable host (transport error path)
# In deploy/docker-compose.yml platform-sync environment, temporarily set:
#   RINGBA_BASE_URL: https://this-host-does-not-exist-xyz.invalid
# Then: docker compose up -d --force-recreate platform-sync

$api = "http://localhost:3000"
$login = Invoke-RestMethod "$api/auth/login" -Method POST -ContentType "application/json" `
  -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
$h = @{ Authorization = "Bearer $($login.access_token)" }

$body = @{
  name = "bogus-ringba-redaction"
  platform = "ringba"
  connection_mode = "api_sync"
  sync_mode = "real"
  api_token = "BOGUS_RINGBA_TOKEN_XYZ"
  account_id = "999999"
} | ConvertTo-Json

$created = Invoke-RestMethod "$api/integrations" -Method POST -Headers $h -ContentType "application/json" -Body $body
Write-Host "created $($created.id)"
Invoke-RestMethod "$api/integrations/$($created.id)/sync" -Method POST -Headers $h | Out-Null
Start-Sleep 8

$leak = (docker logs deploy-platform-sync-1 2>&1 | Select-String -Pattern "BOGUS_RINGBA_TOKEN_XYZ" -SimpleMatch | Measure-Object).Count
$redacted = (docker logs deploy-platform-sync-1 2>&1 | Select-String -Pattern "REDACTED" | Measure-Object).Count
Write-Host "token_leak_count=$leak (MUST be 0)"
Write-Host "redacted_count=$redacted (MUST be >= 1)"

docker exec deploy-postgres-1 psql -U callloom -d callloom -c `
  "SELECT name, sync_status, left(coalesce(last_sync_error,''),120) AS err FROM integrations WHERE name LIKE '%bogus%' ORDER BY updated_at DESC LIMIT 1;"

# 2) Delete bogus integration via API/Settings; remove RINGBA_BASE_URL override; recreate platform-sync.
if ($leak -ne 0) { exit 1 }
Write-Host "BOGUS REDACTION PASS"
exit 0
