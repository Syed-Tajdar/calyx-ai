# v1 sync health, Part 1 item 4 — Calyx survives a reboot whatever order Plane starts in.
#
# LAPTOP ONLY. Never run on the server.
#
# Reproduces 2026-10-18: after a reboot Plane's redis took host port 6379 first and Calyx's redis
# came up without a network. Here a stand-in container (restart=unless-stopped, like Plane) holds
# host 6379 and every other host port Calyx used to publish. Calyx runs with the SERVER compose
# file only (no docker-compose.dev.yml). Then the whole Calyx stack is stopped and started again
# while the stand-in keeps the ports — Plane "wins" every port first, as after the reboot.
#
# This script does NOT restart the Docker engine: on 2026-10-21 `docker desktop restart` crashed
# Docker Desktop on this laptop. The engine-restart proof of that day (all 9 services back by
# their restart policy, without a recreate) is recorded in docs/deploy-v1-sync-health.md.
#
# PASS needs: all 9 services running (none Restarting), each `unless-stopped`; redis on
# deploy_default with alias "redis" and no host port; Calyx publishes only 3100 and 3101; no
# "lookup redis" in platform-sync; a completed sync after the restart; ingestion ready.
#
# Uses a Test-mode (mock) api_sync integration only: no Live integration, no platform API call.
#
#   powershell -ExecutionPolicy Bypass -File scripts/verify-reboot-resilience.ps1
param([int]$SyncWaitSeconds = 300)

$ErrorActionPreference = "Continue"
$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot
$apiBase = "http://localhost:3100"
$standin = "calyx-standin-plane"
$services = @("postgres", "redis", "redpanda", "clickhouse", "ingestion", "consumer", "api", "platform-sync", "web")
$formerHostPorts = @(6379, 5432, 9092, 8123, 9100, 8080, 8081, 8088)
$script:pass = 0
$script:fail = 0

function Check([bool]$ok, [string]$what) {
  if ($ok) { $script:pass++; Write-Host "  PASS $what" -ForegroundColor Green }
  else { $script:fail++; Write-Host "  FAIL $what" -ForegroundColor Red }
}

function Wait-Engine([int]$seconds) {
  $deadline = (Get-Date).AddSeconds($seconds)
  while ((Get-Date) -lt $deadline) {
    docker info --format '{{.ServerVersion}}' *> $null
    if ($LASTEXITCODE -eq 0) { return $true }
    Start-Sleep -Seconds 5
  }
  return $false
}

function Get-State([string]$svc) {
  $s = docker inspect -f '{{.State.Status}}|{{.State.Restarting}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}|{{.HostConfig.RestartPolicy.Name}}' "deploy-$svc-1" 2>$null
  if ($LASTEXITCODE -ne 0 -or -not $s) { return $null }
  $p = $s.Trim().Split('|')
  return @{ Status = $p[0]; Restarting = $p[1]; Health = $p[2]; Policy = $p[3] }
}

function Wait-Stack([int]$seconds) {
  $deadline = (Get-Date).AddSeconds($seconds)
  while ((Get-Date) -lt $deadline) {
    $ready = $true
    foreach ($svc in $services) {
      $st = Get-State $svc
      if (-not $st -or $st.Status -ne "running" -or $st.Restarting -eq "true" -or ($st.Health -ne "none" -and $st.Health -ne "healthy")) { $ready = $false; break }
    }
    if ($ready) { return $true }
    Start-Sleep -Seconds 5
  }
  return $false
}

