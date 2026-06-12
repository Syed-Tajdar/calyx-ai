import { Injectable } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';

export interface IntegrationRecord {
  id: string;
  platform: string;
  name: string;
  webhook_token: string;
  is_active: boolean;
  created_at: string;
}

@Injectable()
export class IntegrationsService {
  constructor(private readonly postgres: PostgresService) {}

  async list(tenantId: string): Promise<IntegrationRecord[]> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<IntegrationRecord>(
        `SELECT id::text, platform, name, webhook_token, is_active,
                created_at::text
         FROM integrations
         ORDER BY created_at DESC
         LIMIT 50`,
      );
      return result.rows;
    });
  }
}
