import { ConflictException, Injectable, NotFoundException } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';
import type { CreateScorecardInput, UpdateScorecardInput } from './scorecards.schemas';

function isUniqueViolation(error: unknown): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    (error as { code: string }).code === '23505'
  );
}

export interface ScorecardRecord {
  id: string;
  tenant_id: string;
  name: string;
  vertical: string;
  rules: unknown[];
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

@Injectable()
export class ScorecardsService {
  constructor(private readonly postgres: PostgresService) {}

  async list(
    tenantId: string,
    limit: number,
    offset: number,
  ): Promise<{ data: ScorecardRecord[]; total: number }> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const countResult = await client.query<{ count: string }>(
        `SELECT COUNT(*)::text AS count FROM scorecards WHERE is_active = TRUE`,
      );
      const result = await client.query<ScorecardRecord>(
        `SELECT id::text, tenant_id::text, name, vertical, rules, is_active,
                created_at::text, updated_at::text
         FROM scorecards
         WHERE is_active = TRUE
         ORDER BY created_at DESC
         LIMIT $1 OFFSET $2`,
        [limit, offset],
      );
      return {
        data: result.rows,
        total: Number(countResult.rows[0]?.count ?? 0),
      };
    });
  }

  async getById(tenantId: string, id: string): Promise<ScorecardRecord> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<ScorecardRecord>(
        `SELECT id::text, tenant_id::text, name, vertical, rules, is_active,
                created_at::text, updated_at::text
         FROM scorecards
         WHERE id = $1::uuid AND is_active = TRUE`,
        [id],
      );
      const row = result.rows[0];
      if (!row) {
        throw new NotFoundException('Scorecard not found');
      }
      return row;
    });
  }

  async create(tenantId: string, input: CreateScorecardInput): Promise<ScorecardRecord> {
    return this.postgres.withTenant(tenantId, async (client) => {
      try {
        const result = await client.query<ScorecardRecord>(
          `INSERT INTO scorecards (tenant_id, name, vertical, rules, is_active)
           VALUES ($1, $2, $3, $4::jsonb, $5)
           RETURNING id::text, tenant_id::text, name, vertical, rules, is_active,
                     created_at::text, updated_at::text`,
          [
            tenantId,
            input.name,
            input.vertical,
            JSON.stringify(input.rules),
            input.is_active ?? true,
          ],
        );
        return result.rows[0];
      } catch (error: unknown) {
        if (isUniqueViolation(error)) {
          throw new ConflictException('A scorecard with this name already exists');
        }
        throw error;
      }
    });
  }

  async update(
    tenantId: string,
    id: string,
    input: UpdateScorecardInput,
  ): Promise<ScorecardRecord> {
    return this.postgres.withTenant(tenantId, async (client) => {
      try {
        const result = await client.query<ScorecardRecord>(
          `UPDATE scorecards
           SET name = COALESCE($3, name),
               vertical = COALESCE($4, vertical),
               rules = COALESCE($5::jsonb, rules),
               is_active = COALESCE($6, is_active),
               updated_at = NOW()
           WHERE id = $1::uuid AND tenant_id = $2::uuid AND is_active = TRUE
           RETURNING id::text, tenant_id::text, name, vertical, rules, is_active,
                     created_at::text, updated_at::text`,
          [
            id,
            tenantId,
            input.name ?? null,
            input.vertical ?? null,
            input.rules ? JSON.stringify(input.rules) : null,
            input.is_active ?? null,
          ],
        );
        const row = result.rows[0];
        if (!row) {
          throw new NotFoundException('Scorecard not found');
        }
        return row;
      } catch (error: unknown) {
        if (isUniqueViolation(error)) {
          throw new ConflictException('A scorecard with this name already exists');
        }
        throw error;
      }
    });
  }

  async softDelete(tenantId: string, id: string): Promise<void> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query(
        `UPDATE scorecards
         SET is_active = FALSE, updated_at = NOW()
         WHERE id = $1::uuid AND tenant_id = $2::uuid AND is_active = TRUE`,
        [id, tenantId],
      );
      if (result.rowCount === 0) {
        throw new NotFoundException('Scorecard not found');
      }
    });
  }
}
