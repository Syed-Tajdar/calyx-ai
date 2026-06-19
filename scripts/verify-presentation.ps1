# Presentation verification: API health + Phase 3 endpoints (MOCK_MODE).
$ErrorActionPreference = "Stop"

function Test-Endpoint {
  param([string]$Name, [string]$Url, [hashtable]$Headers, [string]$Method = "GET", [string]$Body = $null)
  try {
    $params = @{ Uri = $Url; Method = $Method; Headers = $Headers; UseBasicParsing = $true }
    if ($Body) { $params.ContentType = "application/json"; $params.Body = $Body }
    $r = Invoke-WebRequest @params
    if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 300) {
      Write-Host "  PASS $Name"
      return $true
    }
    Write-Host "  FAIL $Name (status $($r.StatusCode))"
    return $false
  } catch {
    Write-Host "  FAIL $Name ($($_.Exception.Message))"
    return $false
  }
}

Write-Host "=== Calyx AI Presentation Verification ==="

$healthUrls = @(
  "http://localhost:8080/healthz",
  "http://localhost:3000/healthz"
)
foreach ($u in $healthUrls) {
  if (-not (Test-Endpoint "health $u" $u @{})) { exit 1 }
}

$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" `
  -Body '{"email":"admin@callloom.dev","password":"callloom_dev"}'

$h = @{ Authorization = "Bearer $($login.access_token)" }
$base = "http://localhost:3000"
$passed = 0
$total = 0

$endpoints = @(
  @{ Name = "GET /auth/me"; Url = "$base/auth/me" },
  @{ Name = "GET /pings"; Url = "$base/pings?limit=5" },
  @{ Name = "GET /analytics/metrics"; Url = "$base/analytics/metrics" },
  @{ Name = "GET /analytics/pnl"; Url = "$base/analytics/pnl" },
  @{ Name = "GET /coverage/heatmap"; Url = "$base/coverage/heatmap" },
  @{ Name = "GET /compliance/exposure"; Url = "$base/compliance/exposure" },
  @{ Name = "GET /compliance/risk-scores"; Url = "$base/compliance/risk-scores?limit=5" },
  @{ Name = "GET /attribution/status"; Url = "$base/attribution/status" },
  @{ Name = "GET /providers/catalog"; Url = "$base/providers/catalog" },
  @{ Name = "GET /providers/bindings"; Url = "$base/providers/bindings" },
  @{ Name = "GET /scoring/scores"; Url = "$base/scoring/scores?limit=5" }
)

foreach ($ep in $endpoints) {
  $total++
  if (Test-Endpoint $ep.Name $ep.Url $h) { $passed++ }
}

$total++
if (Test-Endpoint "POST /ask" "$base/ask" $h "POST" '{"question":"What is my total revenue?"}') { $passed++ }

Write-Host ""
Write-Host "API checks: $passed / $total passed"
if ($passed -lt $total) { exit 1 }
Write-Host "PRESENTATION VERIFY PASS"
