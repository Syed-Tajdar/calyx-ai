CREATE TABLE enrichment_cache_meta (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ani_hash        TEXT NOT NULL,
    provider        TEXT NOT NULL DEFAULT 'mock',
    attribute_count INTEGER NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, ani_hash)
);

CREATE INDEX idx_enrichment_cache_tenant ON enrichment_cache_meta(tenant_id);
CREATE INDEX idx_enrichment_cache_expires ON enrichment_cache_meta(expires_at);

ALTER TABLE enrichment_cache_meta ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrichment_cache_meta FORCE ROW LEVEL SECURITY;

CREATE POLICY enrichment_cache_isolation ON enrichment_cache_meta
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON enrichment_cache_meta TO callloom_api;
