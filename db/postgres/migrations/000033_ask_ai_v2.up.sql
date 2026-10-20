-- v2 Phase 2A — Ask AI on numbers (OpenAI, tool-calling inside the API).
-- Daily token usage (hard budget) and the answer history shown in the UI.
-- No prompt text is logged anywhere else; this table is the user-visible history.

CREATE TABLE IF NOT EXISTS ask_ai_usage_daily (
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    usage_date  DATE NOT NULL,           -- Eastern-time day
    tokens      BIGINT NOT NULL DEFAULT 0,
    questions   INTEGER NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, usage_date)
);

ALTER TABLE ask_ai_usage_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE ask_ai_usage_daily FORCE ROW LEVEL SECURITY;
CREATE POLICY ask_ai_usage_daily_isolation ON ask_ai_usage_daily
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE ON ask_ai_usage_daily TO callloom_api;

CREATE TABLE IF NOT EXISTS ask_ai_answers (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL REFERENCES tenants(id),
    user_id        UUID REFERENCES users(id),
    question       TEXT NOT NULL,
    answer         TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('answered', 'no_data', 'unverified', 'budget', 'error')),
    date_from      DATE,
    date_to        DATE,
    tool_calls     JSONB NOT NULL DEFAULT '[]'::jsonb,   -- [{name, args}]
    data           JSONB NOT NULL DEFAULT '[]'::jsonb,   -- tool results shown as "numbers used"
    tokens         INTEGER NOT NULL DEFAULT 0,
    model          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ask_ai_answers_tenant_created
    ON ask_ai_answers (tenant_id, created_at DESC);

ALTER TABLE ask_ai_answers ENABLE ROW LEVEL SECURITY;
ALTER TABLE ask_ai_answers FORCE ROW LEVEL SECURITY;
CREATE POLICY ask_ai_answers_isolation ON ask_ai_answers
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT ON ask_ai_answers TO callloom_api;
