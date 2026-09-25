# Full system verification: Docker health, all microservices, API, web routes, WebSocket.
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$deployDir = Join-Path $repoRoot "deploy"
if ($env:WEB_BASE_URL) { $webBase = $env:WEB_BASE_URL } else { $webBase = "http://localhost:3001" }
if ($env:API_BASE_URL) { $apiBase = $env:API_BASE_URL } else { $apiBase = "http://localhost:3000" }
$fixtureCallId = "demo-call-1"

$summary = @{
  DockerContainers = @{ Pass = 0; Total = 0 }
  HealthEndpoints  = @{ Pass = 0; Total = 0 }
  WebRoutes        = @{ Pass = 0; Total = 0 }
  ApiPresentation  = $null
  WebSocket        = $null
  BrowserCsp       = $null
  UnitTests        = $null
  PipelineSmoke    = $null
  WebBuild         = $null
}

function Write-Section([string]$Title) {
  Write-Host ""
  Write-Host "=== $Title ==="
}

function Test-HttpStatus {
  param([string]$Name, [string]$Url, [int[]]$Allowed = @(200))
  try {
    $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 15 -MaximumRedirection 0 -ErrorAction Stop
    $ok = $Allowed -contains $r.StatusCode
    if ($ok) { Write-Host "  PASS $Name ($($r.StatusCode))" } else { Write-Host "  FAIL $Name (status $($r.StatusCode), expected $($Allowed -join '/'))" }
    return $ok
  } catch {
    if ($_.Exception.Response) {
      $code = [int]$_.Exception.Response.StatusCode
      $ok = $Allowed -contains $code
      if ($ok) { Write-Host "  PASS $Name ($code)" } else { Write-Host "  FAIL $Name ($code)" }
      return $ok
    }
    Write-Host "  FAIL $Name ($($_.Exception.Message))"
    return $false
  }
}

Write-Section "Docker containers"
Push-Location $deployDir
try {
  $ps = docker compose ps --format json 2>&1 | Where-Object { $_ -match '^\{' }
  $rows = @($ps | ForEach-Object { $_ | ConvertFrom-Json })
  $appServices = @(
    "ingestion", "consumer", "scoring", "enrichment", "providerconfig",
    "compliance-intelligence", "ask-ai", "ad-attribution", "api", "web",
    "platform-sync"
  )
  $infraHealthy = @("postgres", "redis", "clickhouse", "redpanda")
  foreach ($name in ($appServices + $infraHealthy)) {
    $summary.DockerContainers.Total++
    $row = $rows | Where-Object { $_.Service -eq $name } | Select-Object -First 1
    if (-not $row) {
      Write-Host "  FAIL $name (not found)"
      continue
    }
    $state = $row.State
    $health = $row.Health
    $ok = $state -eq "running"
    if ($infraHealthy -contains $name) { $ok = $ok -and ($health -eq "healthy") }
    if ($ok) {
      Write-Host "  PASS $name ($state$(if ($health) { ", $health" }))"
      $summary.DockerContainers.Pass++
    } else {
      Write-Host "  FAIL $name ($state$(if ($health) { ", $health" }))"
    }
  }
} finally {
  Pop-Location
}

Write-Section "Service health endpoints"
$healthPorts = @(8080, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 3000, 8088)
foreach ($port in $healthPorts) {
  $summary.HealthEndpoints.Total++
  if (Test-HttpStatus ":$port/healthz" "http://localhost:$port/healthz") {
    $summary.HealthEndpoints.Pass++
  }
}
$summary.HealthEndpoints.Total++
if (Test-HttpStatus ":3000/readyz" "http://localhost:3000/readyz") {
  $summary.HealthEndpoints.Pass++
}

Write-Section "Live feed status"
$summary["LiveStatus"] = "FAIL"
try {
  $live = Invoke-RestMethod -Uri "$apiBase/live-status" -Method GET -TimeoutSec 15
  $member = [bool]$live.is_group_member
  $required = [bool]$live.required
  Write-Host "  group=$($live.group_id) required=$required is_group_member=$member status=$($live.status) recreate_count=$($live.recreate_count)"
  if (-not $required -or $member) {
    Write-Host "  PASS /live-status"
    $summary["LiveStatus"] = "PASS"
  } else {
    Write-Host "  FAIL /live-status (live consumer not a group member)"
  }
} catch {
  Write-Host "  FAIL /live-status ($($_.Exception.Message))"
}

Write-Section "API presentation verification"
$verifyScript = Join-Path $repoRoot "scripts\verify-presentation.ps1"
& powershell -ExecutionPolicy Bypass -File $verifyScript
$apiExit = $LASTEXITCODE
$summary.ApiPresentation = if ($apiExit -eq 0) { "PASS (20/20)" } else { "FAIL" }

