DELETE FROM alert_rules
WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
  AND name IN ('CPA spike alert', 'Compliance risk threshold', 'Low conversion rate');
