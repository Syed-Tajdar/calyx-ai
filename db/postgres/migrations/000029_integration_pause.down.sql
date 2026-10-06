DROP TABLE IF EXISTS integration_settings_audit;
ALTER TABLE integrations DROP COLUMN IF EXISTS deleted_at;