Write-Section "Web routes ($webBase)"
$routes = @(
  "/login", "/dashboard", "/pings", "/scoring", "/scoring/scorecards",
  "/intelligence", "/ask", "/compliance", "/attribution", "/coverage",
  "/reports", "/settings"
)
foreach ($route in $routes) {
  $summary.WebRoutes.Total++
  if (Test-HttpStatus $route "$webBase$route") { $summary.WebRoutes.Pass++ }
}

# Deep view — fixture call from ClickHouse demo seed
$summary.WebRoutes.Total++
if (Test-HttpStatus "/calls/$fixtureCallId" "$webBase/calls/$fixtureCallId") { $summary.WebRoutes.Pass++ }

Write-Section "WebSocket live feed"
$wsScript = Join-Path $repoRoot "scripts\ws-smoke-test.mjs"
& node $wsScript
$wsExit = $LASTEXITCODE
$summary.WebSocket = if ($wsExit -eq 0) { "PASS" } else { "FAIL" }

Write-Section "Browser CSP + hydration (Playwright)"
$browserScript = Join-Path $repoRoot "scripts\check-dashboard-browser.mjs"
$env:CALYX_WEB_URL = $webBase
$env:CALYX_API_URL = $apiBase
& node $browserScript
$browserExit = $LASTEXITCODE
$summary.BrowserCsp = if ($browserExit -eq 0) { "PASS" } else { "FAIL" }

Write-Section "Pipeline smoke (multi-platform 4/4)"
$smokeScript = Join-Path $repoRoot "scripts\smoke-test-all-platforms.ps1"
& powershell -ExecutionPolicy Bypass -File $smokeScript
$smokeExit = $LASTEXITCODE
$summary.PipelineSmoke = if ($smokeExit -eq 0) { "PASS (4/4)" } else { "FAIL" }

Write-Section "Unit tests"
$testFailed = $false
$goServices = @(
  "services\ingestion", "services\consumer", "services\scoring", "services\enrichment",
  "services\providerconfig", "services\compliance-intelligence", "services\ask-ai",
  "services\ad-attribution", "services\platform-sync", "packages\pingdedup"
)
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
foreach ($svc in $goServices) {
  Push-Location (Join-Path $repoRoot $svc)
  go test ./... 2>&1 | Out-Host
  if ($LASTEXITCODE -ne 0) { $testFailed = $true }
  Pop-Location
}
Push-Location (Join-Path $repoRoot "services\api")
npm test 2>&1 | Out-Host
$apiTestExit = $LASTEXITCODE
if ($apiTestExit -ne 0) { $testFailed = $true }
Pop-Location
$ErrorActionPreference = $prevEap
$summary.UnitTests = if (-not $testFailed) { "PASS" } else { "FAIL" }

Write-Section "Web production build"
Push-Location (Join-Path $repoRoot "web")
$prevEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
npm run build 2>&1 | Out-Host
$buildExit = $LASTEXITCODE
$ErrorActionPreference = $prevEap
Pop-Location
$summary.WebBuild = if ($buildExit -eq 0) { "PASS" } else { "FAIL" }

Write-Section "SYSTEM VERIFICATION SUMMARY"
Write-Host "  Docker:     $($summary.DockerContainers.Pass) / $($summary.DockerContainers.Total)"
Write-Host "  Healthz:    $($summary.HealthEndpoints.Pass) / $($summary.HealthEndpoints.Total)"
Write-Host "  LiveStatus: $($summary.LiveStatus)"
Write-Host "  API:        $($summary.ApiPresentation)"
Write-Host "  Web routes: $($summary.WebRoutes.Pass) / $($summary.WebRoutes.Total)"
Write-Host "  WebSocket:  $($summary.WebSocket)"
Write-Host "  BrowserCSP: $($summary.BrowserCsp)"
Write-Host "  Unit tests: $($summary.UnitTests)"
Write-Host "  Web build:  $($summary.WebBuild)"
Write-Host "  Pipeline:   $($summary.PipelineSmoke)"

$hardFail = $false
if ($summary.DockerContainers.Pass -lt $summary.DockerContainers.Total) { $hardFail = $true }
if ($summary.HealthEndpoints.Pass -lt $summary.HealthEndpoints.Total) { $hardFail = $true }
if ($summary.LiveStatus -ne "PASS") { $hardFail = $true }
if ($apiExit -ne 0) { $hardFail = $true }
if ($summary.WebRoutes.Pass -lt $summary.WebRoutes.Total) { $hardFail = $true }
if ($wsExit -ne 0) { $hardFail = $true }
if ($browserExit -ne 0) { $hardFail = $true }
if ($testFailed) { $hardFail = $true }
if ($buildExit -ne 0) { $hardFail = $true }
if ($smokeExit -ne 0) { $hardFail = $true }

Write-Host ""
if ($hardFail) {
  Write-Host "SYSTEM VERIFY FAIL (see sections above)"
  exit 1
}
Write-Host "SYSTEM VERIFY PASS"
exit 0
