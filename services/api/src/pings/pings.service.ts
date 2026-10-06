import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { buildPingWhereClause, pingConversionPredicate, PingListFilters } from './pings.filters';
import { conversionState, ConversionState, reportsConversionState } from '../common/platform-capabilities';

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
  /**
   * Platform-reported conversion state. `has_converted` is `UInt8 DEFAULT 0` in ClickHouse, so a
   * missing value is indistinguishable from an explicit false (audit F038/K8) — 0 is read as
   * not-converted, matching the deep view.
   */
  has_converted: boolean;
  /** Connected talk time in seconds. */
  duration_seconds: number;
  /** Buyer's qualifying threshold when the platform reports one; null when unknown. */
  required_duration_seconds: number | null;
  /** How many times this call has been written (re-syncs). 1 in raw mode. */
  sync_count: number;
  /**
   * Whether this platform reports conversion state at all (C6). When false, `has_converted`
   * is "never asked", not "not converted" — the UI must not infer a missed conversion from it.
   */
  conversion_reported: boolean;
  /**
   * Stamped by the API from `conversionState`. The web displays this and does not recompute it.
   * converted | converted_zero | not_converted | not_reported
   */
  conversion_state: ConversionState;
  /** Ingestion timestamp (when Calyx received the ping). */
  received_at: string;
  /**
   * Source call time when known (e.g. Retreaver created_at); otherwise same as received_at.
   * Use this for Ping Feed TIME, period filters, and charts.
   */
  occurred_at: string;
}

export interface PingListResult {
  data: PingRecord[];
  /** Raw row count (for pagination through the event stream). */
  total: number;
  /** Distinct calls (`uniqExact(external_call_id)`) — headline / Coverage-aligned. */
  distinct_total: number;
  limit: number;
  offset: number;
}

/** Prefer platform source time; fall back to ingestion time. */
export { EVENT_AT_EXPR } from '../common/ping-event-time';

const PING_SELECT = `
  tenant_id, platform, external_call_id, ani_hash,
  campaign_id, publisher_id, target_id, state,
  event_type, revenue, payout, is_duplicate,
  has_converted, duration_seconds, required_duration_seconds,
  formatDateTime(received_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS received_at_iso,
  formatDateTime(${EVENT_AT_EXPR}, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS occurred_at_iso
`;

/**
 * One row per call instead of one row per sync.
 *
 * The raw `pings` table is an event stream: every re-sync that changed something writes another
 * row, so the same call legitimately appears several times. That is correct storage but reads as a
 * bug in the call list — and it made the table disagree with its own "N calls" headline, which has
 * always counted distinct calls. Collapsing here mirrors what analytics and coverage already do
 * (`argMax(..., received_at)`), so every surface now tells the same story. `raw=true` returns the
 * event stream for debugging a sync.
 */
const PING_SELECT_COLLAPSED = `
  platform,
  external_call_id,
  argMax(tenant_id, received_at)                 AS tenant_id,
  argMax(ani_hash, received_at)                  AS ani_hash,
  argMax(campaign_id, received_at)               AS campaign_id,
  argMax(publisher_id, received_at)              AS publisher_id,
  argMax(target_id, received_at)                 AS target_id,
  argMax(state, received_at)                     AS state,
  argMax(event_type, received_at)                AS event_type,
  argMax(revenue, received_at)                   AS revenue,
  argMax(payout, received_at)                    AS payout,
  argMax(has_converted, received_at)             AS has_converted,
  argMax(duration_seconds, received_at)          AS duration_seconds,
  argMax(required_duration_seconds, received_at) AS required_duration_seconds,
  count()                                        AS sync_count,
  if(count() > 1, 1, 0)                          AS is_duplicate,
  max(${EVENT_AT_EXPR})                          AS event_at,
  formatDateTime(max(received_at), '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS received_at_iso,
  formatDateTime(max(${EVENT_AT_EXPR}), '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS occurred_at_iso
`;

interface PingRecordRow
  extends Omit<
    PingRecord,
    | 'received_at'
    | 'occurred_at'
    | 'has_converted'
    | 'sync_count'
    | 'conversion_reported'
    | 'conversion_state'
  > {
  /** ClickHouse `UInt8` — 1 when the platform reported the call converted. */
  has_converted: number;
  /** Times this call has been written (collapsed mode only; 1 in raw mode). */
  sync_count?: string | number;
  received_at_iso: string;
  occurred_at_iso: string;
}

function mapPingRow(row: PingRecordRow): PingRecord {
  const { received_at_iso, occurred_at_iso, has_converted, sync_count, ...rest } = row;
  const converted = Number(has_converted ?? 0) === 1;
  return {
    ...rest,
    has_converted: converted,
    sync_count: Number(sync_count ?? 1),
    conversion_reported: reportsConversionState(rest.platform),
    conversion_state: conversionState({
      platform: rest.platform,
      hasConverted: converted,
      revenue: Number(rest.revenue ?? 0),
    }),
    received_at: received_at_iso,
    occurred_at: occurred_at_iso,
  };
}

@Injectable()
export class PingsService {
  private static readonly MAX_LIMIT = 100;
  private static readonly EXPORT_MAX = 10_000;

  constructor(private readonly clickhouse: ClickhouseService) {}

