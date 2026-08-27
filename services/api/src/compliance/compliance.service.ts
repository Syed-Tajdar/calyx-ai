import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { apiSyncExclusionSql } from '../common/demo-rows';

export interface RiskScoreRecord {
  tenant_id: string;
  call_id: string;
  publisher_id: string;
  state: string;
  risk_score: number;
  dnc_match: number;
  attestation_level: string;
  exposure_usd: number;
  created_at: string;
}

export interface ViolationRecord {
  tenant_id: string;
  call_id: string;
  violation_type: string;
  risk_score: number;
  exposure_usd: number;
  created_at: string;
}

export interface ExposureSummary {
  total_exposure_usd: number;
  violation_count: number;
  avg_risk_score: number;
  dnc_match_count: number;
}

@Injectable()
export class ComplianceService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async listRiskScores(
    tenantId: string,
    limit = 50,
    showDemo?: boolean,
  ): Promise<{ data: RiskScoreRecord[] }> {
    const safeLimit = Math.min(Math.max(limit, 1), 100);
    const demo = apiSyncExclusionSql('call_id', showDemo);
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT tenant_id, call_id, publisher_id, state, risk_score, dnc_match,
               attestation_level, exposure_usd,
               formatDateTime(created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS created_at
        FROM compliance_risk_scores
        WHERE tenant_id = {tenant_id:UUID}
          ${demo}
        ORDER BY created_at DESC
        LIMIT {limit:UInt32}
      `,
      query_params: { tenant_id: tenantId, limit: safeLimit },
      format: 'JSONEachRow',
    });
    const data = await result.json<RiskScoreRecord>();
    return { data };
  }

  async listViolations(
    tenantId: string,
    limit = 50,
    showDemo?: boolean,
  ): Promise<{ data: ViolationRecord[] }> {
    const safeLimit = Math.min(Math.max(limit, 1), 100);
    const demo = apiSyncExclusionSql('call_id', showDemo);
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT tenant_id, call_id, violation_type, risk_score, exposure_usd,
               formatDateTime(created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS created_at
        FROM compliance_violations
        WHERE tenant_id = {tenant_id:UUID}
          ${demo}
        ORDER BY created_at DESC
        LIMIT {limit:UInt32}
      `,
      query_params: { tenant_id: tenantId, limit: safeLimit },
      format: 'JSONEachRow',
    });
    const data = await result.json<ViolationRecord>();
    return { data };
  }

  async getExposure(tenantId: string, showDemo?: boolean): Promise<ExposureSummary> {
    const demo = apiSyncExclusionSql('call_id', showDemo);
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          sum(exposure_usd) AS total_exposure_usd,
          countIf(risk_score >= 70) AS violation_count,
          avg(risk_score) AS avg_risk_score,
          countIf(dnc_match = 1) AS dnc_match_count
        FROM compliance_risk_scores
        WHERE tenant_id = {tenant_id:UUID}
          ${demo}
      `,
      query_params: { tenant_id: tenantId },
      format: 'JSONEachRow',
    });
    const rows = await result.json<{
      total_exposure_usd: string;
      violation_count: string;
      avg_risk_score: string;
      dnc_match_count: string;
    }>();
    const row = rows[0] ?? {
      total_exposure_usd: '0',
      violation_count: '0',
      avg_risk_score: '0',
      dnc_match_count: '0',
    };
    return {
      total_exposure_usd: Number(row.total_exposure_usd),
      violation_count: Number(row.violation_count),
      avg_risk_score: Number(row.avg_risk_score),
      dnc_match_count: Number(row.dnc_match_count),
    };
  }
}
