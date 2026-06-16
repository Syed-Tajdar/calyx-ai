CREATE TABLE scorecards (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    vertical    TEXT NOT NULL DEFAULT 'general',
    rules       JSONB NOT NULL DEFAULT '[]',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_scorecards_tenant_id ON scorecards(tenant_id);

ALTER TABLE scorecards ENABLE ROW LEVEL SECURITY;
ALTER TABLE scorecards FORCE ROW LEVEL SECURITY;

CREATE POLICY scorecards_isolation ON scorecards
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON scorecards TO callloom_api;
