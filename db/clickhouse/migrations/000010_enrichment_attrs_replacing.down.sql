-- Rollback ReplacingMergeTree → MergeTree (keeps data from _mt_old if present, else from live).
DROP TABLE IF EXISTS enrichment_attrs_mt_restore;

CREATE TABLE enrichment_attrs_mt_restore
(
    `tenant_id` UUID,
    `ani_hash` FixedString(64),
    `attribute_key` String,
    `attribute_value` String,
    `provider` LowCardinality(String) DEFAULT 'mock',
    `created_at` DateTime('UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, ani_hash, attribute_key)
TTL created_at + toIntervalMonth(12)
SETTINGS index_granularity = 8192;

INSERT INTO enrichment_attrs_mt_restore
SELECT tenant_id, ani_hash, attribute_key, attribute_value, provider, created_at
FROM enrichment_attrs;

DROP TABLE IF EXISTS enrichment_attrs;
RENAME TABLE enrichment_attrs_mt_restore TO enrichment_attrs;
DROP TABLE IF EXISTS enrichment_attrs_mt_old;
DROP TABLE IF EXISTS enrichment_attrs_rmt;
