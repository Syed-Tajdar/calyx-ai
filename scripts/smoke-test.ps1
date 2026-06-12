# Smoke test: send a signed Ringba pixel to ingestion and verify API returns it.
$token = "dev_ringba_token_change_me"
$secret = "dev_ringba_hmac_secret_change_me"
$callId = [guid]::NewGuid().ToString()
$query = "CallID=$callId&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50"
$url = "http://localhost:8080/webhooks/ringba/$token" + "?" + $query

$hmac = New-Object System.Security.Cryptography.HMACSHA256
$hmac.Key = [Text.Encoding]::UTF8.GetBytes($secret)
$hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes($query))
$signature = -join ($hash | ForEach-Object { $_.ToString("x2") })

Write-Host "Sending signed test ping"
Invoke-RestMethod -Uri $url -Method GET -Headers @{ "X-Ringba-Signature" = $signature }

Start-Sleep -Seconds 8

$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" `
  -Body '{"email":"admin@callloom.dev","password":"callloom_dev"}'

$pings = Invoke-RestMethod -Uri "http://localhost:3000/pings?limit=5" -Method GET `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }

Write-Host "Pings returned: $($pings.total)"
$pings.data | Format-Table platform, external_call_id, state, revenue, event_type
