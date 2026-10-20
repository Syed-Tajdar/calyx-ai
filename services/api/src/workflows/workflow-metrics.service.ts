import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { testDataExclusionSql } from '../common/demo-rows';
import { Platform, Scope, WORKFLOW_TZ, WorkflowRule } from './workflow-types';

/** Raw per-entity counts for one rule window. All triggers are computed from these in `evaluate`. */
export interface EntityCounts {
  platform: string;
  entity_id: string;
  entity_name: string;
  /** Calls in the current window [from, to). */
  calls: number;
  converted: number;
  short_calls: number;
  repeat_calls: number;
  revenue: number;
  /** Same window 7 days earlier. */
  calls_prev_week: number;
  revenue_prev_week: number;
  /** Sum over k = 1..7 of calls in [from - k days, to - k days) (the "same hours" baseline). */
  baseline_calls_7: number;
  /** Calls in the 7 days before `from` (sample for no_calls). */
  calls_7d_before: number;
}

export interface WindowSpec {
  from: Date;
  to: Date;
}

export interface Match {
  platform: string;
  entity_id: string;
  entity_name: string;
  numbers: Record<string, number | string | null>;
}

export interface EvaluationResult {
  matches: Match[];
  /** Entities that crossed the threshold but had fewer than min_calls (never fire). */
  below_min_calls: number;
  evaluated: number;
  skipped_reason?: string;
}

/**
 * Whitelisted entity expressions over the per-call subquery. Never built from user text.
 * Note the alias names (`ent_*`, `in_cur`, …): ClickHouse resolves a name to a SELECT alias
 * first, so an alias must never reuse a column name (see ask-queries.service.ts).
 */
const ENTITY_EXPR: Record<Scope, { platform: string; id: string; name: string }> = {
  publisher: { platform: 'platform', id: 'publisher_id', name: 'anyLast(publisher_name)' },
  campaign: { platform: 'platform', id: 'campaign_id', name: 'anyLast(campaign_name)' },
  tenant: { platform: "''", id: "''", name: "''" },
};

const round = (v: number, dp = 1) => {
  const f = 10 ** dp;
  return Math.round(v * f) / f;
};

const toTs = (d: Date) => Math.floor(d.getTime() / 1000);

/** Hour and minute of `d` in Eastern time. */
export function easternClock(d: Date): { hour: number; minute: number } {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: WORKFLOW_TZ,
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(d);
  const get = (t: string) => Number(parts.find((p) => p.type === t)?.value ?? 0);
  return { hour: get('hour') % 24, minute: get('minute') };
}

/**
 * no_calls with active hours: the whole window [now - window_hours, now] must lie inside today's
 * active hours in Eastern time, so night-time silence never counts as "no calls".
 */
export function insideActiveHours(
  now: Date,
  windowHours: number,
  active: { start: number; end: number } | undefined,
): boolean {
  if (!active) return true;
  const { hour, minute } = easternClock(now);
  const nowMin = hour * 60 + minute;
  const startMin = active.start * 60;
  const endMin = active.end * 60;
  return nowMin < endMin && nowMin - windowHours * 60 >= startMin;
}

export function ruleWindow(rule: Pick<WorkflowRule, 'window_hours'>, now: Date): WindowSpec {
  return { from: new Date(now.getTime() - rule.window_hours * 3_600_000), to: now };
}

/**
 * Pure trigger logic over the raw counts. Thresholds in percent; min_calls is the sample guard:
 * - rate/share triggers: calls in the window must be >= min_calls;
 * - comparison triggers (volume_change, revenue_drop): the larger of the two compared samples
 *   must be >= min_calls, so a collapse to zero still fires but a tiny publisher never does;
 * - no_calls: the campaign must have had >= min_calls in the 7 days before the window.
 */
