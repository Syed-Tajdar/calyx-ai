import { z } from 'zod';

const boolFromEnv = z
  .union([z.string(), z.boolean()])
  .optional()
  .transform((v) => {
    if (v === undefined) return true;
    if (typeof v === 'boolean') return v;
    return v !== 'false' && v !== '0';
  });

/** Compose passes unset vars as "" (`${VAR:-}`): treat "" as unset before coercing to a number. */
const optionalInt = (min: number) =>
  z.preprocess((v) => (v === '' ? undefined : v), z.coerce.number().int().min(min).optional());

const envSchema = z.object({
  APP_ENV: z.enum(['development', 'production']).default('development'),
  API_PORT: z.coerce.number().default(3000),
  DATABASE_URL: z.string().min(1),
  JWT_SECRET: z.string().min(32),
  CLICKHOUSE_URL: z.string().default('http://localhost:8123'),
  CLICKHOUSE_DB: z.string().default('default'),
  MOCK_MODE: boolFromEnv,
  /** When false, API skips synthetic /live pings and uses Kafka instead (even if MOCK_MODE=true). */
  MOCK_EMITTER_ENABLED: boolFromEnv,
  /** Local default true. Production (APP_ENV=production) always treats this as false. */
  DEMO_MODE_ENABLED: boolFromEnv,
  SINGLE_TENANT_MODE: boolFromEnv,
  REDIS_URL: z.string().default('redis://localhost:6379'),
  PROVIDER_CATALOG_PATH: z.string().optional(),
  PROVIDERCONFIG_URL: z.string().default('http://localhost:8084'),
  ASK_AI_URL: z.string().default('http://localhost:8086'),
  KAFKA_BROKER: z.string().default('localhost:9092'),
  KAFKA_TOPIC_PINGS: z.string().default('callloom.pings.v1'),
  KAFKA_TOPIC_CALL_SCORED: z.string().default('callloom.calls.call_scored.v1'),
  KAFKA_TOPIC_ATTRIBUTION_ATTRIBUTED: z
    .string()
    .default('callloom.attribution.attributed.v1'),
  KAFKA_TOPIC_COMPLIANCE_VIOLATION: z
    .string()
    .default('callloom.compliance.violation_detected.v1'),
  KAFKA_TOPIC_CALL_TRANSCRIPT_READY: z
    .string()
    .default('callloom.calls.call_transcript_ready.v1'),
  DEEPGRAM_CALLBACK_SECRET: z.string().optional(),
  ADSPEND_SOURCE: z.string().default('mock'),
  NOTIFICATION_CHANNEL: z.string().default('mock'),
  CREDENTIAL_ENCRYPTION_KEY: z
    .string()
    .length(64)
    .default('0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'),
  PLATFORM_SYNC_URL: z.string().default('http://localhost:8088'),
  WRITEBACK_INTERNAL_TOKEN: z.string().optional(),
  /** Comma-separated browser origins allowed for CORS + CSRF Origin checks. */
  WEB_ORIGINS: z
    .string()
    .default('http://localhost:3101,http://127.0.0.1:3101'),
  /** Empty in local http; set to parent domain in production (e.g. `.example.com`). */
  COOKIE_DOMAIN: z.string().default(''),
  /**
   * Comma-separated CIDRs / IPs of trusted reverse proxies.
   * X-Forwarded-For is honored only when the immediate peer is in this list.
   * Empty = never trust XFF (use socket remoteAddress).
   */
  TRUSTED_PROXIES: z.string().default(''),
  /** v2 Phase 2A: Ask AI on numbers. Off unless exactly "true". */
  FEATURE_ASK_AI: z.string().optional(),
  OPENAI_API_KEY: z.string().optional(),
  /** Default gpt-5-mini (applied in the Ask AI service). */
  OPENAI_MODEL: z.string().optional(),
  /** Default 300000 tokens per tenant per Eastern day (applied in the Ask AI service). */
  ASK_AI_TOKEN_BUDGET_DAILY: optionalInt(1),
  /** v2 Phase 2D: Workflows. Off unless exactly "true" (route gate + evaluator). */
  FEATURE_WORKFLOWS: z.string().optional(),
  /** Kill switch for publisher pause/unpause. Off unless exactly "true". */
  WORKFLOW_ACTIONS_ENABLED: z.string().optional(),
  /** Ringba pause proven on the sandbox publisher. Off unless exactly "true". */
  WORKFLOW_RINGBA_PAUSE_VERIFIED: z.string().optional(),
  /** Default 3 automatic pauses per tenant per Eastern day (applied in the workflows module). */
  WORKFLOW_MAX_AUTO_PAUSES_DAILY: optionalInt(0),
  /** Alert email. Off while SMTP_HOST is empty. */
  SMTP_HOST: z.string().optional(),
  SMTP_PORT: optionalInt(1),
  SMTP_USER: z.string().optional(),
  SMTP_PASSWORD: z.string().optional(),
  SMTP_FROM: z.string().optional(),
  ALERT_EMAIL_TO: z.string().optional(),
  SESSION_COOKIE_NAME: z.string().default('calyx_session'),
  CSRF_COOKIE_NAME: z.string().default('calyx_csrf'),
});

