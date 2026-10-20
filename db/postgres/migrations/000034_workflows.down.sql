DROP FUNCTION IF EXISTS list_workflow_tenants();
DROP FUNCTION IF EXISTS insert_publisher_action_audit(UUID, UUID, UUID, TEXT, TEXT, TEXT, TEXT, UUID, UUID, TEXT, TEXT, TEXT, TEXT, JSONB);
DROP FUNCTION IF EXISTS get_publisher_pause_integration(UUID, UUID);
DROP TABLE IF EXISTS publisher_action_audit;
DROP FUNCTION IF EXISTS publisher_action_audit_immutable();
DROP TABLE IF EXISTS publisher_pauses;
DROP TABLE IF EXISTS protected_publishers;
DROP TABLE IF EXISTS alert_events;
DROP TABLE IF EXISTS workflow_rules;
ALTER TABLE integrations DROP COLUMN IF EXISTS allow_publisher_pause;
DELETE FROM integration_settings_audit WHERE action IN ('allow_pause_on', 'allow_pause_off');
ALTER TABLE integration_settings_audit DROP CONSTRAINT IF EXISTS integration_settings_audit_action_check;
ALTER TABLE integration_settings_audit ADD CONSTRAINT integration_settings_audit_action_check
    CHECK (action IN ('pause', 'resume', 'delete'));
