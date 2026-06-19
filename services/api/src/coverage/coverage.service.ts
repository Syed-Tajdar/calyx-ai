import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface HeatmapStateRow {
  state: string;
  ping_count: number;
  revenue: number;
}

export interface HeatmapResult {
  data: HeatmapStateRow[];
  total_pings: number;
}

@Injectable()
export class CoverageService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async getHeatmap(tenantId: string): Promise<HeatmapResult> {
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          state,
          count() AS ping_count,
          sum(revenue) AS revenue
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
          AND state != ''
        GROUP BY state
        ORDER BY ping_count DESC
        LIMIT 100
      `,
      query_params: { tenant_id: tenantId },
      format: 'JSONEachRow',
    });

    const rows = await result.json<{
      state: string;
      ping_count: string;
      revenue: string;
    }>();

    const data: HeatmapStateRow[] = rows.map((row) => ({
      state: row.state,
      ping_count: Number(row.ping_count),
      revenue: Number(row.revenue),
    }));

    const total_pings = data.reduce((sum, row) => sum + row.ping_count, 0);

    return { data, total_pings };
  }
}
