import { readFileSync } from 'fs';
import { join } from 'path';
import { Injectable, NotFoundException } from '@nestjs/common';
import { z } from 'zod';
import { PostgresService } from '../database/postgres.service';
import { RedisService } from '../redis/redis.service';
import { loadEnv } from '../config/env';

export interface ProviderCatalogEntry {
  id: string;
  domain: string;
  display_name: string;
  required_secrets: string[];
  optional_config: string[];
  go_adapter: string;
}

export interface ProviderBinding {
  id: string;
  tenant_id: string;
  domain: string;
  provider_id: string;
  config_json: Record<string, unknown>;
  is_active: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

const createBindingSchema = z.object({
  domain: z.string().min(1),
  provider_id: z.string().min(1),
  config_json: z.record(z.unknown()).default({}),
  is_active: z.boolean().optional().default(true),
  priority: z.number().int().optional().default(0),
});

const updateBindingSchema = z.object({
  config_json: z.record(z.unknown()).optional(),
  is_active: z.boolean().optional(),
  priority: z.number().int().optional(),
});

@Injectable()
export class ProvidersService {
  constructor(
    private readonly postgres: PostgresService,
    private readonly redis: RedisService,
  ) {}

  getCatalog(): { version: string; providers: ProviderCatalogEntry[] } {
    const env = loadEnv();
    const catalogPath =
      env.PROVIDER_CATALOG_PATH ??
      join(process.cwd(), '..', '..', 'shared', 'providers', 'catalog.json');
    const raw = readFileSync(catalogPath, 'utf-8');
    return JSON.parse(raw) as { version: string; providers: ProviderCatalogEntry[] };
  }

  async listBindings(tenantId: string): Promise<{ data: ProviderBinding[] }> {
    const data = await this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<ProviderBinding>(
        `SELECT id::text, tenant_id::text, domain, provider_id, config_json,
                is_active, priority, created_at::text, updated_at::text
         FROM tenant_provider_bindings
         WHERE is_active = TRUE
         ORDER BY domain, priority DESC, created_at DESC`,
      );
      return result.rows;
    });
    return { data };
  }

  async createBinding(
    tenantId: string,
    body: unknown,
  ): Promise<ProviderBinding> {
    const parsed = createBindingSchema.parse(body);
    const row = await this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<ProviderBinding>(
        `INSERT INTO tenant_provider_bindings
           (tenant_id, domain, provider_id, config_json, is_active, priority)
         VALUES ($1, $2, $3, $4, $5, $6)
         RETURNING id::text, tenant_id::text, domain, provider_id, config_json,
                   is_active, priority, created_at::text, updated_at::text`,
        [
          tenantId,
          parsed.domain,
          parsed.provider_id,
          JSON.stringify(parsed.config_json),
          parsed.is_active,
          parsed.priority,
        ],
      );
      return result.rows[0];
    });
    await this.invalidateBindingCache(tenantId, parsed.domain);
    return row;
  }

  async updateBinding(
    tenantId: string,
    bindingId: string,
    body: unknown,
  ): Promise<ProviderBinding> {
    const parsed = updateBindingSchema.parse(body);
    let domain = '';
    const row = await this.postgres.withTenant(tenantId, async (client) => {
      const existing = await client.query<{ domain: string }>(
        `SELECT domain FROM tenant_provider_bindings WHERE id = $1`,
        [bindingId],
      );
      if (existing.rows.length === 0) {
        throw new NotFoundException('binding not found');
      }
      domain = existing.rows[0].domain;
      const result = await client.query<ProviderBinding>(
        `UPDATE tenant_provider_bindings
         SET config_json = COALESCE($2, config_json),
             is_active = COALESCE($3, is_active),
             priority = COALESCE($4, priority),
             updated_at = NOW()
         WHERE id = $1
         RETURNING id::text, tenant_id::text, domain, provider_id, config_json,
                   is_active, priority, created_at::text, updated_at::text`,
        [
          bindingId,
          parsed.config_json ? JSON.stringify(parsed.config_json) : null,
          parsed.is_active ?? null,
          parsed.priority ?? null,
        ],
      );
      return result.rows[0];
    });
    await this.invalidateBindingCache(tenantId, domain);
    return row;
  }

  async softDeleteBinding(tenantId: string, bindingId: string): Promise<void> {
    let domain = '';
    await this.postgres.withTenant(tenantId, async (client) => {
      const existing = await client.query<{ domain: string }>(
        `SELECT domain FROM tenant_provider_bindings
         WHERE id = $1::uuid AND is_active = TRUE`,
        [bindingId],
      );
      if (existing.rows.length === 0) {
        throw new NotFoundException('binding not found');
      }
      domain = existing.rows[0].domain;
      await client.query(
        `UPDATE tenant_provider_bindings
         SET is_active = FALSE, updated_at = NOW()
         WHERE id = $1::uuid`,
        [bindingId],
      );
    });
    await this.invalidateBindingCache(tenantId, domain);
  }

  async resolve(
    tenantId: string,
    domain: string,
    traceId?: string,
  ): Promise<Record<string, unknown>> {
    const env = loadEnv();
    const url = `${env.PROVIDERCONFIG_URL}/v1/resolve`;
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        tenant_id: tenantId,
        domain,
        trace_id: traceId ?? 'api-resolve',
      }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`resolve failed: ${text}`);
    }
    return (await res.json()) as Record<string, unknown>;
  }

  private async invalidateBindingCache(tenantId: string, domain: string): Promise<void> {
    const key = `${tenantId}:provider_binding:${domain}`;
    await this.redis.del(key);
  }
}
