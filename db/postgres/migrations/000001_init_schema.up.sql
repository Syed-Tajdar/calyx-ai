CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'admin',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, email)
);

CREATE TABLE integrations (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    platform       TEXT NOT NULL,
    name           TEXT NOT NULL,
    webhook_token  TEXT NOT NULL UNIQUE,
    webhook_secret TEXT,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, platform, name)
);

CREATE INDEX idx_integrations_tenant_id ON integrations(tenant_id);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE integrations ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenants_isolation ON tenants
    USING (id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY users_isolation ON users
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY integrations_isolation ON integrations
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

-- SECURITY DEFINER lookup for ingestion service (cross-tenant token resolution).
CREATE OR REPLACE FUNCTION lookup_integration_by_token(p_token TEXT)
RETURNS TABLE (
    tenant_id      UUID,
    platform       TEXT,
    webhook_secret TEXT
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT i.tenant_id, i.platform, COALESCE(i.webhook_secret, '')
    FROM integrations i
    WHERE i.webhook_token = p_token AND i.is_active = TRUE
    LIMIT 1;
$$;

-- Dev seed tenant + admin user (password: calyx_dev)
INSERT INTO tenants (id, name) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Calyx AI Dev');

INSERT INTO users (tenant_id, email, password_hash, role) VALUES
    ('11111111-1111-1111-1111-111111111111',
     'admin@calyx.ai',
     crypt('calyx_dev', gen_salt('bf')),
     'admin');

INSERT INTO integrations (tenant_id, platform, name, webhook_token) VALUES
    ('11111111-1111-1111-1111-111111111111',
     'ringba',
     'Ringba Dev Pixel',
     'dev_ringba_token_change_me');
