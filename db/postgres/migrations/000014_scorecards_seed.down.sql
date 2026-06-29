DELETE FROM scorecards
WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
  AND name IN ('Medicare Inbound QA', 'Final Expense QA');
