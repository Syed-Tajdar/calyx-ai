-- Rebrand demo admin login from Callloom to Calyx AI.
UPDATE users
SET email = 'admin@calyx.ai',
    password_hash = crypt('calyx_dev', gen_salt('bf')),
    updated_at = NOW()
WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
  AND email = 'admin@callloom.dev';
