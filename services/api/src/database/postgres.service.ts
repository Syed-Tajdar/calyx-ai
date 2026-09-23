import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { execFileSync } from 'child_process';
import { Pool, PoolClient } from 'pg';
import { loadEnv } from '../config/env';

/** Application query budget — real dashboards/auth must outlive brief GC or join work. */
const APP_CONNECTION_TIMEOUT_MS = 10_000;
const APP_QUERY_TIMEOUT_MS = 30_000;

/** Readiness-only budget — fail fast so Kubernetes can react without pinning the app pool. */
const READY_CONNECTION_TIMEOUT_MS = 1_500;
const READY_QUERY_TIMEOUT_MS = 1_500;

/**
 * Resolve hostname → IPv4 once at boot so `/readyz` pings skip Node's DNS threadpool
 * (KafkaJS reconnect storms otherwise starve getaddrinfo for postgres/clickhouse).
 */
function connectionStringWithResolvedHost(connectionString: string): string {
  const u = new URL(connectionString);
  u.hostname = resolveIpv4(u.hostname);
  return u.toString();
}

function resolveIpv4(hostname: string): string {
  try {
    const out = execFileSync('getent', ['hosts', hostname], {
      encoding: 'utf8',
      timeout: 3_000,
    });
    const ip = out.trim().split(/\s+/)[0];
    if (ip && /^\d+\.\d+\.\d+\.\d+$/.test(ip)) {
      return ip;
    }
  } catch {
    // Fall back to hostname; UV_THREADPOOL_SIZE still helps under DNS load.
  }
  return hostname;
}

@Injectable()
export class PostgresService implements OnModuleDestroy {
  /** Shared by application queries (auth, scorecards, integrations, …). */
  private readonly pool: Pool;
  /** Dedicated to `/readyz` so short timeouts never apply to tenant traffic. */
  private readonly readyPool: Pool;

  constructor() {
    const env = loadEnv();
    this.pool = new Pool({
      connectionString: env.DATABASE_URL,
      connectionTimeoutMillis: APP_CONNECTION_TIMEOUT_MS,
      query_timeout: APP_QUERY_TIMEOUT_MS,
      statement_timeout: APP_QUERY_TIMEOUT_MS,
      max: 10,
    });
    this.readyPool = new Pool({
      connectionString: connectionStringWithResolvedHost(env.DATABASE_URL),
      connectionTimeoutMillis: READY_CONNECTION_TIMEOUT_MS,
      query_timeout: READY_QUERY_TIMEOUT_MS,
      statement_timeout: READY_QUERY_TIMEOUT_MS,
      max: 2,
    });
  }

  getPool(): Pool {
    return this.pool;
  }

  async withTenant<T>(
    tenantId: string,
    fn: (client: PoolClient) => Promise<T>,
  ): Promise<T> {
    const client = await this.pool.connect();
    try {
      await client.query('BEGIN');
      await client.query(`SELECT set_config('app.current_tenant', $1, true)`, [
        tenantId,
      ]);
      const result = await fn(client);
      await client.query('COMMIT');
      return result;
    } catch (error: unknown) {
      await client.query('ROLLBACK');
      throw error;
    } finally {
      client.release();
    }
  }

  /** Used only by `/readyz` — short timeouts on a dedicated IP-pinned pool. */
  async ping(): Promise<void> {
    await this.readyPool.query('SELECT 1');
  }

  async onModuleDestroy(): Promise<void> {
    await Promise.all([this.pool.end(), this.readyPool.end()]);
  }
}
