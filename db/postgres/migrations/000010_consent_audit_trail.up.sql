CREATE TABLE consent_audit_trail (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    call_id         TEXT NOT NULL,
    ani_hash        CHAR(64) NOT NULL,
    consent_source  TEXT NOT NULL DEFAULT 'unknown',
    consent_type    TEXT NOT NULL DEFAULT 'express',
    captured_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata_json   JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, call_id)
);

CREATE INDEX idx_consent_audit_tenant_call ON consent_audit_trail(tenant_id, call_id);

ALTER TABLE consent_audit_trail ENABLE ROW LEVEL SECURITY;
ALTER TABLE consent_audit_trail FORCE ROW LEVEL SECURITY;

CREATE POLICY consent_audit_isolation ON consent_audit_trail
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON consent_audit_trail TO callloom_api;
