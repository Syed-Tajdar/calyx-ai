$ErrorActionPreference = "Stop"
if (-not $env:TRACKDRIVE_SUBDOMAIN -or -not $env:TRACKDRIVE_PUBLIC_KEY -or -not $env:TRACKDRIVE_PRIVATE_KEY) {
  Write-Host "Set TRACKDRIVE_SUBDOMAIN, TRACKDRIVE_PUBLIC_KEY, TRACKDRIVE_PRIVATE_KEY" -ForegroundColor Red
  exit 1
}
& "$PSScriptRoot\verify-apisync-platform.ps1" -Platform trackdrive -Credentials @{
  subdomain = $env:TRACKDRIVE_SUBDOMAIN
  public_key = $env:TRACKDRIVE_PUBLIC_KEY
  private_key = $env:TRACKDRIVE_PRIVATE_KEY
}
