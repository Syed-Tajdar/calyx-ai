import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { PostgresService } from '../database/postgres.service';
import { loadEnv } from '../config/env';
import { AdSpendRow, createAdSpendSource } from './adspend.source';

export interface MetricsResult {
  total_pings: number;
  total_revenue: number;
  total_payout: number;
  cpa: number;
  rpc: number;
  roi: number;
}

export interface PnLResult {
  revenue: number;
  payout: number;
  ad_spend: number;
  profit: number;
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

  async getMetrics(tenantId: string, from?: string, to?: string): Promise<MetricsResult> {
    const dateClause = this.buildDateClause(from, to);
    const params: Record<string, string> = { tenant_id: tenantId };
    if (from) params.from = from;
    if (to) params.to = to;

    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          count() AS total_pings,
          sum(revenue) AS total_revenue,
          sum(payout) AS total_payout
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${dateClause}
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<{ total_pings: string; total_revenue: string; total_payout: string }>();
    const row = rows[0] ?? { total_pings: '0', total_revenue: '0', total_payout: '0' };
    const totalPings = Number(row.total_pings);
    const revenue = Number(row.total_revenue);
    const payout = Number(row.total_payout);
    const spendRows = await this.adSpendSource.getSpend(tenantId);
    const adSpend = spendRows.reduce((sum, r) => sum + r.amount, 0);
    const conversions = Math.max(totalPings, 1);

    return {
      total_pings: totalPings,
      total_revenue: revenue,
      total_payout: payout,
      cpa: adSpend > 0 ? adSpend / conversions : 0,
      rpc: conversions > 0 ? revenue / conversions : 0,
      roi: adSpend > 0 ? ((revenue - payout - adSpend) / adSpend) * 100 : 0,
    };
  }

  async getPnL(tenantId: string, from?: string, to?: string): Promise<PnLResult> {
    const metrics = await this.getMetrics(tenantId, from, to);
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
    if (from) parts.push('AND received_at >= parseDateTimeBestEffort({from:String})');
    if (to) parts.push('AND received_at < parseDateTimeBestEffort({to:String})');
    return parts.join('\n        ');
  }
}
