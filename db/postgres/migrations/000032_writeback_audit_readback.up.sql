-- Stage 4b: per-platform write-back. Retreaver audit rows record which platform was written
-- and the read-back fields (converted, revenue, payout, receivable, payable,
-- conversions_determined_at, fired_pixels_count, postback_value, updated_at) before and after.
-- No credentials are ever stored here. Ringba rows keep using insert_writeback_audit (11 args);
-- their platform column stays NULL, which means "ringba" for rows written before or by it.

ALTER TABLE writeback_audit ADD COLUMN IF NOT EXISTS platform TEXT;
ALTER TABLE writeback_audit ADD COLUMN IF NOT EXISTS read_back JSONB;

CREATE OR REPLACE FUNCTION insert_writeback_audit_v2(
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
    p_offline_conversion_uploaded TEXT,
    p_platform TEXT,
    p_read_back JSONB
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
        outcome, ringba_response_ref, campaign_id, message, offline_conversion_uploaded,
        platform, read_back
    ) VALUES (
        p_tenant_id, p_user_id, p_integration_id, p_inbound_call_id, p_action, p_amount,
        p_outcome, p_ringba_response_ref, p_campaign_id, p_message, p_offline_conversion_uploaded,
        p_platform, p_read_back
    )
    RETURNING id INTO new_id;
    RETURN new_id;
END;
$$;

GRANT EXECUTE ON FUNCTION insert_writeback_audit_v2(UUID, UUID, UUID, TEXT, TEXT, NUMERIC, TEXT, TEXT, TEXT, TEXT, TEXT, TEXT, JSONB) TO callloom_sync;
