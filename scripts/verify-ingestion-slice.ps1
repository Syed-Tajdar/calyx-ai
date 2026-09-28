# Phase 0 ingestion slice verification: content-hash contract + call_key ciphertext.
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiBase = if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:3100" }
$ingestionDir = Join-Path $repoRoot "services\ingestion"
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

function Get-AuthHeaders {
  $login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST `
    -ContentType "application/json" `
    -Headers @{ "X-Calyx-Client" = "script" } `
    -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
  if (-not $login.access_token) { throw "login missing access_token (script client header required)" }
  return @{ Authorization = "Bearer $($login.access_token)" }
}

function Send-RingbaPing {
  param(
    [string]$CallId,
    [string]$State = "WA",
    [string]$Revenue = "7.25"
  )
  $token = "dev_ringba_token_change_me"
  $query = "CallID=$CallId&caller_id=15559876543&state=$State&event=incoming&call_revenue=$Revenue"
  $url = "http://localhost:8080/webhooks/ringba/$token" + "?" + $query
  Push-Location $ingestionDir
    $signature = (go run ./cmd/platformsig ringba $query).Trim()
  Pop-Location
  $null = Invoke-WebRequest -Uri $url -Headers @{ "X-Ringba-Signature" = $signature } -UseBasicParsing
}

Write-Host "=== Ingestion slice verify ===" -ForegroundColor Cyan

$headers = Get-AuthHeaders
$metricsDefaultBefore = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$metricsBefore = Invoke-RestMethod -Uri "$apiBase/analytics/metrics?show_demo=true" -Method GET -Headers $headers
$coverageBefore = Invoke-RestMethod -Uri "$apiBase/coverage/heatmap?show_demo=true" -Method GET -Headers $headers
$beforePings = [int]$metricsBefore.total_pings
$beforeDefault = [int]$metricsDefaultBefore.total_pings
$beforeHeatmap = [int]$coverageBefore.total_pings
Write-Host "Before show_demo=true: analytics.total_pings=$beforePings coverage.total_pings=$beforeHeatmap"
Write-Host "Before default view: analytics.total_pings=$beforeDefault"

function Wait-ForPingRow {
  param([string]$ExternalCallId, [int]$TimeoutSec = 30)
  $deadline = (Get-Date).AddSeconds($TimeoutSec)
  do {
    $headers = Get-AuthHeaders
    $pings = Invoke-RestMethod -Uri "$apiBase/pings?limit=100&show_demo=true" -Method GET -Headers $headers
    $row = @($pings.data | Where-Object { $_.external_call_id -eq $ExternalCallId } | Select-Object -First 1)
    if ($row.Count -gt 0) { return $row[0] }
    Start-Sleep -Seconds 2
  } while ((Get-Date) -lt $deadline)
  return $null
}

$callId = "smoke-" + [guid]::NewGuid().ToString()
Write-Host "Sending unique Ringba ping ($callId, state=WA)..."
Send-RingbaPing -CallId $callId -State "WA"

$row = Wait-ForPingRow -ExternalCallId $callId
if (-not $row) {
  Write-Host "FAIL: unique ping not found in /pings within timeout" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: unique ping landed in /pings (external_call_id=$callId)"

$headers = Get-AuthHeaders
$metricsDefaultAfter = Invoke-RestMethod -Uri "$apiBase/analytics/metrics" -Method GET -Headers $headers
$metricsAfter = Invoke-RestMethod -Uri "$apiBase/analytics/metrics?show_demo=true" -Method GET -Headers $headers
$coverageAfter = Invoke-RestMethod -Uri "$apiBase/coverage/heatmap?show_demo=true" -Method GET -Headers $headers
$afterPings = [int]$metricsAfter.total_pings
$afterDefault = [int]$metricsDefaultAfter.total_pings
$afterHeatmap = [int]$coverageAfter.total_pings

Write-Host ("show_demo=true   total " + $beforePings + " -> " + $afterPings)
Write-Host ("default view     total " + $beforeDefault + " -> " + $afterDefault)

if ($afterDefault -ne $beforeDefault) {
  Write-Host "FAIL: default view changed ($beforeDefault -> $afterDefault); smoke row leaked into KPIs" -ForegroundColor Red
  exit 1
}

if ($afterPings -lt ($beforePings + 1)) {
  Write-Host "FAIL: analytics.total_pings expected increase, got $afterPings" -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: unique ping incremented analytics.total_pings (' + $beforePings + ' to ' + $afterPings + ')')

if ($afterHeatmap -lt ($beforeHeatmap + 1)) {
  Write-Host "FAIL: coverage.total_pings expected increase, got $afterHeatmap" -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: unique ping incremented coverage.total_pings (' + $beforeHeatmap + ' to ' + $afterHeatmap + ')')

Write-Host "Resending identical Ringba ping (same CallID + revenue) -- content-hash must suppress..."
$headers = Get-AuthHeaders
$pingsBeforeResend = Invoke-RestMethod -Uri "$apiBase/pings?limit=100&show_demo=true" -Method GET -Headers $headers
$sameBefore = @($pingsBeforeResend.data | Where-Object { $_.external_call_id -eq $callId })
$rowCountBefore = $sameBefore.Count
Send-RingbaPing -CallId $callId -State "WA" -Revenue "7.25"
Start-Sleep -Seconds 10

$headers = Get-AuthHeaders
$pings = Invoke-RestMethod -Uri "$apiBase/pings?limit=100&show_demo=true" -Method GET -Headers $headers
$dupRows = @($pings.data | Where-Object { $_.external_call_id -eq $callId })
if ($dupRows.Count -ne $rowCountBefore) {
  $msg = 'FAIL: identical resend changed row count (' + $rowCountBefore + ' to ' + $dupRows.Count + '); expected unchanged'
  Write-Host $msg -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: identical resend added ZERO new rows (count=' + $dupRows.Count + ')')

Write-Host "Sending changed-revenue Ringba ping (same CallID)..."
Send-RingbaPing -CallId $callId -State "WA" -Revenue "42.50"
Start-Sleep -Seconds 12
$headers = Get-AuthHeaders
$pings2 = Invoke-RestMethod -Uri "$apiBase/pings?limit=100&show_demo=true" -Method GET -Headers $headers
$changedRows = @($pings2.data | Where-Object { $_.external_call_id -eq $callId })
if ($changedRows.Count -lt ($rowCountBefore + 1)) {
  Write-Host ('FAIL: revenue change did not add a new row (had ' + $rowCountBefore + ', got ' + $changedRows.Count + ')') -ForegroundColor Red
  exit 1
}
$newRow = @($changedRows | Where-Object { [double]$_.revenue -ge 42.0 -and $_.is_duplicate -eq 0 } | Select-Object -First 1)
if (-not $newRow) {
  Write-Host "FAIL: missing is_duplicate=0 row with new revenue" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: changed payload published new row with is_duplicate=0 and revenue>=42.5"

$metricsDup = Invoke-RestMethod -Uri "$apiBase/analytics/metrics?show_demo=true" -Method GET -Headers $headers
$dupPings = [int]$metricsDup.total_pings
Write-Host ('PASS: analytics.total_pings after content-hash tests=' + $dupPings)

Write-Host "Retreaver call_key ciphertext check..."
$retCallId = "smoke-" + [guid]::NewGuid().ToString()
$callKey = "verify_key_$([guid]::NewGuid().ToString('N').Substring(0,8))"
$token = "dev_retreaver_token_change_me"
$query = ('call_uuid=' + $retCallId + '&call_key=' + $callKey + '&caller=15551234567&state=CA&event=incoming&revenue=5.50')
$url = "http://localhost:8080/webhooks/retreaver/$token" + "?" + $query
Push-Location $ingestionDir
  $signature = (go run ./cmd/platformsig retreaver $query).Trim()
Pop-Location
$null = Invoke-WebRequest -Uri $url -Headers @{ "X-Retreaver-Signature" = $signature } -UseBasicParsing
Start-Sleep -Seconds 10

$chQuery1 = "SELECT length(call_key) FROM default.pings WHERE platform='retreaver' AND external_call_id='$retCallId' LIMIT 1"
$keyLen = (docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query $chQuery1).Trim()
if ([int]$keyLen -le 30) {
  Write-Host "FAIL: call_key length $keyLen looks like plaintext (expected ciphertext hex >30)" -ForegroundColor Red
  exit 1
}
$chQuery2 = "SELECT count() FROM default.pings WHERE platform='retreaver' AND external_call_id='$retCallId' AND call_key = '$callKey'"
$plaintextHit = (docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query $chQuery2).Trim()
if ($plaintextHit -ne "0") {
  Write-Host "FAIL: plaintext call_key found in ClickHouse" -ForegroundColor Red
  exit 1
}
Write-Host ('PASS: call_key stored as ciphertext (length=' + $keyLen + ', plaintext not in DB)')

Write-Host ""
Write-Host "INGESTION SLICE VERIFY PASS" -ForegroundColor Green
exit 0