export type EnvConfig = z.infer<typeof envSchema>;

const DEV_JWT = 'callloom_dev_jwt_secret_min_32_chars_long';
const DEV_CRED_KEY =
  '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
const DEV_WRITEBACK = 'dev_writeback_internal_token_change_me';
const DEV_DEEPGRAM = 'dev_deepgram_callback_secret_change_me';
const DEV_REDIS = 'redis_dev';

function named(bad: string[], name: string, value: string | undefined, placeholders: string[]): void {
  if (!value || placeholders.some((item) => value === item || value.includes(item))) {
    bad.push(name);
  }
}

function redisUrlIsDev(value: string | undefined): boolean {
  if (!value) return true;
  try {
    const url = new URL(value);
    return !url.password || url.password === DEV_REDIS;
  } catch {
    return true;
  }
}

/**
 * Refuse production boot when a B1 secret is missing or still a known local placeholder.
 * This does not read the users table and does not change the app login.
 */
export function assertProductionSecrets(env: EnvConfig): void {
  if (env.APP_ENV !== 'production') return;
  const bad: string[] = [];
  if (!env.JWT_SECRET || env.JWT_SECRET === DEV_JWT || env.JWT_SECRET.length < 32) {
    bad.push('JWT_SECRET');
  }
  named(bad, 'CREDENTIAL_ENCRYPTION_KEY', env.CREDENTIAL_ENCRYPTION_KEY, [DEV_CRED_KEY]);
  named(bad, 'WRITEBACK_INTERNAL_TOKEN', env.WRITEBACK_INTERNAL_TOKEN, [DEV_WRITEBACK]);
  named(bad, 'DATABASE_URL', env.DATABASE_URL, ['api_dev', 'ingestion_dev', 'sync_dev', 'callloom_dev']);
  named(bad, 'CLICKHOUSE_URL', env.CLICKHOUSE_URL, ['callloom_dev']);
  named(bad, 'DEEPGRAM_CALLBACK_SECRET', env.DEEPGRAM_CALLBACK_SECRET, [DEV_DEEPGRAM]);
  if (redisUrlIsDev(env.REDIS_URL)) bad.push('REDIS_URL');
  named(bad, 'POSTGRES_PASSWORD', process.env.POSTGRES_PASSWORD, ['callloom_dev']);
  named(bad, 'CLICKHOUSE_PASSWORD', process.env.CLICKHOUSE_PASSWORD, ['callloom_dev']);
  named(bad, 'ANI_ENCRYPTION_KEY', process.env.ANI_ENCRYPTION_KEY, [DEV_CRED_KEY]);
  named(bad, 'POSTGRES_API_PASSWORD', process.env.POSTGRES_API_PASSWORD, ['api_dev']);
  named(bad, 'POSTGRES_INGESTION_PASSWORD', process.env.POSTGRES_INGESTION_PASSWORD, ['ingestion_dev']);
  named(bad, 'POSTGRES_SYNC_PASSWORD', process.env.POSTGRES_SYNC_PASSWORD, ['sync_dev']);
  named(bad, 'REDIS_PASSWORD', process.env.REDIS_PASSWORD, [DEV_REDIS]);
  if (env.FEATURE_ASK_AI === 'true') {
    named(bad, 'OPENAI_API_KEY', env.OPENAI_API_KEY, ['sk-dev', 'changeme', 'change_me', 'placeholder']);
  }
  if (env.SMTP_HOST && env.SMTP_HOST.trim()) {
    named(bad, 'SMTP_PASSWORD', env.SMTP_PASSWORD, ['changeme', 'change_me', 'placeholder', 'password', 'smtp_dev']);
  }
  if (bad.length) {
    throw new Error(
      `APP_ENV=production refuses to start with placeholder/missing secrets: ${bad.join(', ')}`,
    );
  }
}

export function loadEnv(): EnvConfig {
  const result = envSchema.safeParse(process.env);
  if (!result.success) {
    throw new Error(`Invalid environment: ${result.error.message}`);
  }

  const env = result.data;
  assertProductionSecrets(env);
  const mockMode = env.MOCK_MODE ?? true;
  if (!mockMode) {
    if (!env.DEEPGRAM_CALLBACK_SECRET) {
      throw new Error(
        'DEEPGRAM_CALLBACK_SECRET required when MOCK_MODE=false',
      );
    }
  }
  return env;
}

export function parseWebOrigins(env: EnvConfig = loadEnv()): string[] {
  return env.WEB_ORIGINS.split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}
