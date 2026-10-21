# v1 sync health, Part 1 items 1, 5 and 6 — a redis outage is visible, nothing crash-loops, and
# everything recovers by itself.
#
# LAPTOP ONLY. Never run on the server.
#
# The outage is the 2026-10-18 one: redis running but without a network, so the name `redis`
# does not resolve (`docker network disconnect`). While it lasts:
#   - platform-sync logs ONE "redis unavailable" error (no per-tick flood) and every api_sync
#     integration goes degraded, then failed, with last_sync_error saying redis is unavailable;
#   - ingestion stays Up — including after its own restart with redis gone — and /readyz is 503.
# After `docker network connect --alias redis`, without touching any other container:
#   - "redis reachable again" is logged, a sync completes, sync_status is ok, the error is cleared
#     and last_success_at moves; ingestion /readyz is ok again.
#
# Needs the laptop stack with the dev ports (scripts/dev-up.ps1) and a Test-mode integration,
# which this script creates and pauses again at the end. No Live integration, no platform call.
param([int]$OutageSeconds = 420)

$ErrorActionPreference = "Continue"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot
$apiBase = "http://localhost:3100"
$script:pass = 0
$script:fail = 0

function Check([bool]$ok, [string]$what) {
  if ($ok) { $script:pass++; Write-Host "  PASS $what" -ForegroundColor Green }
  else { $script:fail++; Write-Host "  FAIL $what" -ForegroundColor Red }
}

function Psql([string]$sql) {
  return (docker exec deploy-postgres-1 psql -U callloom -d callloom -tAc $sql | Out-String).Trim()
}

function Ready([string]$url) {
  try { return (Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 15).StatusCode } catch {
    if ($_.Exception.Response) { return [int]$_.Exception.Response.StatusCode }
    return 0
  }
}

function RestartCount([string]$c) { return [int]((docker inspect -f '{{.RestartCount}}' $c | Out-String).Trim()) }

Write-Host "=== Redis outage (laptop) ===" -ForegroundColor Cyan

