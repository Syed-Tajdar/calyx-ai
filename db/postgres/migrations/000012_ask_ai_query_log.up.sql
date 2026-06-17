CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS ask_ai_query_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    question        TEXT NOT NULL,
    intent          VARCHAR(64) NOT NULL DEFAULT 'unknown',
    answer          TEXT NOT NULL,
    sql_text        TEXT,
    row_count       INTEGER NOT NULL DEFAULT 0,
    cache_hit       BOOLEAN NOT NULL DEFAULT FALSE,
    tokens_used     INTEGER NOT NULL DEFAULT 0,
    embedding       vector(8),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ask_ai_query_log_tenant_created
    ON ask_ai_query_log (tenant_id, created_at DESC);

ALTER TABLE ask_ai_query_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY ask_ai_query_log_tenant_isolation ON ask_ai_query_log
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON ask_ai_query_log TO callloom_api;
