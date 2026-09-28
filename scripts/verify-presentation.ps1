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
  "http://localhost:3100/healthz"
)
foreach ($u in $healthUrls) {
  if (-not (Test-Endpoint "health $u" $u @{})) { exit 1 }
}

$login = Invoke-RestMethod -Uri "http://localhost:3100/auth/login" -Method POST `
  -ContentType "application/json" `
  -Headers @{ "X-Calyx-Client" = "script" } `
  -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'

$h = @{ Authorization = "Bearer $($login.access_token)" }
$base = "http://localhost:3100"
$passed = 0
$total = 0

$endpoints = @(
  @{ Name = "GET /auth/me"; Url = "$base/auth/me" },
  @{ Name = "GET /pings"; Url = "$base/pings?limit=5" },
  @{ Name = "GET /scorecards"; Url = "$base/scorecards?limit=5" },
  @{ Name = "GET /analytics/metrics"; Url = "$base/analytics/metrics" },
  @{ Name = "GET /analytics/pnl"; Url = "$base/analytics/pnl" },
  @{ Name = "GET /analytics/platform-mix"; Url = "$base/analytics/platform-mix" },
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

# Tenant name check
$total++
try {
  $me = Invoke-RestMethod -Uri "$base/auth/me" -Headers $h
  if ($me.tenant_name -eq "Demo Tenant") {
    Write-Host "  PASS GET /auth/me tenant_name = Demo Tenant"
    $passed++
  } else {
    Write-Host "  FAIL GET /auth/me tenant_name = '$($me.tenant_name)' (expected Demo Tenant)"
  }
} catch {
  Write-Host "  FAIL GET /auth/me tenant_name check ($($_.Exception.Message))"
}

# Scorecard CRUD smoke
$total++
$scorecardName = "Verify Scorecard $([guid]::NewGuid().ToString('N').Substring(0, 8))"
$scorecardBody = (@{
  name     = $scorecardName
  vertical = 'general'
  rules    = @(@{ id = 'r1'; weight = 1; question = 'Test?' })
} | ConvertTo-Json -Compress)
try {
  $created = Invoke-RestMethod -Uri "$base/scorecards" -Method POST -Headers $h -ContentType "application/json" -Body $scorecardBody
  $scId = $created.id
  Invoke-RestMethod -Uri "$base/scorecards/$scId" -Method PATCH -Headers $h -ContentType "application/json" -Body '{"name":"Verify Updated"}' | Out-Null
  Invoke-RestMethod -Uri "$base/scorecards/$scId" -Method DELETE -Headers $h | Out-Null
  Write-Host "  PASS scorecard CRUD smoke"
  $passed++
} catch {
  Write-Host "  FAIL scorecard CRUD smoke ($($_.Exception.Message))"
}

# Fixture data check
$total++
try {
  $pingCheck = Invoke-RestMethod -Uri "$base/pings?limit=1" -Headers $h
  if ([int]$pingCheck.total -gt 0) {
    Write-Host "  PASS fixture pings total > 0 ($($pingCheck.total))"
    $passed++
  } else {
    Write-Host "  FAIL fixture pings total = 0 (run clickhouse-init / demo-seed)"
  }
} catch {
  Write-Host "  FAIL fixture pings check ($($_.Exception.Message))"
}

# Demo call deep view fixture
$fixtureCallId = "demo-call-1"
$total++
try {
  $deep = Invoke-RestMethod -Uri "$base/native/call-deep-view?callId=$fixtureCallId" -Headers $h
  if ($deep.ping) {
    Write-Host "  PASS GET /native/call-deep-view?callId=$fixtureCallId (ping present)"
    $passed++
  } else {
    Write-Host "  FAIL GET /native/call-deep-view?callId=$fixtureCallId (ping missing - run clickhouse-init)"
  }
} catch {
  Write-Host "  FAIL GET /native/call-deep-view?callId=$fixtureCallId ($($_.Exception.Message))"
}

$total++
try {
  $score = Invoke-RestMethod -Uri "$base/scoring/scores?callId=$fixtureCallId" -Headers $h
  if ($score -and $score.call_id -eq $fixtureCallId) {
    Write-Host "  PASS GET /scoring/scores?callId=$fixtureCallId"
    $passed++
  } else {
    Write-Host "  FAIL GET /scoring/scores?callId=$fixtureCallId (no score row)"
  }
} catch {
  Write-Host "  FAIL GET /scoring/scores?callId=$fixtureCallId ($($_.Exception.Message))"
}

$total++
try {
  $transcript = Invoke-RestMethod -Uri "$base/scoring/transcript?callId=$fixtureCallId" -Headers $h
  if ($transcript -and $transcript.transcript_text) {
    Write-Host "  PASS GET /scoring/transcript?callId=$fixtureCallId"
    $passed++
  } else {
    Write-Host "  FAIL GET /scoring/transcript?callId=$fixtureCallId (no transcript)"
  }
} catch {
  Write-Host "  FAIL GET /scoring/transcript?callId=$fixtureCallId ($($_.Exception.Message))"
}

Write-Host ""
Write-Host "API checks: $passed / $total passed"
if ($passed -lt $total) { exit 1 }
Write-Host "PRESENTATION VERIFY PASS"
