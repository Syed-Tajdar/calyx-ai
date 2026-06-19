# Demo seed: send multiple signed pings for a rich management dashboard.
# Requires: docker compose stack running (make dev-up).
param(
  [int]$Count = 25,
  [int]$WaitSeconds = 12
)

$ErrorActionPreference = "Stop"
$token = "dev_ringba_token_change_me"
$states = @("TX", "CA", "FL", "NY", "AZ", "GA", "OH", "NC", "IL", "PA")
$platforms = @("ringba", "ringba", "ringba", "mock")
$revenues = @(8.50, 12.00, 15.75, 22.00, 9.25, 18.50)

$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location "$repoRoot\services\ingestion"

Write-Host "Seeding $Count demo pings..."
$callIds = @()

for ($i = 0; $i -lt $Count; $i++) {
  $callId = [guid]::NewGuid().ToString()
  $callIds += $callId
  $state = $states[$i % $states.Length]
  $revenue = $revenues[$i % $revenues.Length]
  $query = "CallID=$callId&caller_id=1555$($i.ToString('0000000'))&state=$state&event=incoming&call_revenue=$revenue"
  $url = "http://localhost:8080/webhooks/ringba/$token" + "?" + $query
  $signature = (go run ./cmd/hmacsig $query).Trim()
  try {
    $null = Invoke-WebRequest -Uri $url -Headers @{ "X-Ringba-Signature" = $signature } -UseBasicParsing
    Write-Host "  [$($i + 1)/$Count] $state ping sent ($callId)"
  } catch {
    Write-Host "  FAILED ping $($i + 1): $_"
    Pop-Location
    exit 1
  }
  Start-Sleep -Milliseconds 200
}

Pop-Location

Write-Host "Waiting ${WaitSeconds}s for pipeline (consumer, scoring, compliance, attribution)..."
Start-Sleep -Seconds $WaitSeconds

$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" `
  -Body '{"email":"admin@callloom.dev","password":"callloom_dev"}'

$pings = Invoke-RestMethod -Uri "http://localhost:3000/pings?limit=5" -Method GET `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }

Write-Host "API total pings: $($pings.total)"
if ($pings.total -lt 1) {
  Write-Host "DEMO SEED FAIL - no pings in API"
  exit 1
}

$metrics = Invoke-RestMethod -Uri "http://localhost:3000/analytics/metrics" -Method GET `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }

Write-Host "Revenue: `$$($metrics.total_revenue) | Pings: $($metrics.total_pings)"
Write-Host "DEMO SEED PASS"
Write-Host "Sample call ID for deep view: $($callIds[0])"
