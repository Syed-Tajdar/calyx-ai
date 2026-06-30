import {
  BadRequestException,
  Injectable,
} from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface CallDeepViewPing {
  platform: string;
  publisher_id: string;
  campaign_id: string;
  target_id: string;
  state: string;
  event_type: string;
  revenue: number;
  payout: number;
  ani_hash: string;
  received_at: string;
}

export interface CallDeepViewScoring {
  overall_score: number;
  sentiment: string;
  rules_passed: number;
  rules_failed: number;
  duration_seconds: number;
  word_count: number;
  scored_at: string;
}

export interface CallDeepViewCompliance {
  risk_score: number;
  attestation_level: string;
  exposure_usd: number;
  dnc_match: number;
  scored_at: string;
}

export interface CallDeepViewAttribution {
  platform: string;
  status: string;
  click_id: string;
  revenue: number;
  http_status: number;
  fired_at: string;
}

export interface CallDeepViewEnrichment {
  attribute_count: number;
  sample_attributes: Array<{
    attribute_key: string;
    attribute_value: string;
    provider: string;
  }>;
}

export interface CallDeepView {
  call_id: string;
  tenant_id: string;
  ping: CallDeepViewPing | null;
  scoring: CallDeepViewScoring | null;
  compliance: CallDeepViewCompliance | null;
  attribution: CallDeepViewAttribution | null;
  enrichment: CallDeepViewEnrichment | null;
}

interface DeepViewRow {
  ping_platform: string;
  call_id: string;
  ping_publisher_id: string;
  campaign_id: string;
  target_id: string;
  state: string;
  event_type: string;
  ping_revenue: string;
  ping_payout: string;
  ani_hash: string;
  ping_received_at: string;
  overall_score: string | null;
  sentiment: string | null;
  rules_passed: string | null;
  rules_failed: string | null;
  score_created_at: string | null;
  duration_seconds: string | null;
  word_count: string | null;
  risk_score: string | null;
  attestation_level: string | null;
  exposure_usd: string | null;
  dnc_match: string | null;
  compliance_created_at: string | null;
  attribution_platform: string | null;
  attribution_status: string | null;
  click_id: string | null;
  attribution_revenue: string | null;
  http_status: string | null;
  attribution_fired_at: string | null;
}

