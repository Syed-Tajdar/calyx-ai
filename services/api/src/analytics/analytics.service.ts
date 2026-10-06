import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { PostgresService } from '../database/postgres.service';
import { loadEnv } from '../config/env';
import { AdSpendRow, createAdSpendSource } from './adspend.source';
import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { apiSyncExclusionSql } from '../common/demo-rows';

export interface MetricsResult {
  total_pings: number;
  total_revenue: number;
  total_payout: number;
  cpa: number;
  rpc: number;
  roi: number;
  /** Fraction of distinct calls with platform_duplicate=1 (0..1). */
  duplicate_rate: number;
}

export interface PnLResult {
  revenue: number;
  payout: number;
  ad_spend: number;
  profit: number;
}

export interface PlatformMixRow {
  platform: string;
  count: number;
  percent: number;
}

@Injectable()
export class AnalyticsService {
  private readonly adSpendSource;

  constructor(
    private readonly clickhouse: ClickhouseService,
    private readonly postgres: PostgresService,
  ) {
    const env = loadEnv();
    this.adSpendSource = createAdSpendSource(env.ADSPEND_SOURCE, (tenantId) =>
      this.loadManualAdSpend(tenantId),
    );
  }

  /** Distinct calls; revenue/payout kept for conditional sum (exclude platform_duplicate). */
  private dedupedCallsSubquery(showDemo?: boolean): string {
    const demo = apiSyncExclusionSql('external_call_id', showDemo);
    return `
        SELECT
          platform,
          external_call_id,
          argMax(revenue, received_at) AS revenue,
          argMax(payout, received_at) AS payout,
          argMax(platform_duplicate, received_at) AS platform_duplicate
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
          AND external_call_id != ''
          ${demo}`;
  }

  async getMetrics(
    tenantId: string,
    from?: string,
    to?: string,
    showDemo?: boolean,
    platform?: string,
  ): Promise<MetricsResult> {
    const dateClause = this.buildDateClause(from, to);
    const platformClause = platform ? 'AND platform = {platform:String}' : '';
    const params: Record<string, string> = { tenant_id: tenantId };
    if (from) params.from = from;
    if (to) params.to = to;
    if (platform) params.platform = platform;

    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          count() AS total_pings,
          sumIf(revenue, platform_duplicate = 0) AS total_revenue,
          sumIf(payout, platform_duplicate = 0) AS total_payout,
          countIf(platform_duplicate = 1) AS duplicate_pings
        FROM (
          ${this.dedupedCallsSubquery(showDemo)}
          ${dateClause}
          ${platformClause}
          GROUP BY platform, external_call_id
        )
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<{
      total_pings: string;
      total_revenue: string;
      total_payout: string;
      duplicate_pings: string;
    }>();
    const row = rows[0] ?? {
      total_pings: '0',
      total_revenue: '0',
      total_payout: '0',
      duplicate_pings: '0',
    };
    const totalPings = Number(row.total_pings);
    const revenue = Number(row.total_revenue);
    const payout = Number(row.total_payout);
    const duplicatePings = Number(row.duplicate_pings);
    const spendRows = await this.adSpendSource.getSpend(tenantId);
    const adSpend = spendRows.reduce((sum, r) => sum + r.amount, 0);
    const conversions = Math.max(totalPings, 1);
    const duplicateRate = totalPings > 0 ? duplicatePings / totalPings : 0;

    return {
      total_pings: totalPings,
      total_revenue: revenue,
      total_payout: payout,
      cpa: adSpend > 0 ? adSpend / conversions : 0,
      rpc: conversions > 0 ? revenue / conversions : 0,
      roi: adSpend > 0 ? ((revenue - payout - adSpend) / adSpend) * 100 : 0,
      duplicate_rate: duplicateRate,
    };
  }

  /**
   * One row per day for the same calls `getMetrics` counts, so the chart total
   * can match Total Pings for the selected period.
   */
  async getPingSeries(
    tenantId: string,
    from?: string,
    to?: string,
    showDemo?: boolean,
    platform?: string,
  ): Promise<Array<{ date: string; count: number }>> {
    const dateClause = this.buildDateClause(from, to);
    const platformClause = platform ? 'AND platform = {platform:String}' : '';
    const demo = apiSyncExclusionSql('external_call_id', showDemo);
    const params: Record<string, string> = { tenant_id: tenantId };
    if (from) params.from = from;
    if (to) params.to = to;
    if (platform) params.platform = platform;
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          formatDateTime(toDate(event_at), '%Y-%m-%d') AS day,
          count() AS count
        FROM (
          SELECT max(${EVENT_AT_EXPR}) AS event_at
          FROM pings
          WHERE tenant_id = {tenant_id:UUID}
            AND external_call_id != ''
            ${demo}
            ${dateClause}
            ${platformClause}
          GROUP BY platform, external_call_id
        )
        GROUP BY day
        ORDER BY day
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<{ day: string; count: string }>();
    return rows.map((row) => ({ date: row.day, count: Number(row.count) }));
  }

  async getPnL(
    tenantId: string,
    from?: string,
    to?: string,
    showDemo?: boolean,
  ): Promise<PnLResult> {
    const metrics = await this.getMetrics(tenantId, from, to, showDemo);
    const spendRows = await this.adSpendSource.getSpend(tenantId);
    const adSpend = spendRows.reduce((sum: number, r: AdSpendRow) => sum + r.amount, 0);
    const profit = metrics.total_revenue - metrics.total_payout - adSpend;
    return {
      revenue: metrics.total_revenue,
      payout: metrics.total_payout,
      ad_spend: adSpend,
      profit,
    };
  }

  async getPlatformMix(
    tenantId: string,
    from?: string,
    to?: string,
    platform?: string,
    showDemo?: boolean,
  ): Promise<PlatformMixRow[]> {
    const dateClause = this.buildDateClause(from, to);
    const platformClause = platform ? 'AND platform = {platform:String}' : '';
    const params: Record<string, string> = { tenant_id: tenantId };
    if (from) params.from = from;
    if (to) params.to = to;
    if (platform) params.platform = platform;

    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT platform, count() AS count
        FROM (
          ${this.dedupedCallsSubquery(showDemo)}
          ${dateClause}
          ${platformClause}
          GROUP BY platform, external_call_id
        )
        GROUP BY platform
        ORDER BY count DESC
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<{ platform: string; count: string }>();
    const total = rows.reduce((sum, row) => sum + Number(row.count), 0) || 1;
    return rows.map((row) => {
      const count = Number(row.count);
      return {
        platform: row.platform,
        count,
        percent: Math.round((count / total) * 100),
      };
    });
  }

  private async loadManualAdSpend(tenantId: string): Promise<AdSpendRow[]> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<{
        campaign_id: string;
        spend_date: Date;
        amount: string;
      }>(
        `SELECT campaign_id, spend_date, amount
         FROM adspend_entries
         WHERE tenant_id = $1
         ORDER BY spend_date DESC
         LIMIT 100`,
        [tenantId],
      );
      return result.rows.map((row) => ({
        campaign_id: row.campaign_id,
        spend_date: row.spend_date.toISOString().slice(0, 10),
        amount: Number(row.amount),
      }));
    });
  }

  private buildDateClause(from?: string, to?: string): string {
    const parts: string[] = [];
    if (from) parts.push(`AND ${EVENT_AT_EXPR} >= parseDateTimeBestEffort({from:String})`);
    if (to) parts.push(`AND ${EVENT_AT_EXPR} < parseDateTimeBestEffort({to:String})`);
    return parts.join('\n          ');
  }
}
