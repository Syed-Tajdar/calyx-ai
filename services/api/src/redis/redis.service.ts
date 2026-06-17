import { Global, Injectable, OnModuleDestroy } from '@nestjs/common';
import { createClient, RedisClientType } from 'redis';
import { loadEnv } from '../config/env';

@Injectable()
export class RedisService implements OnModuleDestroy {
  private client: RedisClientType | null = null;

  async getClient(): Promise<RedisClientType> {
    if (!this.client) {
      const env = loadEnv();
      this.client = createClient({ url: env.REDIS_URL });
      this.client.on('error', () => {
        // connection errors surfaced on command
      });
      await this.client.connect();
    }
    return this.client;
  }

  async del(key: string): Promise<void> {
    const client = await this.getClient();
    await client.del(key);
  }

  async onModuleDestroy(): Promise<void> {
    if (this.client) {
      await this.client.quit();
    }
  }
}
