-- v2 Phase 2D — Workflows: alert → action, with publisher pause.
-- All tables are tenant-scoped with RLS on app.current_tenant (same pattern as every other table).
-- No credentials or platform URLs are stored here.

-- Per-integration toggle "Allow Calyx to pause publishers" (off; admin turns it on; Live only).
ALTER TABLE integrations
    ADD COLUMN IF NOT EXISTS allow_publisher_pause BOOLEAN NOT NULL DEFAULT FALSE;

-- Audit the toggle in the existing integration settings audit (who, when, which integration).
ALTER TABLE integration_settings_audit DROP CONSTRAINT IF EXISTS integration_settings_audit_action_check;
ALTER TABLE integration_settings_audit ADD CONSTRAINT integration_settings_audit_action_check
    CHECK (action IN ('pause', 'resume', 'delete', 'allow_pause_on', 'allow_pause_off'));

-- Rules: IF trigger over window (with a minimum number of calls) THEN actions.
CREATE TABLE IF NOT EXISTS workflow_rules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    name              TEXT NOT NULL,
    trigger_type      TEXT NOT NULL CHECK (trigger_type IN (
                          'conversion_rate_below', 'short_call_share_above', 'repeat_caller_share_above',
                          'volume_change', 'revenue_drop', 'no_calls', 'integration_unhealthy',
                          -- QA triggers (Phase 2C): accepted in the schema, evaluated once QA exists.
                          'qa_flag_rate_above', 'qa_score_below', 'should_convert_share_above', 'do_not_call_requests')),
    params            JSONB NOT NULL DEFAULT '{}'::jsonb,   -- thresholds, e.g. {"threshold_pct": 5, "short_seconds": 20}
    window_hours      INTEGER NOT NULL DEFAULT 24 CHECK (window_hours BETWEEN 1 AND 720),
    min_calls         INTEGER NOT NULL DEFAULT 20 CHECK (min_calls >= 0),
    scope             TEXT NOT NULL DEFAULT 'publisher' CHECK (scope IN ('publisher', 'campaign', 'tenant')),
    platform          TEXT CHECK (platform IN ('ringba', 'retreaver')),   -- NULL = both
    notify_email      BOOLEAN NOT NULL DEFAULT FALSE,
    action_mode       TEXT NOT NULL DEFAULT 'notify' CHECK (action_mode IN ('notify', 'suggest', 'auto')),
    suggest_since     TIMESTAMPTZ,          -- when the rule entered suggest mode (auto needs >= 7 days)
    pause_hours       INTEGER CHECK (pause_hours IS NULL OR pause_hours BETWEEN 1 AND 720),
    cooldown_hours    INTEGER NOT NULL DEFAULT 6 CHECK (cooldown_hours BETWEEN 1 AND 168),
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_by        UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_workflow_rules_tenant ON workflow_rules (tenant_id) WHERE deleted_at IS NULL;

-- Alert centre (the bell). One row per rule × entity firing (cooldown applies per rule × entity).
CREATE TABLE IF NOT EXISTS alert_events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    rule_id           UUID REFERENCES workflow_rules(id),
    trigger_type      TEXT NOT NULL,
    scope             TEXT NOT NULL,
    platform          TEXT,
    entity_id         TEXT NOT NULL DEFAULT '',      -- publisher id / campaign id / '' for tenant
    entity_name       TEXT NOT NULL DEFAULT '',
    integration_id    UUID REFERENCES integrations(id),
    numbers           JSONB NOT NULL DEFAULT '{}'::jsonb,  -- the numbers the rule used
    window_from       TIMESTAMPTZ,
    window_to         TIMESTAMPTZ,
    status            TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'acknowledged', 'actioned', 'resolved')),
    action            TEXT NOT NULL DEFAULT 'none' CHECK (action IN ('none', 'suggested', 'paused', 'pause_blocked', 'pause_failed', 'unpaused')),
    action_detail     JSONB NOT NULL DEFAULT '{}'::jsonb,
    emailed           BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by   UUID REFERENCES users(id),
    acknowledged_at   TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alert_events_tenant_created ON alert_events (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_cooldown ON alert_events (tenant_id, rule_id, entity_id, created_at DESC);

-- Publishers that are never auto-paused (Suggest only).
CREATE TABLE IF NOT EXISTS protected_publishers (
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    platform      TEXT NOT NULL CHECK (platform IN ('ringba', 'retreaver')),
    publisher_id  TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    created_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, platform, publisher_id)
);

