$ev = "c:\Users\DELL\calyx-AI\docs\evidence\s0-verify"
$t0 = [DateTimeOffset]::Parse((Get-Content "$ev\T0_stamp.txt" -Raw).Trim()).UtcDateTime
$targets = @(30,60,120)
foreach ($m in $targets) {
  $due = $t0.AddMinutes($m)
  Write-Host ("waiting until UTC " + $due.ToString("o") + " for T0+" + $m + "m")
  while ([DateTime]::UtcNow -lt $due) {
    $wait = [Math]::Min(60, ($due - [DateTime]::UtcNow).TotalSeconds)
    if ($wait -gt 0) { Start-Sleep -Seconds ([Math]::Ceiling($wait)) }
  }
  $label = "T0plus${m}m"
  $stamp = [DateTimeOffset]::UtcNow.ToString("o")
  $stamp | Set-Content "$ev\${label}_stamp.txt"
  docker exec deploy-postgres-1 psql -U callloom -d callloom -t -A -F "|" -c "SELECT last_sync_at::text, sync_cursor, sync_status FROM integrations WHERE id='ce6fd338-af48-47d8-850a-bd10b855e2f4'" | Set-Content "$ev\${label}_pg.txt"
  docker exec deploy-clickhouse-1 clickhouse-client --password callloom_dev -q "SELECT max(occurred_at), max(received_at), count() FROM pings WHERE platform='ringba' AND sync_path='api_sync' AND external_call_id NOT LIKE 'smoke-%' AND external_call_id NOT LIKE 'demo-%' AND external_call_id NOT LIKE 'apisync-%'" | Set-Content "$ev\${label}_ch.txt"
  docker logs deploy-platform-sync-1 --since 35m 2>&1 | Select-String "ce6fd338" | Select-Object -Last 25 | Set-Content "$ev\${label}_logs.txt"
  Write-Output ("AGENT_LOOP_TICK_s0v1 {""prompt"":""S0 V1 sample " + $label + " ready. Read docs/evidence/s0-verify/" + $label + "_*.txt. Recreate cmd/probe-s0-last3h, run Ringba last3h, delete probe. If T0plus120m: close V1, reconcile V1.3, then V2, write final report, STOP.""}")
}
