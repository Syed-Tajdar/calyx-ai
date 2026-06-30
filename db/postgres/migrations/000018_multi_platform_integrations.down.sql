DELETE FROM integrations
WHERE webhook_token IN (
    'dev_retreaver_token_change_me',
    'dev_trackdrive_token_change_me',
    'dev_callgrid_token_change_me'
);
