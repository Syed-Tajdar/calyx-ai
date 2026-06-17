-- Phase 3 TCPA compliance analytics tables

CREATE TABLE IF NOT EXISTS compliance_risk_scores (
    tenant_id           UUID,
    call_id             String,
    publisher_id        String DEFAULT '',
    state               LowCardinality(String) DEFAULT '',
    risk_score          Float32 DEFAULT 0,
    dnc_match           UInt8 DEFAULT 0,
    attestation_level   LowCardinality(String) DEFAULT 'unknown',
    exposure_usd        Float32 DEFAULT 0,
    created_at          DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, call_id, created_at)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS dnc_match_events (
    tenant_id       UUID,
    call_id         String,
    ani_hash        FixedString(64),
    publisher_id    String DEFAULT '',
    list_name       LowCardinality(String) DEFAULT 'mock',
    matched         UInt8 DEFAULT 0,
    created_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, ani_hash, created_at)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS attestation_events (
    tenant_id           UUID,
    call_id             String,
    attestation_level   LowCardinality(String) DEFAULT 'unknown',
    carrier_block_est   Float32 DEFAULT 0,
    created_at          DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, call_id, created_at)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS compliance_violations (
    tenant_id       UUID,
    call_id         String,
    violation_type  LowCardinality(String),
    risk_score      Float32 DEFAULT 0,
    exposure_usd    Float32 DEFAULT 0,
    created_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, created_at, call_id)
TTL created_at + INTERVAL 24 MONTH;

CREATE TABLE IF NOT EXISTS compliance_dlq (
    tenant_id       UUID,
    call_id         String,
    event_type      LowCardinality(String),
    error_message   String,
    payload_hash    FixedString(64),
    failed_at       DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(failed_at)
ORDER BY (tenant_id, failed_at, call_id);
