$ev = "c:\Users\DELL\calyx-AI\docs\evidence\s0-verify"
while (-not (Test-Path "$ev\T0plus30m_pg.txt")) { Start-Sleep -Seconds 30 }
Write-Output 'AGENT_LOOP_TICK_s0v1 {"prompt":"S0 V1 T0+30m sample files ready. Read docs/evidence/s0-verify/T0plus30m_*.txt, run Ringba last-3h probe (recreate under cmd/probe-s0-last3h, run, delete), update running notes. Do NOT start V2."}'
while (-not (Test-Path "$ev\T0plus60m_pg.txt")) { Start-Sleep -Seconds 30 }
Write-Output 'AGENT_LOOP_TICK_s0v1 {"prompt":"S0 V1 T0+60m sample ready. Read T0plus60m_*.txt + Ringba last3h. Do NOT start V2."}'
while (-not (Test-Path "$ev\T0plus120m_pg.txt")) { Start-Sleep -Seconds 30 }
Write-Output 'AGENT_LOOP_TICK_s0v1 {"prompt":"S0 V1 T0+120m sample ready — CLOSE V1 window. Run V1.3 reconcile, then V2 (CH stop/start + smoke pings), finish V3 leftovers, write docs/s0-verify-report.md, STOP."}'
