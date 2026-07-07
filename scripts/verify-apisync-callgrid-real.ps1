$ErrorActionPreference = "Stop"
if (-not $env:CALLGRID_API_KEY) {
  Write-Host "Set CALLGRID_API_KEY" -ForegroundColor Red
  exit 1
}
& "$PSScriptRoot\verify-apisync-platform.ps1" -Platform callgrid -Credentials @{
  api_key = $env:CALLGRID_API_KEY
}
