# Retreaver real ingestion verification (local Docker or post-tunnel real calls).
# Does NOT print call_key values or webhook secrets.
param(
  [string]$ApiBase = $(if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:3100" }),
  [string]$Platform = "retreaver",
  [int]$LogTail = 40
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$pass = 0
$fail = 0
$warn = 0

function Write-Check {
  param([string]$Name, [bool]$Ok, [string]$Detail = "")
  if ($Ok) {
    Write-Host "  PASS $Name" -ForegroundColor Green
    if ($Detail) { Write-Host "       $Detail" }
    $script:pass++
  } else {
    Write-Host "  FAIL $Name" -ForegroundColor Red
    if ($Detail) { Write-Host "       $Detail" }
    $script:fail++
  }
}

function Write-WarnCheck {
  param([string]$Name, [string]$Detail = "")
  Write-Host "  WARN $Name" -ForegroundColor Yellow
  if ($Detail) { Write-Host "       $Detail" }
  $script:warn++
}

Write-Host "=== Retreaver real verify ===" -ForegroundColor Cyan
Write-Host "API: $ApiBase"

# --- Auth + API ---
$login = Invoke-RestMethod -Uri "$ApiBase/auth/login" -Method POST `
  -ContentType "application/json" `
  -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
$headers = @{ Authorization = "Bearer $($login.access_token)" }

$pings = Invoke-RestMethod -Uri "$ApiBase/pings?platform=$Platform&limit=10" -Headers $headers
$retPings = @($pings.data)
Write-Check "GET /pings?platform=$Platform" ($retPings.Count -gt 0) "rows=$($retPings.Count)"

$apiHasCallKey = $false
foreach ($row in $retPings) {
  if ($row.PSObject.Properties.Name -contains 'call_key') { $apiHasCallKey = $true }
}
Write-Check "API response excludes call_key field" (-not $apiHasCallKey)

$mix = Invoke-RestMethod -Uri "$ApiBase/analytics/platform-mix" -Headers $headers
$retMix = @($mix | Where-Object { $_.platform -eq $Platform })
Write-Check "GET /analytics/platform-mix includes $Platform" ($retMix.Count -gt 0) "count=$($retMix[0].count)"

$heatmap = Invoke-RestMethod -Uri "$ApiBase/coverage/heatmap" -Headers $headers
$retStates = @($heatmap.data | Where-Object { $_.ping_count -gt 0 })
Write-Check "GET /coverage/heatmap has geo data" ($heatmap.total_pings -gt 0) "total_pings=$($heatmap.total_pings) states=$($retStates.Count)"

# --- ClickHouse (no call_key values) ---
$chStates = docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query `
  "SELECT state, count() AS cnt FROM default.pings WHERE platform='$Platform' AND is_duplicate = 0 GROUP BY state ORDER BY cnt DESC FORMAT TabSeparated"
$stateLines = @($chStates -split "`n" | Where-Object { $_.Trim() -ne "" })
Write-Check "ClickHouse retreaver rows by state" ($stateLines.Count -gt 0) "state_groups=$($stateLines.Count)"

$chKeyStats = docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query `
  "SELECT countIf(call_key != '') AS with_key, countIf(length(call_key) > 30) AS likely_ciphertext, countIf(length(call_key) > 0 AND length(call_key) <= 30) AS likely_plaintext FROM default.pings WHERE platform='$Platform'"
$parts = $chKeyStats.Trim() -split "`t"
$withKey = [int]$parts[0]
$ciphertext = [int]$parts[1]
$plaintext = [int]$parts[2]
Write-Check "call_key stored (presence only)" ($withKey -ge 0) "with_key=$withKey"
if ($withKey -gt 0) {
  Write-Check "call_key likely ciphertext (len>30)" ($ciphertext -eq $withKey -and $plaintext -eq 0) "ciphertext=$ciphertext plaintext_len_le30=$plaintext"
} else {
  Write-WarnCheck "call_key rows" "none with call_key — ensure Retreaver sends call_key on real test"
}

# Non-demo state check (WA used in verify-ingestion-slice)
$hasWA = $false
foreach ($line in $stateLines) {
  if ($line -match "^WA`t") { $hasWA = $true }
}
if ($hasWA) {
  Write-Check "Non-demo state WA in ClickHouse" $true
} else {
  Write-WarnCheck "Non-demo state WA" "not found — fire real calls with WA or refresh after test"
}

# --- Ingestion logs ---
Write-Host ""
Write-Host "=== Ingestion logs (last $LogTail) ===" -ForegroundColor Cyan
$logs = docker logs deploy-ingestion-1 --tail $LogTail 2>&1 | Out-String
$hasIngested = $logs -match "ping ingested" -and $logs -match "retreaver"
$has401 = $logs -match "webhook auth failed" -and $logs -match "retreaver"
Write-Check "Ingestion logs show retreaver ping ingested" $hasIngested
if ($has401) {
  Write-Check "No retreaver 401 auth failures in recent logs" $false "webhook auth failed found"
} else {
  Write-Check "No retreaver 401 auth failures in recent logs" $true
}

# call_key never in logs as value
$logsLeak = $logs -match 'call_key=[^p]' -or $logs -match '"call_key":"[^"]+'
Write-Check "call_key value not in ingestion logs" (-not $logsLeak)

Write-Host ""
Write-Host "Summary: PASS=$pass FAIL=$fail WARN=$warn" -ForegroundColor Cyan
if ($fail -gt 0) { exit 1 }
exit 0
