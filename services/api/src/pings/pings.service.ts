import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface PingRecord {
  tenant_id: string;
  platform: string;
  external_call_id: string;
  ani_hash: string;
  campaign_id: string;
  publisher_id: string;
  target_id: string;
  state: string;
  event_type: string;
  revenue: number;
  payout: number;
  is_duplicate: number;
  received_at: string;
}

export interface PingListResult {
  data: PingRecord[];
  total: number;
  limit: number;
  offset: number;
}

@Injectable()
export class PingsService {
  private static readonly MAX_LIMIT = 100;

  constructor(private readonly clickhouse: ClickhouseService) {}

  async list(
    tenantId: string,
    limit = 50,
    offset = 0,
    platform?: string,
  ): Promise<PingListResult> {
    const safeLimit = Math.min(Math.max(limit, 1), PingsService.MAX_LIMIT);
    const safeOffset = Math.max(offset, 0);

    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      limit: safeLimit,
      offset: safeOffset,
    };

    let platformFilter = '';
    if (platform) {
      platformFilter = 'AND platform = {platform:String}';
      params.platform = platform;
    }

    const countResult = await this.clickhouse.getClient().query({
      query: `
        SELECT count() AS total
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${platformFilter}
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const countRows = await countResult.json<{ total: string }>();
    const total = Number(countRows[0]?.total ?? 0);

    const dataResult = await this.clickhouse.getClient().query({
      query: `
        SELECT
          tenant_id, platform, external_call_id, ani_hash,
          campaign_id, publisher_id, target_id, state,
          event_type, revenue, payout, is_duplicate,
          formatDateTime(received_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS received_at
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${platformFilter}
        ORDER BY received_at DESC
        LIMIT {limit:UInt32}
        OFFSET {offset:UInt32}
      `,
      query_params: params,
      format: 'JSONEachRow',
    });

    const data = await dataResult.json<PingRecord>();

    return {
      data,
      total,
      limit: safeLimit,
      offset: safeOffset,
    };
  }
}
