# Shared helper for per-platform API sync real verification.
param(
  [Parameter(Mandatory = $true)][string]$Platform,
  [Parameter(Mandatory = $true)][hashtable]$Credentials,
  [string]$ApiBase = $(if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:3000" })
)

$ErrorActionPreference = "Stop"

function Get-AuthHeaders {
  $login = Invoke-RestMethod -Uri "$ApiBase/auth/login" -Method POST `
    -ContentType "application/json" `
    -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
  return @{ Authorization = "Bearer $($login.access_token)" }
}

Write-Host "=== API Sync verify ($Platform) ===" -ForegroundColor Cyan

$health = Invoke-WebRequest -Uri "http://localhost:8088/healthz" -UseBasicParsing -ErrorAction SilentlyContinue
if (-not $health -or $health.StatusCode -ne 200) {
  Write-Host "FAIL: platform-sync not healthy on :8088" -ForegroundColor Red
  exit 1
}

$headers = Get-AuthHeaders
$metricsBefore = Invoke-RestMethod -Uri "$ApiBase/analytics/metrics" -Method GET -Headers $headers
$beforePings = [int]$metricsBefore.total_pings

$name = "apisync-$Platform-$(Get-Date -Format 'HHmmss')"
$bodyHash = @{
  platform = $Platform
  name = $name
  connection_mode = "api_sync"
}
foreach ($key in $Credentials.Keys) {
  $bodyHash[$key] = $Credentials[$key]
}
$body = $bodyHash | ConvertTo-Json

$created = Invoke-RestMethod -Uri "$ApiBase/integrations" -Method POST `
  -Headers $headers -ContentType "application/json" -Body $body
Write-Host "Created integration $($created.id)"

$test = Invoke-RestMethod -Uri "$ApiBase/integrations/$($created.id)/test" -Method POST -Headers $headers
if (-not $test.ok) {
  Write-Host "FAIL: test - $($test.message)" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: test - $($test.message)"

$null = Invoke-RestMethod -Uri "$ApiBase/integrations/$($created.id)/sync" -Method POST -Headers $headers
Start-Sleep -Seconds 15

$headers = Get-AuthHeaders
$metricsAfter = Invoke-RestMethod -Uri "$ApiBase/analytics/metrics" -Method GET -Headers $headers
$afterPings = [int]$metricsAfter.total_pings

if ($afterPings -le $beforePings) {
  Write-Host "FAIL: total_pings did not increase ($beforePings -> $afterPings)" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: total_pings ($beforePings -> $afterPings)" -ForegroundColor Green
Write-Host "=== API Sync verify ($Platform) complete ===" -ForegroundColor Green
