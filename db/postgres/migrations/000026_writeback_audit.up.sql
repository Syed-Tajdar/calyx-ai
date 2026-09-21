-- Immutable write-back audit. INSERT only. Never updated or deleted by app roles.
CREATE TABLE writeback_audit (
    id                           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                    UUID NOT NULL REFERENCES tenants(id),
    user_id                      UUID REFERENCES users(id),
    integration_id               UUID REFERENCES integrations(id),
    inbound_call_id              TEXT NOT NULL,
    action                       TEXT NOT NULL CHECK (action IN ('convert', 'void')),
    amount                       NUMERIC(12, 2) NOT NULL,
    requested_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    outcome                      TEXT NOT NULL CHECK (outcome IN ('confirmed', 'pending', 'failed')),
    ringba_response_ref          TEXT,
    campaign_id                  TEXT,
    message                      TEXT,
    offline_conversion_uploaded  TEXT
);

CREATE INDEX idx_writeback_audit_tenant_call
    ON writeback_audit (tenant_id, inbound_call_id, requested_at DESC);

ALTER TABLE writeback_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE writeback_audit FORCE ROW LEVEL SECURITY;

CREATE POLICY writeback_audit_isolation ON writeback_audit
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE OR REPLACE FUNCTION writeback_audit_immutable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'writeback_audit is immutable';
END;
$$;

CREATE TRIGGER writeback_audit_no_update
    BEFORE UPDATE OR DELETE ON writeback_audit
    FOR EACH ROW
    EXECUTE FUNCTION writeback_audit_immutable();

CREATE OR REPLACE FUNCTION insert_writeback_audit(
    p_tenant_id UUID,
    p_user_id UUID,
    p_integration_id UUID,
    p_inbound_call_id TEXT,
    p_action TEXT,
    p_amount NUMERIC,
    p_outcome TEXT,
    p_ringba_response_ref TEXT,
    p_campaign_id TEXT,
    p_message TEXT,
    p_offline_conversion_uploaded TEXT
)
RETURNS UUID
SECURITY DEFINER
SET search_path = public
LANGUAGE plpgsql
AS $$
DECLARE
    new_id UUID;
BEGIN
    INSERT INTO writeback_audit (
        tenant_id, user_id, integration_id, inbound_call_id, action, amount,
        outcome, ringba_response_ref, campaign_id, message, offline_conversion_uploaded
    ) VALUES (
        p_tenant_id, p_user_id, p_integration_id, p_inbound_call_id, p_action, p_amount,
        p_outcome, p_ringba_response_ref, p_campaign_id, p_message, p_offline_conversion_uploaded
    )
    RETURNING id INTO new_id;
    RETURN new_id;
END;
$$;

GRANT EXECUTE ON FUNCTION insert_writeback_audit(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT) TO callloom_sync;
GRANT SELECT ON writeback_audit TO callloom_api, callloom_sync;