-- Calyx's own record of pauses it made (Retreaver's GET may not show "paused"; also drives the
-- "pause for N hours" timer).
CREATE TABLE IF NOT EXISTS publisher_pauses (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id),
    integration_id   UUID NOT NULL REFERENCES integrations(id),
    platform         TEXT NOT NULL CHECK (platform IN ('ringba', 'retreaver')),
    publisher_id     TEXT NOT NULL,
    publisher_name   TEXT NOT NULL DEFAULT '',
    mode             TEXT NOT NULL CHECK (mode IN ('manual', 'auto')),
    alert_event_id   UUID REFERENCES alert_events(id),
    rule_id          UUID REFERENCES workflow_rules(id),
    paused_by        UUID REFERENCES users(id),     -- NULL = auto
    paused_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unpause_at       TIMESTAMPTZ,                   -- timer from the rule's pause_hours
    unpaused_at      TIMESTAMPTZ,
    unpaused_by      UUID REFERENCES users(id),     -- NULL with unpaused_at = timer
    confirmation     TEXT NOT NULL DEFAULT 'read_back' CHECK (confirmation IN ('read_back', 'response'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_publisher_pauses_open
    ON publisher_pauses (tenant_id, platform, publisher_id) WHERE unpaused_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_publisher_pauses_timer
    ON publisher_pauses (unpause_at) WHERE unpaused_at IS NULL AND unpause_at IS NOT NULL;

-- Immutable audit of every pause / unpause attempt, including blocked ones.
CREATE TABLE IF NOT EXISTS publisher_action_audit (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL REFERENCES tenants(id),
    user_id           UUID REFERENCES users(id),     -- NULL = auto / timer
    integration_id    UUID REFERENCES integrations(id),
    platform          TEXT NOT NULL,
    publisher_id      TEXT NOT NULL,
    action            TEXT NOT NULL CHECK (action IN ('pause', 'unpause')),
    mode              TEXT NOT NULL CHECK (mode IN ('manual', 'auto', 'timer')),
    rule_id           UUID,
    alert_event_id    UUID,
    outcome           TEXT NOT NULL CHECK (outcome IN ('confirmed', 'confirmed_by_response', 'failed', 'rejected')),
    read_back         TEXT NOT NULL DEFAULT 'none' CHECK (read_back IN ('ok', 'none', 'mismatch')),
    message           TEXT,
    request_redacted  TEXT,                          -- method + path with every key redacted
    numbers           JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_publisher_action_audit_tenant ON publisher_action_audit (tenant_id, created_at DESC);

CREATE OR REPLACE FUNCTION publisher_action_audit_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'publisher_action_audit is immutable';
END;
$$;
DROP TRIGGER IF EXISTS publisher_action_audit_no_update ON publisher_action_audit;
CREATE TRIGGER publisher_action_audit_no_update
    BEFORE UPDATE OR DELETE ON publisher_action_audit
    FOR EACH ROW EXECUTE FUNCTION publisher_action_audit_immutable();

-- RLS everywhere.
DO $$
DECLARE t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['workflow_rules','alert_events','protected_publishers','publisher_pauses','publisher_action_audit'] LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', t || '_isolation', t);
        EXECUTE format(
            'CREATE POLICY %I ON %I USING (tenant_id = current_setting(''app.current_tenant'', true)::uuid) '
            'WITH CHECK (tenant_id = current_setting(''app.current_tenant'', true)::uuid)',
            t || '_isolation', t);
    END LOOP;
END $$;

GRANT SELECT, INSERT, UPDATE ON workflow_rules TO callloom_api;
GRANT SELECT, INSERT, UPDATE ON alert_events TO callloom_api;
GRANT SELECT, INSERT, DELETE ON protected_publishers TO callloom_api;
GRANT SELECT, INSERT, UPDATE ON publisher_pauses TO callloom_api;
-- The API audits rejections it decides itself (guardrails); platform-sync audits every platform attempt.
GRANT SELECT, INSERT ON publisher_action_audit TO callloom_api;

-- platform-sync: read the integration for a pause (also paused/test rows, so it can refuse them with a
-- clear reason) and write the audit row. SECURITY DEFINER, like the write-back functions.
CREATE OR REPLACE FUNCTION get_publisher_pause_integration(p_tenant_id UUID, p_id UUID)
RETURNS TABLE (
    id                     UUID,
    tenant_id              UUID,
    platform               TEXT,
    name                   TEXT,
    sync_mode              TEXT,
    is_active              BOOLEAN,
    allow_publisher_pause  BOOLEAN,
    api_credentials_blob   BYTEA
)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT id, tenant_id, platform, name, sync_mode, is_active, allow_publisher_pause, api_credentials_blob
    FROM integrations
    WHERE id = p_id
      AND tenant_id = p_tenant_id
      AND connection_mode = 'api_sync'
      AND deleted_at IS NULL
      AND api_credentials_blob IS NOT NULL;
$$;
GRANT EXECUTE ON FUNCTION get_publisher_pause_integration(UUID, UUID) TO callloom_sync;

CREATE OR REPLACE FUNCTION insert_publisher_action_audit(
    p_tenant_id UUID, p_user_id UUID, p_integration_id UUID, p_platform TEXT, p_publisher_id TEXT,
    p_action TEXT, p_mode TEXT, p_rule_id UUID, p_alert_event_id UUID, p_outcome TEXT, p_read_back TEXT,
    p_message TEXT, p_request_redacted TEXT, p_numbers JSONB
)
RETURNS UUID
SECURITY DEFINER
SET search_path = public
LANGUAGE plpgsql
AS $$
DECLARE new_id UUID;
BEGIN
    INSERT INTO publisher_action_audit (
        tenant_id, user_id, integration_id, platform, publisher_id, action, mode, rule_id, alert_event_id,
        outcome, read_back, message, request_redacted, numbers
    ) VALUES (
        p_tenant_id, p_user_id, p_integration_id, p_platform, p_publisher_id, p_action, p_mode, p_rule_id,
        p_alert_event_id, p_outcome, p_read_back, p_message, p_request_redacted, COALESCE(p_numbers, '{}'::jsonb)
    ) RETURNING id INTO new_id;
    RETURN new_id;
END;
$$;
GRANT EXECUTE ON FUNCTION insert_publisher_action_audit(UUID, UUID, UUID, TEXT, TEXT, TEXT, TEXT, UUID, UUID, TEXT, TEXT, TEXT, TEXT, JSONB) TO callloom_sync;

-- API evaluator (every 15 min): which tenants have something to do. RLS hides workflow_rules and
-- publisher_pauses from callloom_api until app.current_tenant is set, so the scheduler needs this
-- one cross-tenant read. It returns tenant ids only; all per-tenant work then runs under withTenant.
CREATE OR REPLACE FUNCTION list_workflow_tenants()
RETURNS TABLE (tenant_id UUID)
SECURITY DEFINER
SET search_path = public
LANGUAGE sql
AS $$
    SELECT r.tenant_id FROM workflow_rules r WHERE r.is_active AND r.deleted_at IS NULL
    UNION
    SELECT p.tenant_id FROM publisher_pauses p
     WHERE p.unpaused_at IS NULL AND p.unpause_at IS NOT NULL AND p.unpause_at <= NOW();
$$;
GRANT EXECUTE ON FUNCTION list_workflow_tenants() TO callloom_api;
