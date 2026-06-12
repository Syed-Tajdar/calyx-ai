CREATE TABLE IF NOT EXISTS pings (
    tenant_id        UUID,
    platform         LowCardinality(String),
    external_call_id String,
    ani_hash         FixedString(64),
    campaign_id      String DEFAULT '',
    publisher_id     String DEFAULT '',
    target_id        String DEFAULT '',
    state            LowCardinality(String) DEFAULT '',
    event_type       LowCardinality(String) DEFAULT 'incoming',
    revenue          Decimal64(4) DEFAULT 0,
    payout           Decimal64(4) DEFAULT 0,
    is_duplicate     UInt8 DEFAULT 0,
    received_at      DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(received_at)
ORDER BY (tenant_id, platform, received_at, external_call_id)
TTL received_at + INTERVAL 24 MONTH;
