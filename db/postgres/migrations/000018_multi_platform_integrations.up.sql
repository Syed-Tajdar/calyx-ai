-- Dev integration seeds for multi-platform webhook ingestion.
INSERT INTO integrations (tenant_id, platform, name, webhook_token, webhook_secret) VALUES
    ('11111111-1111-1111-1111-111111111111',
     'retreaver',
     'Retreaver Dev Postback',
     'dev_retreaver_token_change_me',
     'dev_retreaver_hmac_secret_change_me'),
    ('11111111-1111-1111-1111-111111111111',
     'trackdrive',
     'TrackDrive Dev Postback',
     'dev_trackdrive_token_change_me',
     'dev_trackdrive_auth_token_change_me'),
    ('11111111-1111-1111-1111-111111111111',
     'callgrid',
     'CallGrid Dev Postback',
     'dev_callgrid_token_change_me',
     'dev_callgrid_hmac_secret_change_me')
ON CONFLICT (webhook_token) DO NOTHING;
