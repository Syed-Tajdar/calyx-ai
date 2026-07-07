# API Sync verification: create api_sync integration, trigger sync, confirm pings increase.
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiBase = if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:3000" }

function Get-AuthHeaders {
  $login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST `
    -ContentType "application/json" `
    -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
  return @{ Authorization = "Bearer $($login.access_token)" }
}

Write-Host "=== API Sync verify ===" -ForegroundColor Cyan

$health = Invoke-WebRequest -Uri "http://localhost:8088/healthz" -UseBasicParsing -ErrorAction SilentlyContinue
if (-not $health -or $health.StatusCode -ne 200) {
  Write-Host "FAIL: platform-sync not healthy on :8088 - run docker compose up platform-sync" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: platform-sync healthz"

$headers = Get-AuthHeaders
$metricsBefore = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$beforePings = [int]$metricsBefore.total_pings
Write-Host "Before: analytics.total_pings=$beforePings"

$name = "api-sync-verify-$(Get-Date -Format 'HHmmss')"
$body = @{
  platform = "retreaver"
  name = $name
  connection_mode = "api_sync"
  api_key = "mock-retreaver-key-for-verify"
  company_id = "mock-company-id"
} | ConvertTo-Json

$created = Invoke-RestMethod -Uri "$apiBase/integrations" -Method POST `
  -Headers $headers -ContentType "application/json" -Body $body
Write-Host "Created integration $($created.id) mode=$($created.connection_mode)"

$test = Invoke-RestMethod -Uri "$apiBase/integrations/$($created.id)/test" -Method POST -Headers $headers
if (-not $test.ok) {
  Write-Host "FAIL: test connection: $($test.message)" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: test connection - $($test.message)"

$sync = Invoke-RestMethod -Uri "$apiBase/integrations/$($created.id)/sync" -Method POST -Headers $headers
Write-Host "Sync triggered: $($sync.status)"
Start-Sleep -Seconds 12

$headers = Get-AuthHeaders
$metricsAfter = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$afterPings = [int]$metricsAfter.total_pings

if ($afterPings -le $beforePings) {
  Write-Host "FAIL: expected total_pings to increase ($beforePings -> $afterPings)" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: analytics.total_pings increased ($beforePings -> $afterPings)" -ForegroundColor Green

$pings = Invoke-RestMethod -Uri "$apiBase/pings?limit=20" -Method GET -Headers $headers
$apisyncPing = $pings.data | Where-Object { $_.external_call_id -like "apisync-*" } | Select-Object -First 1
if (-not $apisyncPing) {
  Write-Host "WARN: no apisync-* ping in recent list (consumer lag?) - metrics still passed"
} else {
  Write-Host "PASS: found apisync ping $($apisyncPing.external_call_id) platform=$($apisyncPing.platform)"
}

Write-Host "=== API Sync verify complete ===" -ForegroundColor Green
