import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { Pool, PoolClient } from 'pg';
import { loadEnv } from '../config/env';

@Injectable()
export class PostgresService implements OnModuleDestroy {
  private readonly pool: Pool;

  constructor() {
    const env = loadEnv();
    this.pool = new Pool({
      connectionString: env.DATABASE_URL,
      connectionTimeoutMillis: 1_500,
      // Cancels stalled queries so Docker network flaps cannot pin pool clients.
      query_timeout: 1_500,
      statement_timeout: 1_500,
      max: 10,
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

  async ping(): Promise<void> {
    await this.pool.query('SELECT 1');
  }

  async onModuleDestroy(): Promise<void> {
    await this.pool.end();
  }
}
