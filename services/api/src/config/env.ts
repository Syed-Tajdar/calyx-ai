import { z } from 'zod';

const boolFromEnv = z
  .union([z.string(), z.boolean()])
  .optional()
  .transform((v) => {
    if (v === undefined) return true;
    if (typeof v === 'boolean') return v;
    return v !== 'false' && v !== '0';
  });

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
    .default('http://localhost:3001,http://127.0.0.1:3001'),
  /** Empty in local http; set to parent domain in production (e.g. `.example.com`). */
  COOKIE_DOMAIN: z.string().default(''),
  /**
   * Comma-separated CIDRs / IPs of trusted reverse proxies.
   * X-Forwarded-For is honored only when the immediate peer is in this list.
   * Empty = never trust XFF (use socket remoteAddress).
   */
  TRUSTED_PROXIES: z.string().default(''),
  SESSION_COOKIE_NAME: z.string().default('calyx_session'),
  CSRF_COOKIE_NAME: z.string().default('calyx_csrf'),
});

export type EnvConfig = z.infer<typeof envSchema>;

const DEV_JWT = 'callloom_dev_jwt_secret_min_32_chars_long';
const DEV_CRED_KEY =
  '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
const DEV_WRITEBACK = 'dev_writeback_internal_token_change_me';

/** Refuse production boot when secrets are missing or still the known local placeholders. */
export function assertProductionSecrets(env: EnvConfig): void {
  if (env.APP_ENV !== 'production') return;
  const bad: string[] = [];
  if (!env.JWT_SECRET || env.JWT_SECRET === DEV_JWT || env.JWT_SECRET.length < 32) {
    bad.push('JWT_SECRET');
  }
  if (!env.CREDENTIAL_ENCRYPTION_KEY || env.CREDENTIAL_ENCRYPTION_KEY === DEV_CRED_KEY) {
    bad.push('CREDENTIAL_ENCRYPTION_KEY');
  }
  if (!env.WRITEBACK_INTERNAL_TOKEN || env.WRITEBACK_INTERNAL_TOKEN === DEV_WRITEBACK) {
    bad.push('WRITEBACK_INTERNAL_TOKEN');
  }
  if (
    !env.DATABASE_URL ||
    env.DATABASE_URL.includes('api_dev') ||
    env.DATABASE_URL.includes('callloom_dev')
  ) {
    bad.push('DATABASE_URL');
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
