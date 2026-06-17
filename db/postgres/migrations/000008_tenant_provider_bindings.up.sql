CREATE TABLE tenant_provider_bindings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    domain       TEXT NOT NULL,
    provider_id  TEXT NOT NULL,
    config_json  JSONB NOT NULL DEFAULT '{}',
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    priority     INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, domain, provider_id)
);

CREATE INDEX idx_tenant_provider_bindings_tenant_domain
    ON tenant_provider_bindings(tenant_id, domain)
    WHERE is_active = TRUE;

ALTER TABLE tenant_provider_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_provider_bindings FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_provider_bindings_isolation ON tenant_provider_bindings
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON tenant_provider_bindings TO callloom_api;
