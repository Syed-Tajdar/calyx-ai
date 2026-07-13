import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { buildPingWhereClause, PingListFilters } from './pings.filters';

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
  total: number;
  limit: number;
  offset: number;
}

/** Prefer platform source time; fall back to ingestion time. */
export { EVENT_AT_EXPR } from '../common/ping-event-time';

const PING_SELECT = `
  tenant_id, platform, external_call_id, ani_hash,
  campaign_id, publisher_id, target_id, state,
  event_type, revenue, payout, is_duplicate,
  formatDateTime(received_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS received_at_iso,
  formatDateTime(${EVENT_AT_EXPR}, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS occurred_at_iso
`;

interface PingRecordRow extends Omit<PingRecord, 'received_at' | 'occurred_at'> {
  received_at_iso: string;
  occurred_at_iso: string;
}

function mapPingRow(row: PingRecordRow): PingRecord {
  const { received_at_iso, occurred_at_iso, ...rest } = row;
  return {
    ...rest,
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
    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      limit: safeLimit,
      offset: safeOffset,
      ...filterParams,
    };

    const countResult = await this.clickhouse.getClient().query({
      query: `
        SELECT count() AS total
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
      `,
      query_params: params,
      format: 'JSONEachRow',
    });
    const countRows = await countResult.json<{ total: string }>();
    const total = Number(countRows[0]?.total ?? 0);

    const dataResult = await this.clickhouse.getClient().query({
      query: `
        SELECT ${PING_SELECT}
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
        ORDER BY ${EVENT_AT_EXPR} DESC
        LIMIT {limit:UInt32}
        OFFSET {offset:UInt32}
      `,
      query_params: params,
      format: 'JSONEachRow',
    });

    const raw = await dataResult.json<PingRecordRow>();

    return {
      data: raw.map(mapPingRow),
      total,
      limit: safeLimit,
      offset: safeOffset,
    };
  }

  async exportCsv(tenantId: string, filters: PingListFilters = {}): Promise<string> {
    const { clause, params: filterParams } = buildPingWhereClause(filters);
    const params: Record<string, string | number> = {
      tenant_id: tenantId,
      limit: PingsService.EXPORT_MAX,
      ...filterParams,
    };

    const dataResult = await this.clickhouse.getClient().query({
      query: `
        SELECT ${PING_SELECT}
        FROM pings
        WHERE tenant_id = {tenant_id:UUID}
        ${clause}
        ORDER BY ${EVENT_AT_EXPR} DESC
        LIMIT {limit:UInt32}
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
      'is_duplicate',
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
        row.is_duplicate,
        row.ani_hash,
      ]
        .map((v) => `"${String(v).replace(/"/g, '""')}"`)
        .join(','),
    );
    return [headers.join(','), ...lines].join('\n');
  }
}