export function evaluate(
  rule: Pick<WorkflowRule, 'trigger_type' | 'params' | 'min_calls' | 'window_hours'>,
  rows: EntityCounts[],
): EvaluationResult {
  const p = rule.params as Record<string, unknown>;
  const threshold = Number(p.threshold_pct ?? 0);
  const matches: Match[] = [];
  let below = 0;
  const push = (r: EntityCounts, numbers: Match['numbers'], sample: number) => {
    if (sample < rule.min_calls) {
      below++;
      return;
    }
    matches.push({
      platform: r.platform,
      entity_id: r.entity_id,
      entity_name: r.entity_name,
      numbers: { ...numbers, min_calls: rule.min_calls, window_hours: rule.window_hours },
    });
  };
  for (const r of rows) {
    switch (rule.trigger_type) {
      case 'conversion_rate_below': {
        if (r.calls === 0) break;
        const rate = (100 * r.converted) / r.calls;
        if (rate < threshold) {
          push(r, { calls: r.calls, converted: r.converted, conversion_rate_pct: round(rate), threshold_pct: threshold }, r.calls);
        }
        break;
      }
      case 'short_call_share_above': {
        if (r.calls === 0) break;
        const share = (100 * r.short_calls) / r.calls;
        if (share > threshold) {
          push(
            r,
            {
              calls: r.calls,
              short_calls: r.short_calls,
              short_share_pct: round(share),
              short_seconds: Number(p.short_seconds),
              threshold_pct: threshold,
            },
            r.calls,
          );
        }
        break;
      }
      case 'repeat_caller_share_above': {
        if (r.calls === 0) break;
        const share = (100 * r.repeat_calls) / r.calls;
        if (share > threshold) {
          push(
            r,
            { calls: r.calls, repeat_calls: r.repeat_calls, repeat_share_pct: round(share), threshold_pct: threshold },
            r.calls,
          );
        }
        break;
      }
      case 'volume_change': {
        const avg = r.baseline_calls_7 / 7;
        if (avg <= 0) break; // no baseline, no comparison
        const change = (100 * (r.calls - avg)) / avg;
        const dir = (p.direction as string) ?? 'both';
        const hit =
          (dir !== 'down' && change > threshold) || (dir !== 'up' && change < -threshold);
        if (hit) {
          push(
            r,
            {
              calls: r.calls,
              baseline_avg_calls: round(avg, 1),
              change_pct: round(change),
              direction: dir,
              threshold_pct: threshold,
            },
            Math.max(r.calls, avg),
          );
        }
        break;
      }
      case 'revenue_drop': {
        if (r.revenue_prev_week <= 0) break;
        const drop = (100 * (r.revenue_prev_week - r.revenue)) / r.revenue_prev_week;
        if (drop > threshold) {
          push(
            r,
            {
              calls: r.calls,
              revenue: round(r.revenue, 2),
              revenue_prev_week: round(r.revenue_prev_week, 2),
              calls_prev_week: r.calls_prev_week,
              drop_pct: round(drop),
              threshold_pct: threshold,
            },
            Math.max(r.calls, r.calls_prev_week),
          );
        }
        break;
      }
      case 'no_calls': {
        if (r.calls === 0) {
          push(r, { calls: 0, calls_7d_before: r.calls_7d_before }, r.calls_7d_before);
        }
        break;
      }
      default:
        break;
    }
  }
  return { matches, below_min_calls: below, evaluated: rows.length };
}

/**
 * ClickHouse reads for the evaluator. Same deduplication as the dashboard and Ask AI: one row per
 * (platform, external_call_id), latest version wins (`argMax(..., received_at)`), revenue excludes
 * platform duplicates, test rows excluded, Ringba + Retreaver only, converted = flag AND revenue > 0.
 * Every value is a query parameter; the only SQL built in code comes from whitelists.
 */
