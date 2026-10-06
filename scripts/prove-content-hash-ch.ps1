# Content-hash ClickHouse row-level proof (Retreaver Test + MOCK_DEDUP_PROOF=1).
$ErrorActionPreference = "Stop"
$apiBase = if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:3100" }

function Get-AuthHeaders {
  $login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST `
    -ContentType "application/json" `
    -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
  return @{ Authorization = "Bearer $($login.access_token)" }
}

function Get-ProofCounts {
  $q = "SELECT count() AS rows, uniqExact(external_call_id) AS calls FROM default.pings WHERE external_call_id LIKE 'smoke-proof-retreaver-%'"
  $out = docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query $q
  $parts = ($out.Trim() -split '\s+')
  return @{ rows = [int]$parts[0]; calls = [int]$parts[1] }
}

Write-Host "=== Content-hash CH row-level proof ===" -ForegroundColor Cyan

$hz = Invoke-WebRequest -Uri "http://localhost:8088/healthz" -UseBasicParsing
if ($hz.StatusCode -ne 200) { throw "platform-sync unhealthy" }

# Clear prior proof hashes so Sync A is PublishNew
docker exec deploy-redis-1 redis-cli --scan --pattern "*:dedup:retreaver:smoke-proof-retreaver-*" | ForEach-Object {
  if ($_) { docker exec deploy-redis-1 redis-cli DEL $_ | Out-Null }
}

$headers = Get-AuthHeaders
$name = "dedup-proof-$(Get-Date -Format 'HHmmss')"
$body = @{
  platform = "retreaver"
  name = $name
  connection_mode = "api_sync"
  api_key = "mock-retreaver-key-for-dedup-proof"
  company_id = "mock-company-id"
} | ConvertTo-Json

$created = Invoke-RestMethod -Uri "$apiBase/integrations" -Method POST `
  -Headers $headers -ContentType "application/json" -Body $body
Write-Host "Created $($created.id)"

$before = Get-ProofCounts
Write-Host "BASELINE before Sync A: rows=$($before.rows) calls=$($before.calls)"

$null = Invoke-RestMethod -Uri "$apiBase/integrations/$($created.id)/sync" -Method POST -Headers $headers
Start-Sleep -Seconds 15
$afterA = Get-ProofCounts
Write-Host "AFTER Sync A: rows=$($afterA.rows) calls=$($afterA.calls)"

$null = Invoke-RestMethod -Uri "$apiBase/integrations/$($created.id)/sync" -Method POST -Headers $headers
Start-Sleep -Seconds 15
$afterB = Get-ProofCounts
Write-Host "AFTER Sync B (identical): rows=$($afterB.rows) calls=$($afterB.calls)"

if ($afterB.rows -ne $afterA.rows) {
  Write-Host "FAIL: Sync B grew rows ($($afterA.rows) -> $($afterB.rows))" -ForegroundColor Red
  exit 1
}
if ($afterB.calls -ne $afterA.calls) {
  Write-Host "FAIL: Sync B changed uniq calls" -ForegroundColor Red
  exit 1
}
Write-Host "PASS: Sync B added ZERO rows (content-hash)" -ForegroundColor Green

# page1_refresh log
$refresh = docker logs deploy-platform-sync-1 2>&1 | Select-String "page1_refresh" | Select-Object -Last 3
Write-Host "page1_refresh log lines:"
$refresh | ForEach-Object { Write-Host $_ }

Write-Host "=== CH proof complete (Sync A/B). For Sync C bump revenue via MOCK_DEDUP_PROOF_REVENUE and re-sync. ==="
exit 0