@Injectable()
export class DeepQueryService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async getCallDeepView(tenantId: string, callId: string): Promise<CallDeepView> {
    const trimmedCallId = callId.trim();
    if (!trimmedCallId) {
      throw new BadRequestException('callId is required');
    }

    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          p.platform AS ping_platform,
          p.external_call_id AS call_id,
          p.publisher_id AS ping_publisher_id,
          p.campaign_id,
          p.target_id,
          p.state,
          p.event_type,
          toString(p.revenue) AS ping_revenue,
          toString(p.payout) AS ping_payout,
          p.ani_hash,
          formatDateTime(p.received_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS ping_received_at,
          toString(cs.overall_score) AS overall_score,
          cs.sentiment,
          toString(cs.rules_passed) AS rules_passed,
          toString(cs.rules_failed) AS rules_failed,
          formatDateTime(cs.created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS score_created_at,
          toString(t.duration_seconds) AS duration_seconds,
          toString(t.word_count) AS word_count,
          toString(cr.risk_score) AS risk_score,
          cr.attestation_level,
          toString(cr.exposure_usd) AS exposure_usd,
          toString(cr.dnc_match) AS dnc_match,
          formatDateTime(cr.created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS compliance_created_at,
          af.platform AS attribution_platform,
          af.status AS attribution_status,
          af.click_id,
          toString(af.revenue) AS attribution_revenue,
          toString(af.http_status) AS http_status,
          formatDateTime(af.fired_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS attribution_fired_at
        FROM (
          SELECT *
          FROM pings
          WHERE tenant_id = {tenant_id:UUID}
            AND external_call_id = {call_id:String}
          ORDER BY received_at DESC
          LIMIT 1
        ) AS p
        LEFT JOIN (
          SELECT *
          FROM call_scores
          WHERE tenant_id = {tenant_id:UUID} AND call_id = {call_id:String}
          ORDER BY created_at DESC
          LIMIT 1
        ) AS cs ON 1 = 1
        LEFT JOIN (
          SELECT *
          FROM transcripts
          WHERE tenant_id = {tenant_id:UUID} AND call_id = {call_id:String}
          ORDER BY created_at DESC
          LIMIT 1
        ) AS t ON 1 = 1
        LEFT JOIN (
          SELECT *
          FROM compliance_risk_scores
          WHERE tenant_id = {tenant_id:UUID} AND call_id = {call_id:String}
          ORDER BY created_at DESC
          LIMIT 1
        ) AS cr ON 1 = 1
        LEFT JOIN (
          SELECT *
          FROM attribution_fired_events
          WHERE tenant_id = {tenant_id:UUID} AND call_id = {call_id:String}
          ORDER BY fired_at DESC
          LIMIT 1
        ) AS af ON 1 = 1
      `,
      query_params: { tenant_id: tenantId, call_id: trimmedCallId },
      format: 'JSONEachRow',
    });

    const rows = await result.json<DeepViewRow>();
    const row = rows[0];
    if (!row) {
      return {
        call_id: trimmedCallId,
        tenant_id: tenantId,
        ping: null,
        scoring: null,
        compliance: null,
        attribution: null,
        enrichment: null,
      };
    }

    const view: CallDeepView = {
      call_id: row.call_id,
      tenant_id: tenantId,
      ping: {
        platform: row.ping_platform,
        publisher_id: row.ping_publisher_id,
        campaign_id: row.campaign_id,
        target_id: row.target_id,
        state: row.state,
        event_type: row.event_type,
        revenue: Number(row.ping_revenue),
        payout: Number(row.ping_payout),
        ani_hash: row.ani_hash,
        received_at: row.ping_received_at,
      },
      scoring: row.overall_score
        ? {
            overall_score: Number(row.overall_score),
            sentiment: row.sentiment ?? 'neutral',
            rules_passed: Number(row.rules_passed ?? 0),
            rules_failed: Number(row.rules_failed ?? 0),
            duration_seconds: Number(row.duration_seconds ?? 0),
            word_count: Number(row.word_count ?? 0),
            scored_at: row.score_created_at ?? '',
          }
        : null,
      compliance: row.risk_score
        ? {
            risk_score: Number(row.risk_score),
            attestation_level: row.attestation_level ?? 'unknown',
            exposure_usd: Number(row.exposure_usd ?? 0),
            dnc_match: Number(row.dnc_match ?? 0),
            scored_at: row.compliance_created_at ?? '',
          }
        : null,
      attribution: row.attribution_platform
        ? {
            platform: row.attribution_platform,
            status: row.attribution_status ?? 'unknown',
            click_id: row.click_id ?? '',
            revenue: Number(row.attribution_revenue ?? 0),
            http_status: Number(row.http_status ?? 0),
            fired_at: row.attribution_fired_at ?? '',
          }
        : null,
      enrichment: null,
    };

    if (row.ani_hash) {
      view.enrichment = await this.loadEnrichmentSummary(tenantId, row.ani_hash);
    }

    return view;
  }

  private async loadEnrichmentSummary(
    tenantId: string,
    aniHash: string,
  ): Promise<CallDeepViewEnrichment> {
    const countResult = await this.clickhouse.getClient().query({
      query: `
        SELECT count() AS total
        FROM enrichment_attrs
        WHERE tenant_id = {tenant_id:UUID} AND ani_hash = {ani_hash:FixedString(64)}
      `,
      query_params: { tenant_id: tenantId, ani_hash: aniHash },
      format: 'JSONEachRow',
    });
    const countRows = await countResult.json<{ total: string }>();
    const total = Number(countRows[0]?.total ?? 0);

    const sampleResult = await this.clickhouse.getClient().query({
      query: `
        SELECT attribute_key, attribute_value, provider
        FROM enrichment_attrs
        WHERE tenant_id = {tenant_id:UUID} AND ani_hash = {ani_hash:FixedString(64)}
        ORDER BY attribute_key
        LIMIT 5
      `,
      query_params: { tenant_id: tenantId, ani_hash: aniHash },
      format: 'JSONEachRow',
    });
    const sample = await sampleResult.json<{
      attribute_key: string;
      attribute_value: string;
      provider: string;
    }>();

    return {
      attribute_count: total,
      sample_attributes: sample,
    };
  }
}
