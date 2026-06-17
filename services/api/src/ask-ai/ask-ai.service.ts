import { HttpException, HttpStatus, Injectable, Logger } from '@nestjs/common';
import { loadEnv } from '../config/env';

export interface AskResponse {
  query_id: string;
  question: string;
  answer: string;
  intent: string;
  cache_hit: boolean;
  row_count: number;
  tokens_used: number;
}

export interface AskHistoryRecord {
  id: string;
  question: string;
  answer: string;
  intent: string;
  cache_hit: boolean;
  row_count: number;
  tokens_used: number;
  created_at: string;
}

@Injectable()
export class AskAiService {
  private readonly logger = new Logger(AskAiService.name);
  private readonly baseUrl: string;

  constructor() {
    const env = loadEnv();
    this.baseUrl = env.ASK_AI_URL;
  }

  async ask(tenantId: string, question: string): Promise<AskResponse> {
    const response = await fetch(`${this.baseUrl}/v1/ask`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tenant_id: tenantId, question }),
    });
    if (!response.ok) {
      const text = await response.text();
      this.logger.error('ask-ai proxy failed', {
        tenant_id: tenantId,
        status: response.status,
        body: text,
      });
      throw new HttpException(
        `ask-ai service error: ${response.status}`,
        response.status >= 500 ? HttpStatus.BAD_GATEWAY : HttpStatus.UNPROCESSABLE_ENTITY,
      );
    }
    return (await response.json()) as AskResponse;
  }

  async listHistory(
    tenantId: string,
    limit: number,
  ): Promise<{ data: AskHistoryRecord[] }> {
    const safeLimit = Math.min(Math.max(limit, 1), 100);
    const url = new URL(`${this.baseUrl}/v1/history`);
    url.searchParams.set('tenant_id', tenantId);
    url.searchParams.set('limit', String(safeLimit));

    const response = await fetch(url.toString());
    if (!response.ok) {
      const text = await response.text();
      this.logger.error('ask-ai history failed', {
        tenant_id: tenantId,
        status: response.status,
        body: text,
      });
      throw new HttpException(
        `ask-ai history error: ${response.status}`,
        HttpStatus.BAD_GATEWAY,
      );
    }
    return (await response.json()) as { data: AskHistoryRecord[] };
  }
}
