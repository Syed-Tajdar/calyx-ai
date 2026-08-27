-- enrichment_attrs → ReplacingMergeTree(created_at)
-- Newest created_at wins per (tenant_id, ani_hash, attribute_key).
-- Safe to re-run when live table is already ReplacingMergeTree (no-op path via IF).

DROP TABLE IF EXISTS enrichment_attrs_rmt;

CREATE TABLE enrichment_attrs_rmt
(
    `tenant_id` UUID,
    `ani_hash` FixedString(64),
    `attribute_key` String,
    `attribute_value` String,
    `provider` LowCardinality(String) DEFAULT 'mock',
    `created_at` DateTime('UTC')
)
ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, ani_hash, attribute_key)
TTL created_at + toIntervalMonth(12)
SETTINGS index_granularity = 8192;

INSERT INTO enrichment_attrs_rmt
SELECT tenant_id, ani_hash, attribute_key, attribute_value, provider, created_at
FROM enrichment_attrs;

DROP TABLE IF EXISTS enrichment_attrs_mt_old;
RENAME TABLE
    enrichment_attrs TO enrichment_attrs_mt_old,
    enrichment_attrs_rmt TO enrichment_attrs;

-- Collapse duplicates now (background merges continue afterward).
OPTIMIZE TABLE enrichment_attrs FINAL;
