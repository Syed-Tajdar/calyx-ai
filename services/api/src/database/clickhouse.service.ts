import { Injectable, OnModuleDestroy } from '@nestjs/common';
import { execFileSync } from 'child_process';
import { createClient, ClickHouseClient } from '@clickhouse/client';
import { loadEnv } from '../config/env';

function urlWithResolvedHost(url: string): string {
  const u = new URL(url);
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
export class ClickhouseService implements OnModuleDestroy {
  private readonly client: ClickHouseClient;
  /** Readiness-only client: host resolved at boot; short request timeout. */
  private readonly readyClient: ClickHouseClient;

  constructor() {
    const env = loadEnv();
    this.client = createClient({
      url: env.CLICKHOUSE_URL,
      database: env.CLICKHOUSE_DB,
    });
    this.readyClient = createClient({
      url: urlWithResolvedHost(env.CLICKHOUSE_URL),
      database: env.CLICKHOUSE_DB,
      request_timeout: 1_500,
    });
  }

  getClient(): ClickHouseClient {
    return this.client;
  }

  async ping(): Promise<void> {
    await this.readyClient.query({
      query: 'SELECT 1',
      clickhouse_settings: {},
    });
  }

  async onModuleDestroy(): Promise<void> {
    await Promise.all([this.client.close(), this.readyClient.close()]);
  }
}
