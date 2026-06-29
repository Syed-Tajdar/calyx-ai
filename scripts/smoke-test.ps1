# Smoke test: send a signed Ringba pixel to ingestion and verify API returns it.
$token = "dev_ringba_token_change_me"
$callId = [guid]::NewGuid().ToString()
$query = "CallID=$callId&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50"
$url = "http://localhost:8080/webhooks/ringba/$token" + "?" + $query

$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location "$repoRoot\services\ingestion"
$signature = (go run ./cmd/hmacsig $query).Trim()
Pop-Location

Write-Host "Sending signed test ping (CallID=$callId)"
$r = Invoke-WebRequest -Uri $url -Headers @{ "X-Ringba-Signature" = $signature } -UseBasicParsing
Write-Host "Ingestion response: $($r.StatusCode)"

Start-Sleep -Seconds 8

$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" `
  -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'

$pings = Invoke-RestMethod -Uri "http://localhost:3000/pings?limit=5" -Method GET `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }

Write-Host "Pings returned: $($pings.total)"
$found = $pings.data | Where-Object { $_.external_call_id -eq $callId }
if ($found) {
  Write-Host "E2E PASS"
} else {
  Write-Host "E2E FAIL - new ping not found"
  exit 1
}
$pings.data | Select-Object -First 3 | Format-Table platform, external_call_id, state, revenue, event_type
