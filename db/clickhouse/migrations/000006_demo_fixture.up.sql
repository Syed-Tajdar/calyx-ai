-- Demo fixture data for cold-start presentation (demo tenant only).
-- Idempotent: backfills when demo-call-1 row is missing (safe after smoke tests).

INSERT INTO pings (
    tenant_id, platform, external_call_id, ani_hash,
    campaign_id, publisher_id, target_id, state,
    event_type, revenue, payout, is_duplicate, received_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    multiIf(number % 10 < 5, 'ringba', number % 10 < 7, 'retreaver', number % 10 < 8, 'trackdrive', number % 10 < 9, 'callgrid', 'callloom-native'),
    concat('demo-call-', toString(number + 1)),
    lower(hex(SHA256(concat('demo-ani-', toString(number))))),
    multiIf(number % 3 = 0, 'camp_google', number % 3 = 1, 'camp_bing', 'camp_meta'),
    multiIf(number % 4 = 0, 'MediaCo', number % 4 = 1, 'LeadStream', number % 4 = 2, 'DialPro', 'CallStream'),
    concat('tgt_', toString((number % 5) + 1)),
    multiIf(number % 4 = 0, 'TX', number % 4 = 1, 'CA', number % 4 = 2, 'FL', 'NY'),
    'incoming',
    toDecimal64(8.5 + (number % 12) * 1.25, 4),
    toDecimal64(5.0 + (number % 10) * 0.85, 4),
    toUInt8(number % 17 = 0),
    now() - INTERVAL (number % 7) DAY - INTERVAL (number % 12) HOUR
FROM numbers(50)
WHERE NOT EXISTS (
    SELECT 1 FROM pings
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND external_call_id = 'demo-call-1'
);

INSERT INTO call_scores (
    tenant_id, call_id, platform, overall_score, sentiment,
    scorecard_id, rules_passed, rules_failed, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    multiIf(number % 10 < 5, 'ringba', number % 10 < 7, 'retreaver', number % 10 < 8, 'trackdrive', number % 10 < 9, 'callgrid', 'callloom-native'),
    62 + (number % 33),
    multiIf(number % 3 = 0, 'positive', number % 3 = 1, 'neutral', 'negative'),
    if(number % 2 = 0, 'medicare-qa-v1', 'final-expense-v1'),
    toUInt16(4 + (number % 3)),
    toUInt16(number % 2),
    now() - INTERVAL (number % 7) DAY - INTERVAL (number % 12) HOUR
FROM numbers(30)
WHERE NOT EXISTS (
    SELECT 1 FROM call_scores
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-call-1'
);

INSERT INTO transcripts (
    tenant_id, call_id, transcript_text, duration_seconds, word_count, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    concat('Agent greeted caller. Verified intent for call ', toString(number + 1), '. Compliance disclaimer read.'),
    120 + (number % 180),
    180 + (number % 120),
    now() - INTERVAL (number % 7) DAY - INTERVAL (number % 12) HOUR
FROM numbers(10)
WHERE NOT EXISTS (
    SELECT 1 FROM transcripts
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-call-1'
);

INSERT INTO compliance_risk_scores (
    tenant_id, call_id, publisher_id, state, risk_score,
    dnc_match, attestation_level, exposure_usd, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    concat('pub_', toString((number % 8) + 1)),
    multiIf(number % 4 = 0, 'TX', number % 4 = 1, 'CA', number % 4 = 2, 'FL', 'NY'),
    18 + (number % 55),
    toUInt8(number % 11 = 0),
    multiIf(number % 3 = 0, 'A', number % 3 = 1, 'B', 'C'),
    5.5 + (number % 20) * 1.2,
    now() - INTERVAL (number * 4) HOUR
FROM numbers(12)
WHERE NOT EXISTS (
    SELECT 1 FROM compliance_risk_scores
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-call-1'
);

INSERT INTO compliance_violations (
    tenant_id, call_id, violation_type, risk_score, exposure_usd, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    multiIf(number % 4 = 0, 'calling_hours', number % 4 = 1, 'dnc_list', number % 4 = 2, 'consent_missing', 'attestation_low'),
    35 + (number % 40),
    8 + (number % 15) * 2.5,
    now() - INTERVAL (number * 5) HOUR
FROM numbers(10)
WHERE NOT EXISTS (
    SELECT 1 FROM compliance_violations
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-call-1'
);

INSERT INTO attribution_fired_events (
    tenant_id, call_id, event_id, platform, click_id,
    revenue, status, http_status, error_message, fired_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    concat('attr-evt-', toString(number + 1)),
    multiIf(number % 3 = 0, 'meta', number % 3 = 1, 'google', 'bing'),
    concat('clk_', toString(number + 1000)),
    toDecimal64(12 + (number % 8) * 2.5, 4),
    if(number % 7 = 0, 'failed', 'fired'),
    if(number % 7 = 0, toUInt16(500), toUInt16(200)),
    if(number % 7 = 0, 'timeout', ''),
    now() - INTERVAL (number * 4) HOUR
FROM numbers(15)
WHERE NOT EXISTS (
    SELECT 1 FROM attribution_fired_events
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-call-1'
);

INSERT INTO enrichment_attrs (
    tenant_id, ani_hash, attribute_key, attribute_value, provider, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    lower(hex(SHA256(concat('demo-ani-', toString(ani_idx))))),
    concat('attr_', leftPad(toString(attr_n), 3, '0')),
    concat('value_', toString(attr_n)),
    'enrollhere',
    now() - INTERVAL 1 DAY
FROM
(
    SELECT number AS ani_idx FROM numbers(3)
) AS ani
CROSS JOIN
(
    SELECT number AS attr_n FROM numbers(20)
) AS attrs
WHERE NOT EXISTS (
    SELECT 1 FROM enrichment_attrs
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND ani_hash = lower(hex(SHA256('demo-ani-0')))
);

-- Call Loom native direct source demo rows (separate idempotent block).
INSERT INTO pings (
    tenant_id, platform, external_call_id, ani_hash,
    campaign_id, publisher_id, target_id, state,
    event_type, revenue, payout, is_duplicate, received_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    'callloom-native',
    concat('demo-callloom-', toString(number + 1)),
    lower(hex(SHA256(concat('demo-callloom-ani-', toString(number))))),
    'camp_callloom_native',
    concat('pub_callloom_', toString(number + 1)),
    concat('tgt_callloom_', toString(number + 1)),
    multiIf(number = 0, 'TX', number = 1, 'CA', 'FL'),
    'incoming',
    toDecimal64(14.0 + (number * 1.5), 4),
    toDecimal64(9.5 + (number * 0.75), 4),
    toUInt8(0),
    now() - INTERVAL ((number + 1) * 2) HOUR
FROM numbers(3)
WHERE NOT EXISTS (
    SELECT 1 FROM pings
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND external_call_id = 'demo-callloom-1'
);

INSERT INTO call_scores (
    tenant_id, call_id, platform, overall_score, sentiment,
    scorecard_id, rules_passed, rules_failed, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-callloom-', toString(number + 1)),
    'callloom-native',
    75 + (number * 6),
    multiIf(number = 0, 'positive', number = 1, 'neutral', 'positive'),
    'callloom-native-v1',
    toUInt16(5),
    toUInt16(number % 2),
    now() - INTERVAL ((number + 1) * 2) HOUR
FROM numbers(3)
WHERE NOT EXISTS (
    SELECT 1 FROM call_scores
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-callloom-1'
);

INSERT INTO transcripts (
    tenant_id, call_id, transcript_text, duration_seconds, word_count, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-callloom-', toString(number + 1)),
    concat(
        'Call Loom native direct source. Agent verified caller intent for demo-callloom-',
        toString(number + 1),
        '. Compliance disclaimer read via native API.'
    ),
    140 + (number * 15),
    200 + (number * 10),
    now() - INTERVAL ((number + 1) * 2) HOUR
FROM numbers(3)
WHERE NOT EXISTS (
    SELECT 1 FROM transcripts
    WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')
      AND call_id = 'demo-callloom-1'
);
