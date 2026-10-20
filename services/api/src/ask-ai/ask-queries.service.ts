import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { testDataExclusionSql } from '../common/demo-rows';
import {
  assertRange,
  CompareArgs,
  Dimension,
  ListValuesArgs,
  MetricsArgs,
} from './ask-tools';

/** Eastern time is the default for every Ask AI date (owner decision, same as the go-live recon). */
export const ASK_TZ = 'America/New_York';

export interface MetricRow {
  group: string;
  calls: number;
  converted: number;
  conversion_rate_pct: number;
  revenue: number;
  payout: number;
  profit: number;
  revenue_per_call: number;
}

export interface CompareRow {
  group: string;
  current: MetricRow;
  previous: MetricRow;
  calls_change_pct: number | null;
  revenue_change_pct: number | null;
  conversion_rate_change: number;
}

/** Whitelisted dimension expressions over the per-call subquery. Never built from user text. */
const DIM_EXPR: Record<Dimension | 'none', string> = {
  none: "'all'",
  platform: 'platform',
  campaign: "if(campaign_name != '', campaign_name, campaign_id)",
  publisher: "if(publisher_name != '', publisher_name, publisher_id)",
  buyer: "multiIf(buyer_name != '', buyer_name, target_name != '', target_name, buyer_id)",
  state: 'state',
  day: `toString(toDate(toTimeZone(event_at, '${ASK_TZ}')))`,
};

const ORDER_EXPR: Record<string, string> = {
  calls: 'calls',
  converted: 'converted',
  conversion_rate: 'conversion_rate_pct',
  revenue: 'revenue',
  payout: 'payout',
  profit: 'profit',
  revenue_per_call: 'revenue_per_call',
};

const round = (v: number, dp: number) => {
  const f = 10 ** dp;
  return Math.round(v * f) / f;
};

/**
 * Note: the per-call columns are named `rev` / `pay`, not `revenue` / `payout`. ClickHouse
 * resolves a name to a SELECT alias first, so reusing `revenue` for the per-call value and the
 * total would nest one aggregate inside another (found on the laptop stack, not by mocks).
 *
 * Read-only analytics for Ask AI. Same deduplication as the dashboard and the go-live
 * reconciliation: one row per (platform, external_call_id), latest version wins
 * (`argMax(..., received_at)`), revenue and payout exclude platform duplicates, test rows
 * excluded, "converted" = flag and revenue > 0. A call belongs to the Eastern-time day of its
 * event time. Every user-supplied value is a query parameter.
 */
