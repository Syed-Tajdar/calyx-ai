INSERT INTO scorecards (tenant_id, name, vertical, rules, is_active)
VALUES
  (
    '11111111-1111-1111-1111-111111111111',
    'Medicare Inbound QA',
    'medicare',
    '[
      {"id": "greeting", "weight": 0.2, "question": "Did the agent provide a compliant greeting?"},
      {"id": "disclosure", "weight": 0.3, "question": "Were required disclosures read clearly?"},
      {"id": "intent", "weight": 0.5, "question": "Was caller intent qualified correctly?"}
    ]'::jsonb,
    TRUE
  ),
  (
    '11111111-1111-1111-1111-111111111111',
    'Final Expense QA',
    'insurance',
    '[
      {"id": "tcpa", "weight": 0.4, "question": "TCPA consent captured before transfer?"},
      {"id": "product", "weight": 0.3, "question": "Correct product positioning?"},
      {"id": "close", "weight": 0.3, "question": "Appropriate close attempt?"}
    ]'::jsonb,
    TRUE
  )
ON CONFLICT (tenant_id, name) DO NOTHING;