function Get-Token {
  $login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST -ContentType "application/json" `
    -Headers @{ "X-Calyx-Client" = "script" } -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
  return @{ Authorization = "Bearer $($login.access_token)" }
}

Write-Host "=== Reboot resilience (laptop, server compose file only) ===" -ForegroundColor Cyan

# 1. Server layout first, so the laptop dev ports are released; then the stand-in takes them.
docker rm -f $standin *> $null
docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --remove-orphans *> $null
Wait-Stack 300 | Out-Null
$portArgs = @()
foreach ($p in $formerHostPorts) { $portArgs += @("-p", "127.0.0.1:${p}:6379") }
docker run -d --name $standin --restart unless-stopped @portArgs redis:7-alpine | Out-Null
Check ($LASTEXITCODE -eq 0) "stand-in holds host ports $($formerHostPorts -join ', ')"

# 2. Calyx recreated again while the stand-in holds the ports: nothing may need them.
docker compose --env-file .env -f deploy/docker-compose.yml up -d --force-recreate --remove-orphans *> $null
Check ($LASTEXITCODE -eq 0) "Calyx starts while the ports are taken"
Check (Wait-Stack 300) "all 9 services running before the restart"

# 3. A Test-mode api_sync integration so there is something to sync (mock adapter, no network).
$headers = Get-Token
$testName = "reboot-test-mock-" + (Get-Date -Format "yyyyMMddHHmmss")
$body = @{ platform = "ringba"; name = $testName; connection_mode = "api_sync"; sync_mode = "mock";
           api_token = "mock-token-reboot-test"; account_id = "mock-account" } | ConvertTo-Json
$created = Invoke-RestMethod -Uri "$apiBase/integrations" -Method POST -Headers $headers -ContentType "application/json" -Body $body
$integrationId = $created.id
Check ([bool]$integrationId -and $created.sync_mode -eq "mock") "Test-mode integration created ($integrationId)"
if (-not $integrationId) { Write-Host "Cannot continue without the test integration." -ForegroundColor Red; exit 1 }

# 4. "Reboot": stop every Calyx container, then start them again while the stand-in holds the
#    ports. No recreate: the same containers must come back as they are.
Write-Host "Stopping and starting the Calyx stack while the stand-in holds the ports..." -ForegroundColor Yellow
$createdBefore = @{}
foreach ($svc in $services) { $createdBefore[$svc] = (docker inspect -f '{{.Created}}' "deploy-$svc-1" | Out-String).Trim() }
$restartedAt = (Get-Date).ToUniversalTime()
docker compose --env-file .env -f deploy/docker-compose.yml stop *> $null
Check ($LASTEXITCODE -eq 0) "Calyx stopped"
$sState = (docker inspect -f '{{.State.Status}}' $standin | Out-String).Trim()
Check ($sState -eq "running") "stand-in still running while Calyx is down"
docker compose --env-file .env -f deploy/docker-compose.yml start *> $null
Check ($LASTEXITCODE -eq 0) "Calyx started again"
Check (Wait-Engine 60) "Docker engine is up"
Check (Wait-Stack 420) "all 9 services running after the restart (no Restarting)"
$recreated = @($services | Where-Object { (docker inspect -f '{{.Created}}' "deploy-$_-1" | Out-String).Trim() -ne $createdBefore[$_] })
Check ($recreated.Count -eq 0) "same containers came back, none recreated ($($recreated -join ', '))"

# 5. Checks.
foreach ($svc in $services) {
  $st = Get-State $svc
  Check ($st -and $st.Policy -eq "unless-stopped") "deploy-$svc-1 restart policy unless-stopped (got $($st.Policy))"
}
$net = (docker inspect deploy-redis-1 --format '{{range $n,$v := .NetworkSettings.Networks}}{{$n}}={{$v.Aliases}};{{end}}' | Out-String).Trim()
Check ($net -match 'deploy_default=\[[^\]]*\bredis\b') "redis on deploy_default with alias redis ($net)"
$redisPorts = docker port deploy-redis-1
Check ([string]::IsNullOrWhiteSpace(($redisPorts | Out-String))) "redis has no host port"
$sState = docker inspect -f '{{.State.Status}}' $standin
Check ($sState.Trim() -eq "running") "stand-in still holds 6379 after the restart"

$published = @()
foreach ($svc in $services) { $published += (docker port "deploy-$svc-1") }
$hostPorts = $published | Where-Object { $_ } | ForEach-Object { ($_ -split ':')[-1] } | Sort-Object -Unique
Check ((($hostPorts -join ',') -eq '3100,3101')) "Calyx publishes only 3100 and 3101 (got: $($hostPorts -join ', '))"

$since = $restartedAt.ToString("yyyy-MM-ddTHH:mm:ssZ")
$deadline = (Get-Date).AddSeconds($SyncWaitSeconds)
$synced = $false
while ((Get-Date) -lt $deadline) {
  $logs = docker logs --since $since deploy-platform-sync-1 2>&1 | Out-String
  if ($logs -match '"api sync completed"' -and $logs -match $integrationId) { $synced = $true; break }
  Start-Sleep -Seconds 10
}
$logs = docker logs --since $since deploy-platform-sync-1 2>&1 | Out-String
Check $synced "platform-sync completed a sync of the test integration after the restart"
Check (-not ($logs -match 'lookup redis')) "no 'lookup redis' in platform-sync since the restart"

$ready = docker exec deploy-ingestion-1 wget -qO- http://127.0.0.1:8080/readyz 2>$null
Check (($ready | Out-String).Trim() -eq "ok") "ingestion /readyz ok"
$row = docker exec deploy-postgres-1 psql -U callloom -d callloom -tAc "SELECT sync_status || '|' || COALESCE(to_char(last_success_at AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS'),'') FROM integrations WHERE id = '$integrationId'::uuid"
Check (($row | Out-String).Trim() -match '^ok\|\d{4}-') "test integration is ok with a last_success_at ($(($row | Out-String).Trim()))"

# 6. Clean up: stand-in removed, test integration paused (not deleted, like verify-api-sync).
docker rm -f $standin *> $null
docker exec deploy-postgres-1 psql -U callloom -d callloom -c "UPDATE integrations SET is_active = FALSE, updated_at = NOW() WHERE id = '$integrationId'::uuid AND name = '$testName';" *> $null
Write-Host "Stand-in removed; test integration paused. The stack now runs in the server layout;"
Write-Host "run scripts/dev-up.ps1 to re-publish the laptop gate ports."

Write-Host ""
Write-Host "Reboot resilience: $script:pass passed, $script:fail failed"
if ($script:fail -gt 0) { Write-Host "REBOOT RESILIENCE: FAIL" -ForegroundColor Red; exit 1 }
Write-Host "REBOOT RESILIENCE: PASS" -ForegroundColor Green
exit 0