@Injectable()
export class AskQueriesService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  private perCallSubquery(): string {
    return `
      SELECT
        platform,
        external_call_id,
        argMax(revenue, received_at) AS rev,
        argMax(payout, received_at) AS pay,
        argMax(platform_duplicate, received_at) AS dup,
        argMax(has_converted, received_at) AS conv,
        argMax(campaign_id, received_at) AS campaign_id,
        argMax(campaign_name, received_at) AS campaign_name,
        argMax(publisher_id, received_at) AS publisher_id,
        argMax(publisher_name, received_at) AS publisher_name,
        argMax(buyer_id, received_at) AS buyer_id,
        argMax(buyer_name, received_at) AS buyer_name,
        argMax(target_name, received_at) AS target_name,
        argMax(state, received_at) AS state,
        max(${EVENT_AT_EXPR}) AS event_at
      FROM pings
      WHERE tenant_id = {tenant_id:UUID}
        AND external_call_id != ''
        ${testDataExclusionSql('external_call_id')}
        AND platform IN ('ringba', 'retreaver')
        AND (platform = {platform:String} OR {platform:String} = 'all')
        AND ${EVENT_AT_EXPR} >= toDateTime({from:Date}) - INTERVAL 1 DAY
        AND ${EVENT_AT_EXPR} <  toDateTime({to:Date}) + INTERVAL 2 DAY
      GROUP BY platform, external_call_id`;
  }

  private filterSql(filter: MetricsArgs['filter']): string {
    if (!filter) return '';
    return `AND ${DIM_EXPR[filter.dimension]} = {filter_value:String}`;
  }

  private async grouped(
    tenantId: string,
    groupBy: Dimension | 'none',
    from: string,
    to: string,
    platform: string,
    filter: MetricsArgs['filter'],
    orderBy: string,
    order: 'asc' | 'desc',
    limit: number,
    minCalls: number,
  ): Promise<MetricRow[]> {
    assertRange(from, to);
    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      from,
      to,
      platform,
      min_calls: minCalls,
      limit,
    };
    if (filter) params.filter_value = filter.value;
    const orderExpr = ORDER_EXPR[orderBy] ?? 'calls';
    const dir = order === 'asc' ? 'ASC' : 'DESC';
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          ${DIM_EXPR[groupBy]} AS grp,
          count() AS calls,
          countIf(conv = 1 AND rev > 0) AS converted,
          round(100 * converted / calls, 1) AS conversion_rate_pct,
          round(toFloat64(sumIf(rev, dup = 0)), 2) AS revenue,
          round(toFloat64(sumIf(pay, dup = 0)), 2) AS payout,
          round(revenue - payout, 2) AS profit,
          round(revenue / calls, 2) AS revenue_per_call
        FROM (${this.perCallSubquery()})
        WHERE toDate(toTimeZone(event_at, '${ASK_TZ}')) >= {from:Date}
          AND toDate(toTimeZone(event_at, '${ASK_TZ}')) <= {to:Date}
          ${this.filterSql(filter)}
        GROUP BY grp
        HAVING calls >= {min_calls:UInt32}
        ORDER BY ${orderExpr} ${dir}, grp ASC
        LIMIT {limit:UInt32}`,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<Record<string, string | number>>();
    return rows.map((r) => ({
      group: String(r.grp) || '(unknown)',
      calls: Number(r.calls),
      converted: Number(r.converted),
      conversion_rate_pct: Number(r.conversion_rate_pct),
      revenue: Number(r.revenue),
      payout: Number(r.payout),
      profit: Number(r.profit),
      revenue_per_call: Number(r.revenue_per_call),
    }));
  }

  metrics(tenantId: string, a: MetricsArgs): Promise<MetricRow[]> {
    return this.grouped(
      tenantId,
      a.group_by,
      a.from,
      a.to,
      a.platform,
      a.filter,
      a.order_by,
      a.order,
      a.group_by === 'none' ? 1 : a.limit,
      a.group_by === 'none' ? 0 : a.min_calls,
    );
  }

  async compare(tenantId: string, a: CompareArgs): Promise<CompareRow[]> {
    const fetch = (from: string, to: string) =>
      this.grouped(tenantId, a.group_by, from, to, a.platform, a.filter, 'calls', 'desc', 500, 0);
    const [cur, prev] = await Promise.all([
      fetch(a.current.from, a.current.to),
      fetch(a.previous.from, a.previous.to),
    ]);
    const empty = (group: string): MetricRow => ({
      group,
      calls: 0,
      converted: 0,
      conversion_rate_pct: 0,
      revenue: 0,
      payout: 0,
      profit: 0,
      revenue_per_call: 0,
    });
    const prevBy = new Map(prev.map((r) => [r.group, r]));
    const groups = new Set([...cur.map((r) => r.group), ...prev.map((r) => r.group)]);
    const curBy = new Map(cur.map((r) => [r.group, r]));
    const pct = (now: number, before: number) =>
      before === 0 ? null : round(((now - before) / before) * 100, 1);
    const rows: CompareRow[] = [...groups].map((g) => {
      const c = curBy.get(g) ?? empty(g);
      const p = prevBy.get(g) ?? empty(g);
      return {
        group: g,
        current: c,
        previous: p,
        calls_change_pct: pct(c.calls, p.calls),
        revenue_change_pct: pct(c.revenue, p.revenue),
        conversion_rate_change: round(c.conversion_rate_pct - p.conversion_rate_pct, 1),
      };
    });
    const key = (r: CompareRow): number => {
      switch (a.order_by) {
        case 'calls_change_pct':
          return r.calls_change_pct ?? Number.NEGATIVE_INFINITY;
        case 'revenue_change_pct':
          return r.revenue_change_pct ?? Number.NEGATIVE_INFINITY;
        case 'conversion_rate_change':
          return r.conversion_rate_change;
        case 'revenue':
          return r.current.revenue;
        default:
          return r.current.calls;
      }
    };
    return rows
      .filter((r) => r.previous.calls >= a.min_calls || a.group_by === 'none')
      .sort((x, y) => (a.order === 'asc' ? key(x) - key(y) : key(y) - key(x)) || x.group.localeCompare(y.group))
      .slice(0, a.limit);
  }

  async listValues(tenantId: string, a: ListValuesArgs): Promise<Array<{ value: string; calls: number }>> {
    assertRange(a.from, a.to);
    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      from: a.from,
      to: a.to,
      platform: 'all',
      limit: a.limit,
      search: a.search ?? '',
    };
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT ${DIM_EXPR[a.dimension]} AS value, count() AS calls
        FROM (${this.perCallSubquery()})
        WHERE toDate(toTimeZone(event_at, '${ASK_TZ}')) >= {from:Date}
          AND toDate(toTimeZone(event_at, '${ASK_TZ}')) <= {to:Date}
        GROUP BY value
        HAVING value != '' AND ({search:String} = '' OR positionCaseInsensitiveUTF8(value, {search:String}) > 0)
        ORDER BY calls DESC, value ASC
        LIMIT {limit:UInt32}`,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<{ value: string; calls: string }>();
    return rows.map((r) => ({ value: r.value, calls: Number(r.calls) }));
  }
}
