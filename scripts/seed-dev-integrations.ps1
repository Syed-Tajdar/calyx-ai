# Re-seed dev webhook integrations (idempotent). Mirrors migrations 000001, 000002, 000018.
$ErrorActionPreference = "Stop"

$tenantId = "11111111-1111-1111-1111-111111111111"

$sql = @"
INSERT INTO integrations (tenant_id, platform, name, webhook_token, webhook_secret, connection_mode, sync_mode)
VALUES
  ('$tenantId', 'ringba', 'Ringba Dev Pixel', 'dev_ringba_token_change_me', 'dev_ringba_hmac_secret_change_me', 'webhook', 'mock'),
  ('$tenantId', 'retreaver', 'Retreaver Dev Postback', 'dev_retreaver_token_change_me', 'dev_retreaver_hmac_secret_change_me', 'webhook', 'mock'),
  ('$tenantId', 'trackdrive', 'TrackDrive Dev Postback', 'dev_trackdrive_token_change_me', 'dev_trackdrive_auth_token_change_me', 'webhook', 'mock'),
  ('$tenantId', 'callgrid', 'CallGrid Dev Postback', 'dev_callgrid_token_change_me', 'dev_callgrid_hmac_secret_change_me', 'webhook', 'mock')
ON CONFLICT (webhook_token) DO UPDATE SET
  webhook_secret = EXCLUDED.webhook_secret,
  is_active = TRUE,
  connection_mode = 'webhook',
  updated_at = NOW();

UPDATE integrations
SET webhook_secret = 'dev_ringba_hmac_secret_change_me'
WHERE webhook_token = 'dev_ringba_token_change_me'
  AND (webhook_secret IS NULL OR webhook_secret = '');
"@

Write-Host "=== Seeding dev webhook integrations ===" -ForegroundColor Cyan
docker exec -i deploy-postgres-1 psql -U callloom -d callloom -v ON_ERROR_STOP=1 -c $sql

$check = docker exec deploy-postgres-1 psql -U callloom -d callloom -t -A -c `
  "SELECT platform, webhook_token FROM integrations WHERE tenant_id = '$tenantId' AND connection_mode = 'webhook' ORDER BY platform;"

Write-Host $check
Write-Host "PASS: dev webhook integrations seeded" -ForegroundColor Green
