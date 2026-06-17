-- Ask AI semantic result cache (ClickHouse — batch-friendly analytics cache)

CREATE TABLE IF NOT EXISTS nl_query_results_cache (
    tenant_id       UUID,
    question_hash   FixedString(64),
    intent          LowCardinality(String) DEFAULT 'unknown',
    answer          String,
    row_count       UInt32 DEFAULT 0,
    hit_count       UInt32 DEFAULT 1,
    created_at      DateTime('UTC'),
    last_hit_at     DateTime('UTC')
) ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, question_hash, created_at)
TTL created_at + INTERVAL 6 MONTH;
