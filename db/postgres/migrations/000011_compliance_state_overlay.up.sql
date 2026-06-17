CREATE TABLE compliance_state_overlay (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    state_code   CHAR(2) NOT NULL,
    rules_json   JSONB NOT NULL DEFAULT '{}',
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, state_code)
);

ALTER TABLE compliance_state_overlay ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_state_overlay FORCE ROW LEVEL SECURITY;

CREATE POLICY compliance_state_overlay_isolation ON compliance_state_overlay
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON compliance_state_overlay TO callloom_api;

INSERT INTO compliance_state_overlay (tenant_id, state_code, rules_json) VALUES
    ('11111111-1111-1111-1111-111111111111', 'TX', '{"calling_hours_start":8,"calling_hours_end":21}'::jsonb),
    ('11111111-1111-1111-1111-111111111111', 'CA', '{"calling_hours_start":8,"calling_hours_end":20}'::jsonb)
ON CONFLICT (tenant_id, state_code) DO NOTHING;
