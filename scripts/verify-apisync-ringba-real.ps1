$ErrorActionPreference = "Stop"
if (-not $env:RINGBA_API_TOKEN -or -not $env:RINGBA_ACCOUNT_ID) {
  Write-Host "Set RINGBA_API_TOKEN and RINGBA_ACCOUNT_ID" -ForegroundColor Red
  exit 1
}
& "$PSScriptRoot\verify-apisync-platform.ps1" -Platform ringba -Credentials @{
  api_token = $env:RINGBA_API_TOKEN
  account_id = $env:RINGBA_ACCOUNT_ID
}
