import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface AttributionEventRecord {
  tenant_id: string;
  call_id: string;
  event_id: string;
  platform: string;
  click_id: string;
  revenue: number;
  status: string;
  http_status: number;
  error_message: string;
  fired_at: string;
}

export interface PlatformStatusSummary {
  platform: string;
  fired_count: number;
  failed_count: number;
  last_fired_at: string | null;
}

@Injectable()
export class AttributionService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async listEvents(
    tenantId: string,
    limit = 50,
  ): Promise<{ data: AttributionEventRecord[] }> {
    const safeLimit = Math.min(Math.max(limit, 1), 100);
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT tenant_id, call_id, event_id, platform, click_id, revenue, status,
               http_status, error_message,
               formatDateTime(fired_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS fired_at
        FROM attribution_fired_events
        WHERE tenant_id = {tenant_id:UUID}
        ORDER BY fired_at DESC
        LIMIT {limit:UInt32}
      `,
      query_params: { tenant_id: tenantId, limit: safeLimit },
      format: 'JSONEachRow',
    });
    const data = await result.json<AttributionEventRecord>();
    return { data };
  }

  async getPlatformStatus(
    tenantId: string,
  ): Promise<{ data: PlatformStatusSummary[] }> {
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          platform,
          countIf(status = 'fired') AS fired_count,
          countIf(status = 'failed') AS failed_count,
          formatDateTime(max(fired_at), '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS last_fired_at
        FROM attribution_fired_events
        WHERE tenant_id = {tenant_id:UUID}
        GROUP BY platform
        ORDER BY fired_count DESC
        LIMIT 20
      `,
      query_params: { tenant_id: tenantId },
      format: 'JSONEachRow',
    });
    const rows = await result.json<{
      platform: string;
      fired_count: string;
      failed_count: string;
      last_fired_at: string;
    }>();
    const data = rows.map((row) => ({
      platform: row.platform,
      fired_count: Number(row.fired_count),
      failed_count: Number(row.failed_count),
      last_fired_at: row.last_fired_at || null,
    }));
    return { data };
  }
}
