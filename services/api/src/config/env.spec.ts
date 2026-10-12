import { assertProductionSecrets, type EnvConfig } from './env';

const GOOD_KEY = 'ab'.repeat(32);

function goodEnv(): EnvConfig {
  return {
    APP_ENV: 'production',
    API_PORT: 3000,
    DATABASE_URL: 'postgres://callloom_api:prodapi@postgres:5432/callloom',
    JWT_SECRET: 'p'.repeat(32),
    CLICKHOUSE_URL: 'http://default:prodch@clickhouse:8123',
    CLICKHOUSE_DB: 'default',
    MOCK_MODE: true,
    MOCK_EMITTER_ENABLED: false,
    DEMO_MODE_ENABLED: true,
    SINGLE_TENANT_MODE: false,
    REDIS_URL: 'redis://:prodredis@redis:6379',
    PROVIDERCONFIG_URL: 'http://providerconfig:8084',
    ASK_AI_URL: 'http://ask-ai:8086',
    KAFKA_BROKER: 'redpanda:9092',
    KAFKA_TOPIC_PINGS: 'callloom.pings.v1',
    KAFKA_TOPIC_CALL_SCORED: 'callloom.calls.call_scored.v1',
    KAFKA_TOPIC_ATTRIBUTION_ATTRIBUTED: 'callloom.attribution.attributed.v1',
    KAFKA_TOPIC_COMPLIANCE_VIOLATION: 'callloom.compliance.violation_detected.v1',
    KAFKA_TOPIC_CALL_TRANSCRIPT_READY: 'callloom.calls.call_transcript_ready.v1',
    DEEPGRAM_CALLBACK_SECRET: 'prod-deepgram',
    ADSPEND_SOURCE: 'mock',
    NOTIFICATION_CHANNEL: 'mock',
    CREDENTIAL_ENCRYPTION_KEY: GOOD_KEY,
    PLATFORM_SYNC_URL: 'http://platform-sync:8088',
    WRITEBACK_INTERNAL_TOKEN: 'prod-writeback',
    WEB_ORIGINS: 'https://calyx.ridgerisemedia.com',
    COOKIE_DOMAIN: '',
    TRUSTED_PROXIES: '',
    SESSION_COOKIE_NAME: 'calyx_session',
    CSRF_COOKIE_NAME: 'calyx_csrf',
  };
}

const PROCESS_GOOD: Record<string, string> = {
  POSTGRES_PASSWORD: 'prod-pg',
  POSTGRES_API_PASSWORD: 'prod-api',
  POSTGRES_INGESTION_PASSWORD: 'prod-ingestion',
  POSTGRES_SYNC_PASSWORD: 'prod-sync',
  CLICKHOUSE_PASSWORD: 'prod-ch',
  ANI_ENCRYPTION_KEY: GOOD_KEY,
  REDIS_PASSWORD: 'prod-redis',
};

describe('assertProductionSecrets', () => {
  const previous: Record<string, string | undefined> = {};

  beforeEach(() => {
    for (const key of Object.keys(PROCESS_GOOD)) {
      previous[key] = process.env[key];
      process.env[key] = PROCESS_GOOD[key];
    }
  });

  afterEach(() => {
    for (const key of Object.keys(PROCESS_GOOD)) {
      if (previous[key] === undefined) delete process.env[key];
      else process.env[key] = previous[key];
    }
  });

  it('accepts a production env that has no dev placeholder', () => {
    expect(() => assertProductionSecrets(goodEnv())).not.toThrow();
  });

  it.each([
    ['JWT_SECRET', (env: EnvConfig) => ({ ...env, JWT_SECRET: 'callloom_dev_jwt_secret_min_32_chars_long' })],
    ['CREDENTIAL_ENCRYPTION_KEY', (env: EnvConfig) => ({ ...env, CREDENTIAL_ENCRYPTION_KEY: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef' })],
    ['WRITEBACK_INTERNAL_TOKEN', (env: EnvConfig) => ({ ...env, WRITEBACK_INTERNAL_TOKEN: 'dev_writeback_internal_token_change_me' })],
    ['DATABASE_URL', (env: EnvConfig) => ({ ...env, DATABASE_URL: 'postgres://callloom_api:api_dev@postgres:5432/callloom' })],
    ['CLICKHOUSE_URL', (env: EnvConfig) => ({ ...env, CLICKHOUSE_URL: 'http://default:callloom_dev@clickhouse:8123' })],
    ['DEEPGRAM_CALLBACK_SECRET', (env: EnvConfig) => ({ ...env, DEEPGRAM_CALLBACK_SECRET: 'dev_deepgram_callback_secret_change_me' })],
    ['REDIS_URL', (env: EnvConfig) => ({ ...env, REDIS_URL: 'redis://:redis_dev@redis:6379' })],
  ])('names %s when that value is still the dev placeholder', (name, mutate) => {
    expect(() => assertProductionSecrets(mutate(goodEnv()))).toThrow(name);
  });

  it.each([
    'POSTGRES_PASSWORD',
    'POSTGRES_API_PASSWORD',
    'POSTGRES_INGESTION_PASSWORD',
    'POSTGRES_SYNC_PASSWORD',
    'CLICKHOUSE_PASSWORD',
    'ANI_ENCRYPTION_KEY',
    'REDIS_PASSWORD',
  ])('names %s when that process env is still the dev placeholder', (name) => {
    const placeholders: Record<string, string> = {
      POSTGRES_PASSWORD: 'callloom_dev',
      POSTGRES_API_PASSWORD: 'api_dev',
      POSTGRES_INGESTION_PASSWORD: 'ingestion_dev',
      POSTGRES_SYNC_PASSWORD: 'sync_dev',
      CLICKHOUSE_PASSWORD: 'callloom_dev',
      ANI_ENCRYPTION_KEY: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
      REDIS_PASSWORD: 'redis_dev',
    };
    process.env[name] = placeholders[name];
    expect(() => assertProductionSecrets(goodEnv())).toThrow(name);
  });
});
