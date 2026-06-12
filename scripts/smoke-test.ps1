# Smoke test: send a Ringba pixel to ingestion and verify API returns it.
$token = "dev_ringba_token_change_me"
$callId = [guid]::NewGuid().ToString()
$url = "http://localhost:8080/webhooks/ringba/$token" +
  "?CallID=$callId&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50"

Write-Host "Sending test ping to $url"
Invoke-RestMethod -Uri $url -Method GET

Start-Sleep -Seconds 8

$login = Invoke-RestMethod -Uri "http://localhost:3000/auth/login" -Method POST `
  -ContentType "application/json" `
  -Body '{"email":"admin@callloom.dev","password":"callloom_dev"}'

$pings = Invoke-RestMethod -Uri "http://localhost:3000/pings?limit=5" -Method GET `
  -Headers @{ Authorization = "Bearer $($login.access_token)" }

Write-Host "Pings returned: $($pings.total)"
$pings.data | Format-Table platform, external_call_id, state, revenue, event_type
