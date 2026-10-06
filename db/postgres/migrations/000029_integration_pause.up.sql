-- Pause is separate from delete. A paused row stays in Settings (is_active = false,
-- deleted_at IS NULL). Delete sets deleted_at and drops the row from the list.
-- Existing is_active = false rows were the old soft-delete; they stay visible as Paused
-- until someone deletes them for real.

ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE TABLE integration_settings_audit (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    user_id         UUID,
    integration_id  UUID,
    action          TEXT NOT NULL CHECK (action IN ('pause', 'resume', 'delete')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE integration_settings_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration_settings_audit FORCE ROW LEVEL SECURITY;

CREATE POLICY integration_settings_audit_isolation ON integration_settings_audit
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT INSERT, SELECT ON integration_settings_audit TO callloom_api;
