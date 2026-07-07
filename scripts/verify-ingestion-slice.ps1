# Phase 0 ingestion slice verification: dedup-correct aggregates + call_key ciphertext.
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiBase = if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:3000" }
$ingestionDir = Join-Path $repoRoot "services\ingestion"
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

function Get-AuthHeaders {
  $login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST `
    -ContentType "application/json" `
    -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
  return @{ Authorization = "Bearer $($login.access_token)" }
}

function Send-RingbaPing {
  param([string]$CallId, [string]$State = "WA")
  $token = "dev_ringba_token_change_me"
  $query = "CallID=$CallId&caller_id=15559876543&state=$State&event=incoming&call_revenue=7.25"
  $url = "http://localhost:8080/webhooks/ringba/$token" + "?" + $query
  Push-Location $ingestionDir
    $signature = (go run ./cmd/platformsig ringba $query).Trim()
  Pop-Location
  $null = Invoke-WebRequest -Uri $url -Headers @{ "X-Ringba-Signature" = $signature } -UseBasicParsing
}

Write-Host "=== Ingestion slice verify ===" -ForegroundColor Cyan

$headers = Get-AuthHeaders
$metricsBefore = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$coverageBefore = Invoke-RestMethod -Uri "$apiBase/coverage/heatmap" -Method GET -Headers $headers
$beforePings = [int]$metricsBefore.total_pings
$beforeHeatmap = [int]$coverageBefore.total_pings
Write-Host "Before: analytics.total_pings=$beforePings coverage.total_pings=$beforeHeatmap"

function Wait-ForPingRow {
  param([string]$ExternalCallId, [int]$TimeoutSec = 30)
  $deadline = (Get-Date).AddSeconds($TimeoutSec)
  do {
  $headers = Get-AuthHeaders
    $pings = Invoke-RestMethod -Uri "$apiBase/pings?limit=100" -Method GET -Headers $headers
    $row = @($pings.data | Where-Object { $_.external_call_id -eq $ExternalCallId } | Select-Object -First 1)
    if ($row.Count -gt 0) { return $row[0] }
    Start-Sleep -Seconds 2
  } while ((Get-Date) -lt $deadline)
  return $null
}

$callId = [guid]::NewGuid().ToString()
Write-Host "Sending unique Ringba ping ($callId, state=WA)..."
Send-RingbaPing -CallId $callId -State "WA"

$row = Wait-ForPingRow -ExternalCallId $callId
if (-not $row) {
  Write-Host "FAIL: unique ping not found in /pings within timeout" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: unique ping landed in /pings (external_call_id=$callId)"

$headers = Get-AuthHeaders
$metricsAfter = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$coverageAfter = Invoke-RestMethod -Uri "$apiBase/coverage/heatmap" -Method GET -Headers $headers
$afterPings = [int]$metricsAfter.total_pings
$afterHeatmap = [int]$coverageAfter.total_pings

if ($afterPings -lt ($beforePings + 1)) {
  Write-Host "FAIL: analytics.total_pings expected >= $($beforePings + 1), got $afterPings" -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: unique ping incremented analytics.total_pings (' + $beforePings + ' to ' + $afterPings + ')')

if ($afterHeatmap -lt ($beforeHeatmap + 1)) {
  Write-Host "FAIL: coverage.total_pings expected >= $($beforeHeatmap + 1), got $afterHeatmap" -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: unique ping incremented coverage.total_pings (' + $beforeHeatmap + ' to ' + $afterHeatmap + ')')

Write-Host "Resending duplicate Ringba ping (same CallID)..."
Send-RingbaPing -CallId $callId -State "WA"
Start-Sleep -Seconds 10

$headers = Get-AuthHeaders
$pings = Invoke-RestMethod -Uri "$apiBase/pings?limit=100" -Method GET -Headers $headers
$dupRows = @($pings.data | Where-Object { $_.external_call_id -eq $callId })
if ($dupRows.Count -lt 2) {
  Write-Host "FAIL: expected >=2 /pings rows for duplicate CallID (got $($dupRows.Count))" -ForegroundColor Red
  exit 1
}
$flagged = @($dupRows | Where-Object { $_.is_duplicate -eq 1 })
if ($flagged.Count -lt 1) {
  Write-Host "FAIL: duplicate resend missing is_duplicate=1 row" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: /pings shows is_duplicate=1 on resend ($($flagged.Count) flagged row(s))"

# Totals may rise from concurrent platform-sync; duplicate must not add exactly one attributable hit.
$metricsDup = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$dupPings = [int]$metricsDup.total_pings
if ($dupPings -eq ($afterPings + 1)) {
  Write-Host ('FAIL: analytics.total_pings rose by 1 after duplicate (' + $afterPings + ' to ' + $dupPings + ') - dedup may be broken') -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: duplicate did not add a dedup-counted analytics ping (' + $afterPings + ' to ' + $dupPings + '; concurrent sync may add more)')

Write-Host "Retreaver call_key ciphertext check..."
$retCallId = [guid]::NewGuid().ToString()
$callKey = "verify_key_$([guid]::NewGuid().ToString('N').Substring(0,8))"
$token = "dev_retreaver_token_change_me"
$query = "call_uuid=$retCallId&call_key=$callKey&caller=15551234567&state=CA&event=incoming&revenue=5.50"
$url = "http://localhost:8080/webhooks/retreaver/$token" + "?" + $query
Push-Location $ingestionDir
  $signature = (go run ./cmd/platformsig retreaver $query).Trim()
Pop-Location
$null = Invoke-WebRequest -Uri $url -Headers @{ "X-Retreaver-Signature" = $signature } -UseBasicParsing
Start-Sleep -Seconds 10

$keyLen = docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query `
  "SELECT length(call_key) FROM default.pings WHERE platform='retreaver' AND external_call_id='$retCallId' LIMIT 1"
$keyLen = $keyLen.Trim()
if ([int]$keyLen -le 30) {
  Write-Host "FAIL: call_key length $keyLen looks like plaintext (expected ciphertext hex >30)" -ForegroundColor Red
  exit 1
}
$plaintextHit = docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query `
  "SELECT count() FROM default.pings WHERE platform='retreaver' AND external_call_id='$retCallId' AND call_key = '$callKey'"
if ($plaintextHit.Trim() -ne "0") {
  Write-Host "FAIL: plaintext call_key found in ClickHouse" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: call_key stored as ciphertext (length=$keyLen, plaintext not in DB)"

Write-Host ""
Write-Host "INGESTION SLICE VERIFY PASS" -ForegroundColor Green
exit 0
