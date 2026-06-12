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
    const result = await this.postgres.getPool().query<IntegrationRecord>(
      `SELECT id::text, platform, name, webhook_token, is_active,
              created_at::text
       FROM integrations
       WHERE tenant_id = $1
       ORDER BY created_at DESC
       LIMIT 50`,
      [tenantId],
    );
    return result.rows;
  }
}
