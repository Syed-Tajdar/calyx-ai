-- Dedicated application roles (non-superuser).
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'callloom_api') THEN
        CREATE ROLE callloom_api LOGIN PASSWORD 'api_dev';
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'callloom_ingestion') THEN
        CREATE ROLE callloom_ingestion LOGIN PASSWORD 'ingestion_dev';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE callloom TO callloom_api, callloom_ingestion;
GRANT USAGE ON SCHEMA public TO callloom_api, callloom_ingestion;

-- Force RLS even for table owners.
ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
ALTER TABLE integrations FORCE ROW LEVEL SECURITY;

-- API role: tenant-scoped reads via RLS.
GRANT SELECT ON tenants, users, integrations TO callloom_api;

-- Ingestion role: webhook lookup function only.
GRANT EXECUTE ON FUNCTION lookup_integration_by_token(TEXT) TO callloom_ingestion;

-- SECURITY DEFINER auth for login (cross-tenant email lookup).
CREATE OR REPLACE FUNCTION authenticate_user(p_email TEXT, p_password TEXT)
RETURNS TABLE (
    id        UUID,
    tenant_id UUID,
    email     TEXT,
    role      TEXT
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT u.id, u.tenant_id, u.email, u.role
    FROM users u
    WHERE u.email = p_email
      AND u.password_hash = crypt(p_password, u.password_hash)
    LIMIT 1;
$$;

GRANT EXECUTE ON FUNCTION authenticate_user(TEXT, TEXT) TO callloom_api;

-- Dev Ringba integration: mandatory HMAC secret.
UPDATE integrations
SET webhook_secret = 'dev_ringba_hmac_secret_change_me'
WHERE webhook_token = 'dev_ringba_token_change_me'
  AND (webhook_secret IS NULL OR webhook_secret = '');