  async list(
    tenantId: string,
    limit = 50,
    offset = 0,
    filters: PingListFilters = {},
  ): Promise<PingListResult> {
    const safeLimit = Math.min(Math.max(limit, 1), PingsService.MAX_LIMIT);
    const safeOffset = Math.max(offset, 0);

    const { clause, params: filterParams } = buildPingWhereClause(filters);
    const rawConversion = pingConversionPredicate(filters, 'raw');
    const latestConversion = pingConversionPredicate(filters, 'latest');
    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      limit: safeLimit,
      offset: safeOffset,
      ...filterParams,
    };

    const countResult = await this.clickhouse.getClient().query({
      query: filters.raw
        ? `
        SELECT
          count() AS total,
          uniqExactIf(external_call_id, external_call_id != '') AS distinct_total
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
        ${rawConversion ? `AND ${rawConversion}` : ''}
      `
        : filters.conversion
          ? `
        SELECT count() AS total, count() AS distinct_total
        FROM (
          SELECT platform, external_call_id
          FROM (
            SELECT * FROM pings
            WHERE tenant_id = {tenant_id:UUID}
              AND external_call_id != ''
            ${clause}
          )
          GROUP BY platform, external_call_id
          HAVING ${latestConversion}
        )
      `
          : `
        SELECT
          count() AS total,
          uniqExactIf(external_call_id, external_call_id != '') AS distinct_total
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const countRows = await countResult.json<{ total: string; distinct_total: string }>();
    const total = Number(countRows[0]?.total ?? 0);
    const distinctTotal = Number(countRows[0]?.distinct_total ?? 0);

    // HAVING repeats argMax(has_converted) while the SELECT aliases that same
    // expression AS has_converted. ClickHouse then nests the alias inside the
    // HAVING aggregate (code 184) and the call list 500s. Prefer the column.
    const dataResult = await this.clickhouse.getClient().query({
      query: filters.raw
        ? `
        SELECT ${PING_SELECT}
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
        ${rawConversion ? `AND ${rawConversion}` : ''}
        ORDER BY ${EVENT_AT_EXPR} DESC
        LIMIT {limit:UInt32}
        OFFSET {offset:UInt32}
      `
        : `
        SELECT ${PING_SELECT_COLLAPSED}
        FROM (
          SELECT * FROM pings
          WHERE tenant_id = {tenant_id:UUID}
            AND external_call_id != ''
          ${clause}
        )
        GROUP BY platform, external_call_id
        ${latestConversion ? `HAVING ${latestConversion}` : ''}
        ORDER BY event_at DESC
        LIMIT {limit:UInt32}
        OFFSET {offset:UInt32}
        SETTINGS prefer_column_name_to_alias = 1
      `,
      query_params: params,
      format: 'JSONEachRow',
    });

    const raw = await dataResult.json<PingRecordRow>();

    return {
      data: raw.map(mapPingRow),
      // Collapsed mode paginates over calls, so the page count must be the distinct count —
      // otherwise Next stays enabled past the last row.
      total: filters.raw ? total : distinctTotal,
      distinct_total: distinctTotal,
      limit: safeLimit,
      offset: safeOffset,
    };
  }

  async exportCsv(tenantId: string, filters: PingListFilters = {}): Promise<string> {
    const { clause, params: filterParams } = buildPingWhereClause(filters);
    const rawConversion = pingConversionPredicate(filters, 'raw');
    const latestConversion = pingConversionPredicate(filters, 'latest');
    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      limit: PingsService.EXPORT_MAX,
      ...filterParams,
    };

    // Export mirrors the screen: one row per call unless the raw stream was asked for.
    const dataResult = await this.clickhouse.getClient().query({
      query: filters.raw
        ? `
        SELECT ${PING_SELECT}
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
        ${rawConversion ? `AND ${rawConversion}` : ''}
        ORDER BY ${EVENT_AT_EXPR} DESC
        LIMIT {limit:UInt32}
      `
        : `
        SELECT ${PING_SELECT_COLLAPSED}
        FROM (
          SELECT * FROM pings
          WHERE tenant_id = {tenant_id:UUID}
            AND external_call_id != ''
          ${clause}
        )
        GROUP BY platform, external_call_id
        ${latestConversion ? `HAVING ${latestConversion}` : ''}
        ORDER BY event_at DESC
        LIMIT {limit:UInt32}
        SETTINGS prefer_column_name_to_alias = 1
      `,
      query_params: params,
      format: 'JSONEachRow',
    });

    const rows = (await dataResult.json<PingRecordRow>()).map(mapPingRow);
    const headers = [
      'occurred_at',
      'received_at',
      'platform',
      'external_call_id',
      'state',
      'event_type',
      'revenue',
      'payout',
      'sync_count',
      'has_converted',
      'conversion_state',
      'ani_hash',
    ];
    const lines = rows.map((row) =>
      [
        row.occurred_at,
        row.received_at,
        row.platform,
        row.external_call_id,
        row.state,
        row.event_type,
        row.revenue,
        row.payout,
        row.sync_count,
        row.has_converted ? 1 : 0,
        row.conversion_state,
        row.ani_hash,
      ]
        .map((v) => `"${String(v).replace(/"/g, '""')}"`)
        .join(','),
    );
    return [headers.join(','), ...lines].join('\n');
  }
}
