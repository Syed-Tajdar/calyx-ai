$ErrorActionPreference = "Stop"
if (-not $env:RETREAVER_API_KEY -or -not $env:RETREAVER_COMPANY_ID) {
  Write-Host "Set RETREAVER_API_KEY and RETREAVER_COMPANY_ID" -ForegroundColor Red
  exit 1
}
& "$PSScriptRoot\verify-apisync-platform.ps1" -Platform retreaver -Credentials @{
  api_key = $env:RETREAVER_API_KEY
  company_id = $env:RETREAVER_COMPANY_ID
}
