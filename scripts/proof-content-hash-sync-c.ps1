# Content-hash Sync C proof — run on rebuilt stack with MOCK_DEDUP_PROOF=1 / REVENUE=99.99
$ErrorActionPreference = "Stop"
$apiBase = "http://localhost:3100"
$integrationId = if ($env:DEDUP_PROOF_INTEGRATION_ID) { $env:DEDUP_PROOF_INTEGRATION_ID } else { "c78106df-723f-4694-9401-e45af3c47926" }

$login = Invoke-RestMethod -Uri "$apiBase/auth/login" -Method POST -ContentType "application/json" -Body '{"email":"admin@calyx.ai","password":"calyx_dev"}'
$h = @{ Authorization = "Bearer $($login.access_token)" }

Write-Host "BEFORE:"
docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "SELECT count(), uniqExact(external_call_id), max(revenue) FROM pings WHERE external_call_id LIKE 'smoke-proof-retreaver-%'"

Invoke-RestMethod -Uri "$apiBase/integrations/$integrationId/sync" -Method POST -Headers $h | Out-Null
Start-Sleep -Seconds 18

Write-Host "AFTER:"
docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "SELECT count(), uniqExact(external_call_id), max(revenue) FROM pings WHERE external_call_id LIKE 'smoke-proof-retreaver-%'"
docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev --query "SELECT external_call_id, argMax(revenue, received_at), count() FROM pings WHERE external_call_id='smoke-proof-retreaver-0' GROUP BY external_call_id"
docker logs deploy-platform-sync-1 2>&1 | Select-String "api sync completed" | Select-Object -Last 3
