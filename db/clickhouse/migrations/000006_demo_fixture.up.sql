-- Demo fixture data for cold-start presentation (demo tenant only).
-- Idempotent: skips insert when demo pings already exist.

INSERT INTO pings (
    tenant_id, platform, external_call_id, ani_hash,
    campaign_id, publisher_id, target_id, state,
    event_type, revenue, payout, is_duplicate, received_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    multiIf(number % 10 < 7, 'ringba', number % 10 < 9, 'retreaver', 'mock'),
    concat('demo-call-', toString(number + 1)),
    lower(hex(SHA256(concat('demo-ani-', toString(number))))),
    multiIf(number % 3 = 0, 'camp_google', number % 3 = 1, 'camp_bing', 'camp_meta'),
    concat('pub_', toString((number % 8) + 1)),
    concat('tgt_', toString((number % 5) + 1)),
    multiIf(number % 4 = 0, 'TX', number % 4 = 1, 'CA', number % 4 = 2, 'FL', 'NY'),
    'incoming',
    toDecimal64(8.5 + (number % 12) * 1.25, 4),
    toDecimal64(5.0 + (number % 10) * 0.85, 4),
    toUInt8(number % 17 = 0),
    now() - INTERVAL (number * 3) HOUR
FROM numbers(50)
WHERE (SELECT count() FROM pings WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;

INSERT INTO call_scores (
    tenant_id, call_id, platform, overall_score, sentiment,
    scorecard_id, rules_passed, rules_failed, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    multiIf(number % 10 < 7, 'ringba', number % 10 < 9, 'retreaver', 'mock'),
    62 + (number % 33),
    multiIf(number % 3 = 0, 'positive', number % 3 = 1, 'neutral', 'negative'),
    if(number % 2 = 0, 'medicare-qa-v1', 'final-expense-v1'),
    toUInt16(4 + (number % 3)),
    toUInt16(number % 2),
    now() - INTERVAL (number * 3) HOUR
FROM numbers(30)
WHERE (SELECT count() FROM call_scores WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;

INSERT INTO transcripts (
    tenant_id, call_id, transcript_text, duration_seconds, word_count, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    concat('demo-call-', toString(number + 1)),
    concat('Agent greeted caller. Verified intent for call ', toString(number + 1), '. Compliance disclaimer read.'),
    120 + (number % 180),
    180 + (number % 120),
    now() - INTERVAL (number * 3) HOUR
FROM numbers(10)
WHERE (SELECT count() FROM transcripts WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;

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
WHERE (SELECT count() FROM compliance_risk_scores WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;

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
WHERE (SELECT count() FROM compliance_violations WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;

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
WHERE (SELECT count() FROM attribution_fired_events WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;

INSERT INTO enrichment_attrs (
    tenant_id, ani_hash, attribute_key, attribute_value, provider, created_at
)
SELECT
    toUUID('11111111-1111-1111-1111-111111111111'),
    lower(hex(SHA256(concat('demo-ani-', toString(ani_idx))))),
    concat('attr_', leftPad(toString(attr_n), 3, '0')),
    concat('value_', toString(attr_n)),
    'mock',
    now() - INTERVAL 1 DAY
FROM
(
    SELECT number AS ani_idx FROM numbers(3)
) AS ani
CROSS JOIN
(
    SELECT number AS attr_n FROM numbers(20)
) AS attrs
WHERE (SELECT count() FROM enrichment_attrs WHERE tenant_id = toUUID('11111111-1111-1111-1111-111111111111')) = 0;
