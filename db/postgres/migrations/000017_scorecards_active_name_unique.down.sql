DROP INDEX IF EXISTS idx_scorecards_tenant_name_active;

ALTER TABLE scorecards
  ADD CONSTRAINT scorecards_tenant_id_name_key UNIQUE (tenant_id, name);
