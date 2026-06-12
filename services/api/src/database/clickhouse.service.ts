import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { createClient, ClickHouseClient } from '@clickhouse/client';
import { loadEnv } from '../config/env';

@Injectable()
export class ClickhouseService implements OnModuleDestroy {
  private readonly client: ClickHouseClient;

  constructor() {
    const env = loadEnv();
    this.client = createClient({
      url: env.CLICKHOUSE_URL,
      database: env.CLICKHOUSE_DB,
    });
  }

  getClient(): ClickHouseClient {
    return this.client;
  }

  async ping(): Promise<void> {
    await this.client.ping();
  }

  async onModuleDestroy(): Promise<void> {
    await this.client.close();
  }
}
