CREATE TABLE adspend_entries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    campaign_id TEXT NOT NULL DEFAULT '',
    spend_date  DATE NOT NULL,
    amount      NUMERIC(12, 4) NOT NULL DEFAULT 0,
    source      TEXT NOT NULL DEFAULT 'manual',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, campaign_id, spend_date, source)
);

CREATE INDEX idx_adspend_tenant_date ON adspend_entries(tenant_id, spend_date);

ALTER TABLE adspend_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE adspend_entries FORCE ROW LEVEL SECURITY;

CREATE POLICY adspend_isolation ON adspend_entries
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON adspend_entries TO callloom_api;
