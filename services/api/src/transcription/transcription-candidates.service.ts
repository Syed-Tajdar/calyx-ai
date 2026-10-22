import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { testDataExclusionSql } from '../common/demo-rows';

export type CandidateReason = 'converted' | 'should_convert' | 'archive_only';

export interface Candidate {
  platform: 'ringba' | 'retreaver';
  external_call_id: string;
  reason: CandidateReason;
  occurred_at: string; // ISO
  duration_seconds: number;
  campaign_name: string;
}

export interface CandidateQuery {
  tenantId: string;
  /** Calls whose LATEST version was received after this (the "became converted" scan). */
  receivedSince?: Date;
  /** Calls that happened in [from, to) (the backfill). */
  eventFrom?: Date;
  eventTo?: Date;
  minSeconds: number;
  shouldConvert: boolean;
  /** Also return recorded calls that are not transcribed (admin switch "Archive all recorded calls"). */
  archiveAll: boolean;
  limit: number;
}

const ts = (d: Date) => Math.floor(d.getTime() / 1000);

/**
 * Which calls to queue. Same per-call view as the dashboard: one row per (platform, external_call_id),
 * latest version wins; Ringba + Retreaver only; test rows excluded.
 * - converted = flag AND revenue > 0 (the dashboard's definition)
 * - should_convert (only with TRANSCRIBE_SHOULD_CONVERT) = not converted AND (revenue > 0 OR duration ≥
 *   the buyer's required duration) — the Pings "should have converted" filter
 * - never: platform duplicates, calls shorter than TRANSCRIBE_MIN_SECONDS, calls known to have no recording
 * Every value is a query parameter.
 */
@Injectable()
export class TranscriptionCandidatesService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async find(q: CandidateQuery): Promise<Candidate[]> {
    const since = q.receivedSince ? ts(q.receivedSince) : 0;
    const from = q.eventFrom ? ts(q.eventFrom) : 0;
    const to = q.eventTo ? ts(q.eventTo) : 4_102_444_800; // 2100-01-01
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT platform, external_call_id, reason, toString(event_at) AS occurred_at, dur, campaign_name
        FROM (
          SELECT
            platform, external_call_id, event_at, dur, campaign_name,
            multiIf(conv = 1 AND rev > 0, 'converted',
                    {should_convert:UInt8} = 1 AND conv = 0 AND (rev > 0 OR (req IS NOT NULL AND dur >= req)), 'should_convert',
                    {archive_all:UInt8} = 1 AND rk = 1 AND hr = 1, 'archive_only',
                    '') AS reason
          FROM (
            SELECT
              platform, external_call_id,
              argMax(revenue, received_at) AS rev,
              argMax(has_converted, received_at) AS conv,
              argMax(platform_duplicate, received_at) AS dup,
              argMax(duration_seconds, received_at) AS dur,
              argMax(required_duration_seconds, received_at) AS req,
              argMax(has_recording, received_at) AS hr,
              argMax(recording_known, received_at) AS rk,
              argMax(campaign_name, received_at) AS campaign_name,
              max(received_at) AS last_received,
              max(${EVENT_AT_EXPR}) AS event_at
            FROM pings
            WHERE tenant_id = {tenant_id:UUID}
              AND external_call_id != ''
              ${testDataExclusionSql('external_call_id')}
              AND platform IN ('ringba', 'retreaver')
            GROUP BY platform, external_call_id
          )
          WHERE dup = 0
            AND dur >= {min_seconds:UInt32}
            AND NOT (rk = 1 AND hr = 0)
            AND toUInt32(toUnixTimestamp(last_received)) >= {since:UInt32}
            AND toUInt32(toUnixTimestamp(event_at)) >= {from:UInt32}
            AND toUInt32(toUnixTimestamp(event_at)) < {to:UInt32}
        )
        WHERE reason != ''
        ORDER BY event_at
        LIMIT {limit:UInt32}`,
      query_params: {
        tenant_id: q.tenantId,
        should_convert: q.shouldConvert ? 1 : 0,
        archive_all: q.archiveAll ? 1 : 0,
        min_seconds: Math.max(0, Math.floor(q.minSeconds)),
        since,
        from,
        to,
        limit: q.limit,
      },
      format: 'JSONEachRow',
    });
    const rows = (await result.json()) as Array<{
      platform: string;
      external_call_id: string;
      reason: CandidateReason;
      occurred_at: string;
      dur: number | string;
      campaign_name: string;
    }>;
    return rows
      .filter((r) => r.platform === 'ringba' || r.platform === 'retreaver')
      .map((r) => ({
        platform: r.platform as 'ringba' | 'retreaver',
        external_call_id: r.external_call_id,
        reason: r.reason,
        occurred_at: new Date(`${r.occurred_at.replace(' ', 'T')}Z`).toISOString(),
        duration_seconds: Number(r.dur) || 0,
        campaign_name: r.campaign_name ?? '',
      }));
  }

  /**
   * One call's facts for the "Transcribe" button (latest version): when it happened (the Ringba lookup
   * window), its duration (the cap estimate) and its campaign (the Spanish rule). Null when unknown.
   */
  async one(tenantId: string, platform: 'ringba' | 'retreaver', externalCallId: string): Promise<Omit<Candidate, 'reason'> | null> {
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT toString(max(${EVENT_AT_EXPR})) AS occurred_at,
               argMax(duration_seconds, received_at) AS dur,
               argMax(campaign_name, received_at) AS campaign_name
        FROM pings
        WHERE tenant_id = {tenant_id:UUID} AND platform = {platform:String} AND external_call_id = {call_id:String}
        HAVING count() > 0`,
      query_params: { tenant_id: tenantId, platform, call_id: externalCallId },
      format: 'JSONEachRow',
    });
    const rows = (await result.json()) as Array<{ occurred_at: string; dur: number | string; campaign_name: string }>;
    const r = rows[0];
    if (!r) return null;
    return {
      platform,
      external_call_id: externalCallId,
      occurred_at: new Date(`${r.occurred_at.replace(' ', 'T')}Z`).toISOString(),
      duration_seconds: Number(r.dur) || 0,
      campaign_name: r.campaign_name ?? '',
    };
  }
}
