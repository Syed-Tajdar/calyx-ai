-- Allow reusing scorecard names after soft-delete (is_active = FALSE).
ALTER TABLE scorecards DROP CONSTRAINT IF EXISTS scorecards_tenant_id_name_key;

CREATE UNIQUE INDEX idx_scorecards_tenant_name_active
  ON scorecards (tenant_id, name)
  WHERE is_active = TRUE;
