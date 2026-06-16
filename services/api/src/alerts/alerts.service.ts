import { Injectable } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';

export interface AlertRule {
  id: string;
  tenant_id: string;
  name: string;
  condition: Record<string, unknown>;
  threshold: number;
  channels: string[];
  actions: Record<string, unknown>[];
  is_active: boolean;
}

@Injectable()
export class AlertsService {
  constructor(private readonly postgres: PostgresService) {}

  async list(tenantId: string): Promise<AlertRule[]> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<AlertRule>(
        `SELECT id, tenant_id, name, condition, threshold, channels, actions, is_active
         FROM alert_rules WHERE is_active = TRUE ORDER BY created_at DESC`,
      );
      return result.rows;
    });
  }

  async create(
    tenantId: string,
    input: { name: string; condition: Record<string, unknown>; threshold: number },
  ): Promise<AlertRule> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<AlertRule>(
        `INSERT INTO alert_rules (tenant_id, name, condition, threshold)
         VALUES ($1, $2, $3, $4)
         RETURNING id, tenant_id, name, condition, threshold, channels, actions, is_active`,
        [tenantId, input.name, JSON.stringify(input.condition), input.threshold],
      );
      return result.rows[0];
    });
  }
}
