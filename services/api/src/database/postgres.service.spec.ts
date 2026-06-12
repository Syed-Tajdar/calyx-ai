import { PostgresService } from './postgres.service';

describe('PostgresService', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    process.env = {
      ...originalEnv,
      DATABASE_URL: 'postgres://callloom_api:api_dev@localhost:5432/callloom',
      JWT_SECRET: 'test_jwt_secret_minimum_32_chars_long',
      CLICKHOUSE_URL: 'http://localhost:8123',
    };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('exposes withTenant for RLS-scoped queries', () => {
    const service = new PostgresService();
    expect(typeof service.withTenant).toBe('function');
  });
});
