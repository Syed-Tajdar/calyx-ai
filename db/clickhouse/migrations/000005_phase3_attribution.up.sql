-- Phase 3 ad attribution analytics tables

CREATE TABLE IF NOT EXISTS attribution_fired_events (
    tenant_id       UUID,
    call_id         String,
    event_id        String,
    platform        LowCardinality(String),
    click_id        String DEFAULT '',
    revenue         Decimal64(4) DEFAULT 0,
    status          LowCardinality(String) DEFAULT 'fired',
    http_status     UInt16 DEFAULT 0,
    error_message   String DEFAULT '',
    fired_at        DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(fired_at)
ORDER BY (tenant_id, platform, fired_at, call_id)
TTL fired_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS attribution_dlq (
    tenant_id       UUID,
    call_id         String,
    event_id        String,
    stage           LowCardinality(String),
    error_message   String,
    payload_hash    FixedString(64),
    failed_at       DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(failed_at)
ORDER BY (tenant_id, failed_at, call_id);