$login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST -ContentType "application/json" `
  -Headers @{ "X-Calyx-Client" = "script" } -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
$headers = @{ Authorization = "Bearer $($login.access_token)" }
$testName = "redis-outage-test-mock-" + (Get-Date -Format "yyyyMMddHHmmss")
$body = @{ platform = "retreaver"; name = $testName; connection_mode = "api_sync"; sync_mode = "mock";
           api_key = "mock-retreaver-key-outage-test"; company_id = "mock-company" } | ConvertTo-Json
$created = Invoke-RestMethod -Uri "$apiBase/integrations" -Method POST -Headers $headers -ContentType "application/json" -Body $body
$id = $created.id
Check ([bool]$id -and $created.sync_mode -eq "mock") "Test-mode integration created ($id)"
if (-not $id) { Write-Host "Cannot continue without the test integration." -ForegroundColor Red; exit 1 }

# Let one healthy sync land first, so last_success_at has a value to hold.
$deadline = (Get-Date).AddSeconds(200)
while ((Get-Date) -lt $deadline -and (Psql "SELECT sync_status FROM integrations WHERE id='$id'") -ne "ok") { Start-Sleep -Seconds 5 }
$successBefore = Psql "SELECT last_success_at FROM integrations WHERE id='$id'"
Check ($successBefore -ne "") "healthy sync before the outage (last_success_at $successBefore)"

$ingRestartsBefore = RestartCount "deploy-ingestion-1"
$outageAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
docker network disconnect deploy_default deploy-redis-1 | Out-Null
Check ($LASTEXITCODE -eq 0) "redis disconnected from deploy_default (name 'redis' no longer resolves)"

# ingestion restarted while redis is gone: it must come up and stay up (2026-10-18: crash loop).
docker restart deploy-ingestion-1 | Out-Null
$ingRestartsAfterManual = RestartCount "deploy-ingestion-1"

Write-Host "Outage running for up to $OutageSeconds s (SYNC_FAILED_AFTER=3 ticks of 120 s)..." -ForegroundColor Yellow
$deadline = (Get-Date).AddSeconds($OutageSeconds)
while ((Get-Date) -lt $deadline -and (Psql "SELECT sync_status FROM integrations WHERE id='$id'") -ne "failed") { Start-Sleep -Seconds 10 }

$row = Psql "SELECT sync_status || '|' || COALESCE(last_sync_error,'') FROM integrations WHERE id='$id'"
Check ($row -match '^failed\|') "integration is failed after repeated failures ($row)"
Check ($row -match 'redis unavailable') "last_sync_error says redis is unavailable"
Check ((Psql "SELECT last_success_at FROM integrations WHERE id='$id'") -eq $successBefore) "last_success_at did not move during the outage"

$psLogs = docker logs --since $outageAt deploy-platform-sync-1 2>&1 | Out-String
$errLines = ([regex]::Matches($psLogs, '"msg":"redis unavailable')).Count
Check ($errLines -eq 1) "platform-sync logged the outage once (got $errLines)"
Check (-not ($psLogs -match '"msg":"sync failed"')) "no per-integration 'sync failed' flood"

$ingState = (docker inspect -f '{{.State.Status}}|{{.State.Restarting}}' deploy-ingestion-1 | Out-String).Trim()
Check ($ingState -eq "running|false") "ingestion is Up, not Restarting ($ingState)"
Check ((RestartCount "deploy-ingestion-1") -eq $ingRestartsAfterManual) "ingestion did not crash-loop (restart count unchanged)"
Check ((Ready "http://127.0.0.1:8080/readyz") -eq 503) "ingestion /readyz is 503 while redis is down"
$ingLogs = docker logs --since $outageAt deploy-ingestion-1 2>&1 | Out-String
Check ($ingLogs -match '"msg":"redis unavailable') "ingestion logged a clear redis error"

# Recovery: reconnect redis only. Nothing else is restarted.
$recoverAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
docker network connect --alias redis deploy_default deploy-redis-1 | Out-Null
Check ($LASTEXITCODE -eq 0) "redis reconnected with alias redis"

$deadline = (Get-Date).AddSeconds(240)
while ((Get-Date) -lt $deadline -and (Psql "SELECT sync_status FROM integrations WHERE id='$id'") -ne "ok") { Start-Sleep -Seconds 5 }
$row = Psql "SELECT sync_status || '|' || COALESCE(last_sync_error,'') || '|' || consecutive_failures FROM integrations WHERE id='$id'"
Check ($row -eq "ok||0") "recovered by itself: ok, error cleared, failure count 0 ($row)"
$successAfter = Psql "SELECT last_success_at FROM integrations WHERE id='$id'"
Check ($successAfter -ne $successBefore -and $successAfter -ne "") "last_success_at moved on recovery ($successAfter)"
$recLogs = docker logs --since $recoverAt deploy-platform-sync-1 2>&1 | Out-String
Check ($recLogs -match 'redis reachable again') "platform-sync logged the recovery"
$deadline = (Get-Date).AddSeconds(60)
while ((Get-Date) -lt $deadline -and (Ready "http://127.0.0.1:8080/readyz") -ne 200) { Start-Sleep -Seconds 3 }
Check ((Ready "http://127.0.0.1:8080/readyz") -eq 200) "ingestion /readyz ok again without a restart"
Check ((RestartCount "deploy-ingestion-1") -eq $ingRestartsAfterManual) "ingestion was not restarted to recover"

docker exec deploy-postgres-1 psql -U callloom -d callloom -c "UPDATE integrations SET is_active = FALSE, updated_at = NOW() WHERE id = '$id'::uuid AND name = '$testName';" *> $null
Write-Host "Test integration paused (not deleted)."

Write-Host ""
Write-Host "Redis outage: $script:pass passed, $script:fail failed"
if ($script:fail -gt 0) { Write-Host "REDIS OUTAGE: FAIL" -ForegroundColor Red; exit 1 }
Write-Host "REDIS OUTAGE: PASS" -ForegroundColor Green
exit 0
