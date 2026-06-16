-- Phase 2 analytics tables

CREATE TABLE IF NOT EXISTS call_scores (
    tenant_id       UUID,
    call_id         String,
    platform        LowCardinality(String) DEFAULT '',
    overall_score   Float32 DEFAULT 0,
    sentiment       LowCardinality(String) DEFAULT 'neutral',
    scorecard_id    String DEFAULT '',
    rules_passed    UInt16 DEFAULT 0,
    rules_failed    UInt16 DEFAULT 0,
    created_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, call_id, created_at)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS transcripts (
    tenant_id       UUID,
    call_id         String,
    transcript_text String,
    duration_seconds Float32 DEFAULT 0,
    word_count      UInt32 DEFAULT 0,
    created_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, call_id, created_at)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS financial_events (
    tenant_id       UUID,
    call_id         String,
    campaign_id     String DEFAULT '',
    event_type      LowCardinality(String) DEFAULT 'conversion',
    revenue         Decimal64(4) DEFAULT 0,
    payout          Decimal64(4) DEFAULT 0,
    cost            Decimal64(4) DEFAULT 0,
    created_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, campaign_id, created_at)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS enrichment_attrs (
    tenant_id       UUID,
    ani_hash        FixedString(64),
    attribute_key   String,
    attribute_value String,
    provider        LowCardinality(String) DEFAULT 'mock',
    created_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, ani_hash, attribute_key)
TTL created_at + INTERVAL 12 MONTH;

CREATE TABLE IF NOT EXISTS scoring_dlq (
    tenant_id       UUID,
    call_id         String,
    event_type      LowCardinality(String),
    error_message   String,
    payload_hash    FixedString(64),
    failed_at       DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(failed_at)
ORDER BY (tenant_id, failed_at, call_id);

CREATE TABLE IF NOT EXISTS enrichment_dlq (
    tenant_id       UUID,
    ani_hash        FixedString(64),
    event_type      LowCardinality(String),
    error_message   String,
    payload_hash    FixedString(64),
    failed_at       DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(failed_at)
ORDER BY (tenant_id, failed_at, ani_hash);
