UPDATE users
SET email = 'admin@callloom.dev',
    password_hash = crypt('callloom_dev', gen_salt('bf')),
    updated_at = NOW()
WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
  AND email = 'admin@calyx.ai';
