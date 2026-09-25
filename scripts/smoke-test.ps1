# Multi-platform ingestion smoke test.
# Usage: smoke-test.ps1 [-Platform ringba|retreaver|trackdrive|callgrid]
param(
    [ValidateSet("ringba", "retreaver", "trackdrive", "callgrid")]
    [string]$Platform = "ringba"
)

$callId = "smoke-" + [guid]::NewGuid().ToString()
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
$repoRoot = Split-Path -Parent $PSScriptRoot
$ingestionDir = Join-Path $repoRoot "services\ingestion"

switch ($Platform) {
    "ringba" {
        $token = "dev_ringba_token_change_me"
        $query = "CallID=$callId&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50"
        $url = "http://localhost:8080/webhooks/ringba/$token" + "?" + $query
        Push-Location $ingestionDir
        $signature = (go run ./cmd/platformsig ringba $query).Trim()
        Pop-Location
        $headers = @{ "X-Ringba-Signature" = $signature }
    }
    "retreaver" {
        $token = "dev_retreaver_token_change_me"
        $callKey = "smoke_call_key_$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $query = "call_uuid=$callId&call_key=$callKey&caller=15551234567&state=CA&event=incoming&revenue=9.75"
        $url = "http://localhost:8080/webhooks/retreaver/$token" + "?" + $query
        Push-Location $ingestionDir
        $signature = (go run ./cmd/platformsig retreaver $query).Trim()
        Pop-Location
        $headers = @{ "X-Retreaver-Signature" = $signature }
    }
    "trackdrive" {
        $token = "dev_trackdrive_token_change_me"
        $authToken = "dev_trackdrive_auth_token_change_me"
        $query = "call_uuid=$callId&caller_number=15551234567&state=FL&event=buyer_converted&revenue=15.00&lead_token=$authToken"
        $url = "http://localhost:8080/webhooks/trackdrive/$token" + "?" + $query
        $headers = @{}
    }
    "callgrid" {
        $token = "dev_callgrid_token_change_me"
        $query = "call_id=$callId&caller=15551234567&state=NY&event=incoming&revenue=11.25"
        $url = "http://localhost:8080/webhooks/callgrid/$token" + "?" + $query
        Push-Location $ingestionDir
        $signature = (go run ./cmd/platformsig callgrid $query).Trim()
        Pop-Location
        $headers = @{ "X-CallGrid-Signature" = $signature }
    }
}

Write-Host "[$Platform] Sending test ping (call_id=$callId)"
$r = Invoke-WebRequest -Uri $url -Headers $headers -UseBasicParsing
Write-Host "Ingestion response: $($r.StatusCode)"
if ($r.StatusCode -ne 200) { exit 1 }

Start-Sleep -Seconds 8

$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" `
  -Headers @{ "X-Calyx-Client" = "script" } `
  -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
if (-not $login.access_token) { throw "login missing access_token" }

$pings = Invoke-RestMethod -Uri "http://localhost:3000/pings?limit=20&show_demo=true" -Method GET `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }

$found = $pings.data | Where-Object { $_.external_call_id -eq $callId -and $_.platform -eq $Platform }
if ($found) {
  Write-Host "E2E PASS ($Platform)"
} else {
  Write-Host "E2E FAIL ($Platform) - ping not found"
  exit 1
}
$found | Format-Table platform, external_call_id, state, revenue, event_type

if ($Platform -eq "retreaver") {
    $hasKey = docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "SELECT call_key != '' FROM default.pings WHERE platform='retreaver' AND external_call_id='$callId' LIMIT 1"
    if ($hasKey.Trim() -ne "1") {
        Write-Host "E2E FAIL (retreaver) - call_key not stored in ClickHouse"
        exit 1
    }
    Write-Host "call_key stored in ClickHouse (value not logged)"
}