@Injectable()
export class WorkflowMetricsService {
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
        argMax(duration_seconds, received_at) AS dur,
        replaceAll(toString(argMax(ani_hash, received_at)), '\\0', '') AS ani,
        argMax(campaign_id, received_at) AS campaign_id,
        argMax(campaign_name, received_at) AS campaign_name,
        argMax(publisher_id, received_at) AS publisher_id,
        argMax(publisher_name, received_at) AS publisher_name,
        max(${EVENT_AT_EXPR}) AS event_at
      FROM pings
      WHERE tenant_id = {tenant_id:UUID}
        AND external_call_id != ''
        ${testDataExclusionSql('external_call_id')}
        AND platform IN ('ringba', 'retreaver')
        AND (platform = {platform:String} OR {platform:String} = 'all')
        AND ${EVENT_AT_EXPR} >= toDateTime({scan_from:UInt32}, 'UTC') - INTERVAL 1 DAY
        AND ${EVENT_AT_EXPR} <  toDateTime({to:UInt32}, 'UTC') + INTERVAL 1 DAY
      GROUP BY platform, external_call_id`;
  }

  /** Raw counts per entity for the rule's scope and window (plus the 7-day comparison windows). */
  async entityCounts(
    tenantId: string,
    scope: Scope,
    platform: Platform | null,
    win: WindowSpec,
    shortSeconds: number,
  ): Promise<EntityCounts[]> {
    const e = ENTITY_EXPR[scope];
    const from = toTs(win.from);
    const to = toTs(win.to);
    const params = {
      tenant_id: tenantId,
      platform: platform ?? 'all',
      from,
      to,
      scan_from: from - 7 * 86_400,
      short_seconds: shortSeconds,
    };
    const having = scope === 'tenant' ? '' : "HAVING ent_id != ''";
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          ${e.platform} AS ent_platform,
          ${e.id} AS ent_id,
          ${e.name} AS ent_name,
          countIf(in_cur) AS calls,
          countIf(in_cur AND conv = 1 AND rev > 0) AS converted,
          countIf(in_cur AND dur < {short_seconds:UInt32}) AS short_calls,
          countIf(in_cur AND ani != '' AND ani_calls > 1) AS repeat_calls,
          toFloat64(sumIf(rev, in_cur AND dup = 0)) AS rev_cur,
          countIf(in_prev_week) AS calls_prev_week,
          toFloat64(sumIf(rev, in_prev_week AND dup = 0)) AS rev_prev_week,
          sum(base_hits) AS baseline_calls_7,
          countIf(ts >= {scan_from:UInt32} AND ts < {from:UInt32}) AS calls_7d_before
        FROM (
          SELECT
            *,
            toUInt32(toUnixTimestamp(event_at)) AS ts,
            ts >= {from:UInt32} AND ts < {to:UInt32} AS in_cur,
            ts >= {from:UInt32} - 604800 AND ts < {to:UInt32} - 604800 AS in_prev_week,
            arrayCount(k -> ts >= {from:UInt32} - k * 86400 AND ts < {to:UInt32} - k * 86400, range(1, 8)) AS base_hits,
            sum(toUInt32(in_cur)) OVER (PARTITION BY ani) AS ani_calls
          FROM (${this.perCallSubquery()})
          WHERE event_at >= toDateTime({scan_from:UInt32}, 'UTC')
            AND event_at <  toDateTime({to:UInt32}, 'UTC')
        )
        GROUP BY ent_platform, ent_id
        ${having}`,
      query_params: params,
      format: 'JSONEachRow',
    });
    const rows = await result.json<Record<string, string | number>>();
    return rows.map((r) => ({
      platform: String(r.ent_platform ?? ''),
      entity_id: String(r.ent_id ?? ''),
      entity_name: String(r.ent_name ?? ''),
      calls: Number(r.calls),
      converted: Number(r.converted),
      short_calls: Number(r.short_calls),
      repeat_calls: Number(r.repeat_calls),
      revenue: Number(r.rev_cur),
      calls_prev_week: Number(r.calls_prev_week),
      revenue_prev_week: Number(r.rev_prev_week),
      baseline_calls_7: Number(r.baseline_calls_7),
      calls_7d_before: Number(r.calls_7d_before),
    }));
  }

  /** Calls and revenue for one publisher over the last 7 × 24 hours (the confirm popup). */
  async publisherImpact(
    tenantId: string,
    platform: Platform,
    publisherId: string,
    now: Date = new Date(),
  ): Promise<{ calls: number; revenue: number; publisher_name: string; from: string; to: string }> {
    const to = toTs(now);
    const from = to - 7 * 86_400;
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          count() AS calls,
          toFloat64(sumIf(rev, dup = 0)) AS rev_total,
          anyLast(publisher_name) AS pub_name
        FROM (${this.perCallSubquery()})
        WHERE publisher_id = {publisher_id:String}
          AND event_at >= toDateTime({from:UInt32}, 'UTC')
          AND event_at <  toDateTime({to:UInt32}, 'UTC')`,
      query_params: { tenant_id: tenantId, platform, publisher_id: publisherId, from, to, scan_from: from },
      format: 'JSONEachRow',
    });
    const rows = await result.json<Record<string, string | number>>();
    const r = rows[0] ?? {};
    return {
      calls: Number(r.calls ?? 0),
      revenue: round(Number(r.rev_total ?? 0), 2),
      publisher_name: String(r.pub_name ?? ''),
      from: new Date(from * 1000).toISOString(),
      to: new Date(to * 1000).toISOString(),
    };
  }
}
