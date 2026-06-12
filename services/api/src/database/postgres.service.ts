import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { Pool } from 'pg';
import { loadEnv } from '../config/env';

@Injectable()
export class PostgresService implements OnModuleDestroy {
  private readonly pool: Pool;

  constructor() {
    const env = loadEnv();
    this.pool = new Pool({ connectionString: env.DATABASE_URL });
  }

  getPool(): Pool {
    return this.pool;
  }

  async ping(): Promise<void> {
    await this.pool.query('SELECT 1');
  }

  async onModuleDestroy(): Promise<void> {
    await this.pool.end();
  }
}
