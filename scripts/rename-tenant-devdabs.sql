-- A5: rename the existing tenant. Do not insert a new row.
-- 1. Read-only: find the tenant the owner logs into.
SELECT id, name FROM tenants ORDER BY created_at;
-- 2. Change only the name, by id. Users, integrations and data stay.
UPDATE tenants
SET name = 'Devdabs', updated_at = NOW()
WHERE id = '11111111-1111-1111-1111-111111111111';
