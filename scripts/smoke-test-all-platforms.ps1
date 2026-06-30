# Run E2E smoke tests for all webhook platforms.
$ErrorActionPreference = "Stop"
$platforms = @("ringba", "retreaver", "trackdrive", "callgrid")
$failed = @()

foreach ($platform in $platforms) {
  Write-Host "`n=== $platform ===" -ForegroundColor Cyan
  & "$PSScriptRoot\smoke-test.ps1" -Platform $platform
  if ($LASTEXITCODE -ne 0) {
    $failed += $platform
  }
}

if ($failed.Count -gt 0) {
  Write-Host "`nFAILED platforms: $($failed -join ', ')" -ForegroundColor Red
  exit 1
}

Write-Host "`nALL PLATFORMS PASS ($($platforms.Count)/$($platforms.Count))" -ForegroundColor Green
