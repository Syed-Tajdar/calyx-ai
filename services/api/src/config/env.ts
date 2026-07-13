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
});

export type EnvConfig = z.infer<typeof envSchema>;

export function loadEnv(): EnvConfig {
  const result = envSchema.safeParse(process.env);
  if (!result.success) {
    throw new Error(`Invalid environment: ${result.error.message}`);
  }

  const env = result.data;
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
