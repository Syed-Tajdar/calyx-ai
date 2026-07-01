INSERT INTO alert_rules (tenant_id, name, condition, threshold, channels, actions)
SELECT
    '11111111-1111-1111-1111-111111111111'::uuid,
    'CPA spike alert',
    '{"metric":"cpa","operator":"gt","window_minutes":60}'::jsonb,
    45.0000,
    '{email}',
    '[{"type":"notify","channel":"email"}]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM alert_rules
    WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
      AND name = 'CPA spike alert'
);

INSERT INTO alert_rules (tenant_id, name, condition, threshold, channels, actions)
SELECT
    '11111111-1111-1111-1111-111111111111'::uuid,
    'Compliance risk threshold',
    '{"metric":"compliance_risk","operator":"gte","window_minutes":30}'::jsonb,
    70.0000,
    '{slack}',
    '[{"type":"notify","channel":"slack"}]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM alert_rules
    WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
      AND name = 'Compliance risk threshold'
);

INSERT INTO alert_rules (tenant_id, name, condition, threshold, channels, actions)
SELECT
    '11111111-1111-1111-1111-111111111111'::uuid,
    'Low conversion rate',
    '{"metric":"conversion_rate","operator":"lt","window_minutes":120}'::jsonb,
    0.1500,
    '{sms}',
    '[{"type":"notify","channel":"sms"}]'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM alert_rules
    WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
      AND name = 'Low conversion rate'
);
