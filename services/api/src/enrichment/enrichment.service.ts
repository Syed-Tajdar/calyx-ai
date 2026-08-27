import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface EnrichmentAttribute {
  attribute_key: string;
  attribute_value: string;
  provider: string;
}

export interface EnrichmentListResult {
  data: EnrichmentAttribute[];
  /** Unique attribute keys (meaningful count — not raw MergeTree row count). */
  total: number;
  /** Max rows returned in `data` (server-side LIMIT on unique keys). */
  limit: number;
}

@Injectable()
export class EnrichmentApiService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async getByAniHash(
    tenantId: string,
    aniHash: string,
    limit = 100,
  ): Promise<EnrichmentListResult> {
    const safeLimit = Math.min(Math.max(limit, 1), 500);

    // Meaningful count = unique keys. Raw count() is inflated by republish duplicates.
    const countResult = await this.clickhouse.getClient().query({
      query: `
        SELECT uniqExact(attribute_key) AS total
        FROM enrichment_attrs
        WHERE tenant_id = {tenant_id:UUID} AND ani_hash = {ani_hash:FixedString(64)}
      `,
      query_params: { tenant_id: tenantId, ani_hash: aniHash },
      format: 'JSONEachRow',
    });
    const countRows = await countResult.json<{ total: string }>();
    const total = Number(countRows[0]?.total ?? 0);

    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT
          attribute_key,
          argMax(attribute_value, created_at) AS attribute_value,
          argMax(provider, created_at) AS provider
        FROM enrichment_attrs
        WHERE tenant_id = {tenant_id:UUID} AND ani_hash = {ani_hash:FixedString(64)}
        GROUP BY attribute_key
        ORDER BY attribute_key
        LIMIT {limit:UInt32}
      `,
      query_params: { tenant_id: tenantId, ani_hash: aniHash, limit: safeLimit },
      format: 'JSONEachRow',
    });
    const data = await result.json<EnrichmentAttribute>();

    return { data, total, limit: safeLimit };
  }
}
