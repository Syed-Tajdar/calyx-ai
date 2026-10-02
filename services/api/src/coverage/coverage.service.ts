import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { countryForRegion, RegionCountry } from '../common/na-regions';
import { apiSyncExclusionSql } from '../common/demo-rows';
import { EVENT_AT_EXPR } from '../common/ping-event-time';

export interface HeatmapStateRow {
  state: string;
  country: RegionCountry | '';
  ping_count: number;
  revenue: number;
}

export interface HeatmapResult {
  data: HeatmapStateRow[];
  total_pings: number;
  us_regions: number;
  ca_regions: number;
  /** Distinct calls in the same filter that are not on the US/CA map. */
  unmapped_pings: number;
}

@Injectable()
export class CoverageService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  /**
   * Geo heatmap of distinct calls.
   *
   * `from` / `to` use the same event-time expression and semantics as `/analytics/metrics`, so the
   * Coverage map and the dashboard agree for a given period. Without them the map is all-time,
   * which is what produced the long-standing dashboard-vs-coverage discrepancy (enterprise-roadmap
   * Gap C): the dashboard defaulted to 30 days while the map silently showed everything.
   */
  async getHeatmap(
    tenantId: string,
    showDemo?: boolean,
    from?: string,
    to?: string,
    platform?: string,
  ): Promise<HeatmapResult> {
    const demo = apiSyncExclusionSql('external_call_id', showDemo);
    const params: Record<string, string> = { tenant_id: tenantId };
    let period = '';
    if (from) {
      period += `\n            AND ${EVENT_AT_EXPR} >= parseDateTimeBestEffort({from:String})`;
      params.from = from;
    }
    if (to) {
      period += `\n            AND ${EVENT_AT_EXPR} < parseDateTimeBestEffort({to:String})`;
      params.to = to;
    }
    if (platform) {
      period += `\n            AND platform = {platform:String}`;
      params.platform = platform;
    }
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          state,
          count() AS ping_count,
          sumIf(revenue, platform_duplicate = 0) AS revenue
        FROM (
          SELECT
            platform,
            external_call_id,
            argMax(state, received_at) AS state,
            argMax(revenue, received_at) AS revenue,
            argMax(platform_duplicate, received_at) AS platform_duplicate
          FROM pings
          WHERE tenant_id = {tenant_id:UUID}
            AND external_call_id != ''
            ${demo}${period}
          GROUP BY platform, external_call_id
        )
        WHERE state != ''
        GROUP BY state
        ORDER BY ping_count DESC
        LIMIT 100
      `,
      query_params: params,
      format: 'JSONEachRow',
    });

    const rows = await result.json<{
      state: string;
      ping_count: string;
      revenue: string;
    }>();

    const data: HeatmapStateRow[] = [];
    let usRegions = 0;
    let caRegions = 0;

    for (const row of rows) {
      const code = row.state.trim().toUpperCase();
      const country = countryForRegion(code);
      // Drop junk regions that predate the US+CA allowlist (or slipped through).
      if (!country) continue;
      if (country === 'US') usRegions += 1;
      else caRegions += 1;
      data.push({
        state: code,
        country,
        ping_count: Number(row.ping_count),
        revenue: Number(row.revenue),
      });
    }

    const total_pings = data.reduce((sum, row) => sum + row.ping_count, 0);

    const allResult = await this.clickhouse.getClient().query({
      query: `
        SELECT count() AS total
        FROM (
          SELECT platform, external_call_id
          FROM pings
          WHERE tenant_id = {tenant_id:UUID}
            AND external_call_id != ''
            ${demo}${period}
          GROUP BY platform, external_call_id
        )
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const allRows = await allResult.json<{ total: string }>();
    const allCalls = Number(allRows[0]?.total ?? 0);

    return {
      data,
      total_pings,
      us_regions: usRegions,
      ca_regions: caRegions,
      unmapped_pings: Math.max(0, allCalls - total_pings),
    };
  }
}
