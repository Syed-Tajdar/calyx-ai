import { Injectable } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';

export interface EnrichmentAttribute {
  attribute_key: string;
  attribute_value: string;
  provider: string;
}

@Injectable()
export class EnrichmentApiService {
  constructor(private readonly clickhouse: ClickhouseService) {}

  async getByAniHash(tenantId: string, aniHash: string, limit = 100): Promise<EnrichmentAttribute[]> {
    const safeLimit = Math.min(Math.max(limit, 1), 500);
    const result = await this.clickhouse.getClient().query({
      query: `
        SELECT attribute_key, attribute_value, provider
        FROM enrichment_attrs
        WHERE tenant_id = {tenant_id:UUID} AND ani_hash = {ani_hash:FixedString(64)}
        ORDER BY attribute_key
        LIMIT {limit:UInt32}
      `,
      query_params: { tenant_id: tenantId, ani_hash: aniHash, limit: safeLimit },
      format: 'JSONEachRow',
    });
    return result.json<EnrichmentAttribute>();
  }
}
