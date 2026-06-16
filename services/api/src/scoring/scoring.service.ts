import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface ScoreRecord {
  tenant_id: string;
  call_id: string;
  platform: string;
  overall_score: number;
  sentiment: string;
  rules_passed: number;
  rules_failed: number;
  created_at: string;
}

export interface TranscriptRecord {
  tenant_id: string;
  call_id: string;
  transcript_text: string;
  duration_seconds: number;
  word_count: number;
  created_at: string;
}

@Injectable()
export class ScoringService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async getScore(tenantId: string, callId: string): Promise<ScoreRecord | null> {
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT tenant_id, call_id, platform, overall_score, sentiment,
               rules_passed, rules_failed,
               formatDateTime(created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS created_at
        FROM call_scores
        WHERE tenant_id = {tenant_id:UUID} AND call_id = {call_id:String}
        ORDER BY created_at DESC
        LIMIT 1
      `,
      query_params: { tenant_id: tenantId, call_id: callId },
      format: 'JSONEachRow',
    });
    const rows = await result.json<ScoreRecord>();
    return rows[0] ?? null;
  }

  async getTranscript(tenantId: string, callId: string): Promise<TranscriptRecord | null> {
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT tenant_id, call_id, transcript_text, duration_seconds, word_count,
               formatDateTime(created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS created_at
        FROM transcripts
        WHERE tenant_id = {tenant_id:UUID} AND call_id = {call_id:String}
        ORDER BY created_at DESC
        LIMIT 1
      `,
      query_params: { tenant_id: tenantId, call_id: callId },
      format: 'JSONEachRow',
    });
    const rows = await result.json<TranscriptRecord>();
    return rows[0] ?? null;
  }

  async listScores(tenantId: string, limit = 50, offset = 0): Promise<ScoreRecord[]> {
    const safeLimit = Math.min(Math.max(limit, 1), 100);
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT tenant_id, call_id, platform, overall_score, sentiment,
               rules_passed, rules_failed,
               formatDateTime(created_at, '%Y-%m-%dT%H:%i:%S.%fZ', 'UTC') AS created_at
        FROM call_scores
        WHERE tenant_id = {tenant_id:UUID}
        ORDER BY created_at DESC
        LIMIT {limit:UInt32} OFFSET {offset:UInt32}
      `,
      query_params: { tenant_id: tenantId, limit: safeLimit, offset: Math.max(offset, 0) },
      format: 'JSONEachRow',
    });
    return result.json<ScoreRecord>();
  }
}
