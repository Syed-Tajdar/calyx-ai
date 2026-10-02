import {
  BadRequestException,
  ForbiddenException,
  HttpException,
  HttpStatus,
  Injectable,
  NotFoundException,
  ServiceUnavailableException,
} from '@nestjs/common';
import { randomUUID } from 'crypto';
import { PostgresService } from '../database/postgres.service';
import {
  decryptCredentialJson,
  encryptCredentialJson,
} from './credential-crypto';
import {
  apiSyncCredentialFields,
  buildCredentialBlob,
  validateApiSyncCredentials,
} from './platform-credentials';
import {
  API_SYNC_PLATFORMS,
  ConnectionMode,
  SyncMode,
  supportsApiSync,
  supportsWebhook,
} from './platform-connection';

const BACKFILL_COOLDOWN_MS = 5 * 60 * 1000;
const DEFAULT_LOOKBACK_DAYS = 30;

export interface IntegrationRecord {
  id: string;
  platform: string;
  name: string;
  connection_mode: ConnectionMode;
  sync_mode: SyncMode | null;
  webhook_token: string | null;
  is_active: boolean;
  last_sync_at: string | null;
  sync_status: string;
  last_sync_error: string | null;
  created_at: string;
  supports_api_sync: boolean;
  supports_webhook: boolean;
  /** Empty-cursor / backfill window in days (api_sync only; null for webhook). */
  initial_lookback_days: number | null;
  /**
   * Per-tenant write-back gate (default false). Exposed so the UI can explain why Convert /
   * Cancel are unavailable instead of surfacing a 403 after the operator clicks.
   */
  writeback_enabled: boolean;
}

export interface CreateIntegrationInput {
  platform: string;
  name: string;
  connection_mode: ConnectionMode;
  sync_mode?: SyncMode;
  initial_lookback_days?: number;
  webhook_secret?: string;
  api_key?: string;
  api_token?: string;
  account_id?: string;
  company_id?: string;
  subdomain?: string;
  public_key?: string;
  private_key?: string;
  auth_token?: string;
}

export interface TestConnectionResult {
  ok: boolean;
  message: string;
}

interface IntegrationRow {
  id: string;
  platform: string;
  name: string;
  connection_mode: ConnectionMode;
  sync_mode: SyncMode | null;
  webhook_token: string | null;
  is_active: boolean;
  last_sync_at: string | null;
  sync_status: string;
  last_sync_error: string | null;
  created_at: string;
  initial_lookback_days: number | null;
  writeback_enabled?: boolean;
}

@Injectable()
export class IntegrationsService {
  private readonly lastBackfillAt = new Map<string, number>();

  constructor(private readonly postgres: PostgresService) {}

  async list(tenantId: string): Promise<IntegrationRecord[]> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<IntegrationRow>(
        `SELECT id::text, platform, name, connection_mode,
                CASE WHEN connection_mode = 'api_sync' THEN sync_mode ELSE NULL END AS sync_mode,
                CASE WHEN connection_mode = 'webhook' THEN webhook_token ELSE NULL END AS webhook_token,
                is_active, last_sync_at::text, sync_status, last_sync_error,
                created_at::text,
                CASE WHEN connection_mode = 'api_sync' THEN initial_lookback_days ELSE NULL END AS initial_lookback_days,
                COALESCE(writeback_enabled, FALSE) AS writeback_enabled
         FROM integrations
         WHERE deleted_at IS NULL
         ORDER BY created_at DESC
         LIMIT 50`,
      );
      return result.rows.map((row) => this.toRecord(row));
    });
  }

  async create(
    tenantId: string,
    input: CreateIntegrationInput,
  ): Promise<IntegrationRecord> {
    this.validateCreateInput(input);

    const webhookToken =
      input.connection_mode === 'webhook'
        ? randomUUID()
        : `apisync_${randomUUID().replace(/-/g, '')}`;

    let apiBlob: Buffer | null = null;
    if (input.connection_mode === 'api_sync') {
      const creds = buildCredentialBlob(input.platform, {
        api_key: input.api_key,
        api_token: input.api_token,
        account_id: input.account_id,
        company_id: input.company_id,
        subdomain: input.subdomain,
        public_key: input.public_key,
        private_key: input.private_key,
        auth_token: input.auth_token,
      });
      apiBlob = encryptCredentialJson(creds);
    }

    const syncMode: SyncMode =
      input.connection_mode === 'api_sync' ? (input.sync_mode ?? 'mock') : 'mock';
    const lookbackDays =
      input.connection_mode === 'api_sync'
        ? this.normalizeLookbackDays(input.initial_lookback_days)
        : DEFAULT_LOOKBACK_DAYS;

    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<IntegrationRow>(
        `INSERT INTO integrations (
           tenant_id, platform, name, connection_mode, sync_mode,
           webhook_token, webhook_secret, api_credentials_blob, initial_lookback_days
         ) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
         RETURNING id::text, platform, name, connection_mode,
                   CASE WHEN connection_mode = 'api_sync' THEN sync_mode ELSE NULL END AS sync_mode,
                   CASE WHEN connection_mode = 'webhook' THEN webhook_token ELSE NULL END AS webhook_token,
                   is_active, last_sync_at::text, sync_status, last_sync_error,
                   created_at::text,
                   CASE WHEN connection_mode = 'api_sync' THEN initial_lookback_days ELSE NULL END AS initial_lookback_days,
                   COALESCE(writeback_enabled, FALSE) AS writeback_enabled`,
        [
          tenantId,
          input.platform,
          input.name.trim(),
          input.connection_mode,
          syncMode,
          webhookToken,
          input.connection_mode === 'webhook'
            ? (input.webhook_secret?.trim() ?? null)
            : null,
          apiBlob,
          lookbackDays,
        ],
      );
      return this.toRecord(result.rows[0]);
    });
  }

  async testConnection(
    tenantId: string,
    id: string,
  ): Promise<TestConnectionResult> {
    const row = await this.getRow(tenantId, id);
    if (row.connection_mode === 'webhook') {
      return {
        ok: true,
        message:
          'Webhook mode: add the postback URL in your platform account. Use Test after a live ping arrives.',
      };
    }
    if (!row.api_credentials_blob) {
      return { ok: false, message: 'API credentials not configured' };
    }
    const creds = decryptCredentialJson(row.api_credentials_blob);
    try {
      validateApiSyncCredentials(row.platform, creds);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'invalid credentials';
      return { ok: false, message: msg };
    }
    const syncMode = row.sync_mode ?? 'mock';
    if (syncMode === 'mock') {
      return {
        ok: true,
        message: `Test mode: credentials stored for ${row.platform}. Sync worker will pull demo calls.`,
      };
    }
    const syncUrl = process.env.PLATFORM_SYNC_URL ?? 'http://localhost:8088';
    const token = process.env.WRITEBACK_INTERNAL_TOKEN ?? '';
    if (!token) {
      return { ok: false, message: 'Internal sync token is not configured' };
    }
    const res = await fetch(`${syncUrl}/test-auth`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ integration_id: id, tenant_id: tenantId }),
    });
    const body = (await res.json().catch(() => ({}))) as {
      ok?: string;
      message?: string;
    };
    if (!res.ok) {
      return {
        ok: false,
        message: body.message ?? `Platform auth failed (${res.status})`,
      };
    }
    return {
      ok: true,
      message: body.message ?? `Credentials valid for ${row.platform}`,
    };
  }

  async convertWriteback(
    tenantId: string,
    userId: string,
    id: string,
    inboundCallId: string,
    amount: number,
  ): Promise<Record<string, unknown>> {
    return this.callWriteback(tenantId, userId, id, 'convert', inboundCallId, amount);
  }

  async voidWriteback(
    tenantId: string,
    userId: string,
    id: string,
    inboundCallId: string,
  ): Promise<Record<string, unknown>> {
    return this.callWriteback(tenantId, userId, id, 'void', inboundCallId, undefined);
  }

  private async callWriteback(
    tenantId: string,
    userId: string,
    id: string,
    action: 'convert' | 'void',
    inboundCallId: string,
    amount?: number,
  ): Promise<Record<string, unknown>> {
    const row = await this.getRowAny(tenantId, id);
    if (row.connection_mode !== 'api_sync') {
      throw new BadRequestException('Write-back is only available for API key integrations');
    }
    if (row.platform !== 'ringba') {
      throw new BadRequestException('Write-back currently supports Ringba only');
    }
    if (row.sync_mode !== 'real') {
      throw new BadRequestException('Write-back requires Live (sync_mode=real)');
    }
    if (!row.writeback_enabled) {
      throw new ForbiddenException('Write-back is not enabled for this integration');
    }
    const token = process.env.WRITEBACK_INTERNAL_TOKEN ?? '';
    if (!token) {
      throw new ServiceUnavailableException('Write-back internal token is not configured');
    }
    if (action === 'convert' && (amount === undefined || !(amount > 0))) {
      throw new BadRequestException('amount must be a positive number');
    }

    const syncUrl = process.env.PLATFORM_SYNC_URL ?? 'http://localhost:8088';
    const res = await fetch(`${syncUrl}/writeback/${action}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        integration_id: id,
        tenant_id: tenantId,
        user_id: userId,
        inbound_call_id: inboundCallId,
        amount: amount,
      }),
    });
    const body = (await res.json().catch(() => ({}))) as {
      message?: string;
      [key: string]: unknown;
    };
    if (res.status === HttpStatus.ACCEPTED) {
      throw new HttpException(body, HttpStatus.ACCEPTED);
    }
    if (!res.ok) {
      const msg = body.message ?? `Write-back failed (${res.status})`;
      if (res.status === HttpStatus.UNAUTHORIZED) {
        throw new HttpException(msg, HttpStatus.UNAUTHORIZED);
      }
      if (res.status === HttpStatus.FORBIDDEN) {
        throw new ForbiddenException(msg);
      }
      if (res.status === HttpStatus.NOT_FOUND) {
        throw new NotFoundException(msg);
      }
      if (res.status === HttpStatus.BAD_REQUEST) {
        throw new BadRequestException(msg);
      }
      if (res.status === HttpStatus.CONFLICT) {
        throw new HttpException(msg, HttpStatus.CONFLICT);
      }
      throw new ServiceUnavailableException(msg);
    }
    return body;
  }

  async triggerSync(tenantId: string, id: string): Promise<{ status: string }> {
    const row = await this.getRow(tenantId, id);
    if (row.connection_mode !== 'api_sync') {
      throw new BadRequestException('Sync is only available for API key integrations');
    }
    return this.callPlatformSync(tenantId, id);
  }

  async triggerBackfill(
    tenantId: string,
    id: string,
  ): Promise<{ status: string }> {
    const cooldownKey = `${tenantId}:${id}`;
    const last = this.lastBackfillAt.get(cooldownKey) ?? 0;
    if (Date.now() - last < BACKFILL_COOLDOWN_MS) {
      throw new HttpException(
        'Backfill rate limited — try again in a few minutes',
        HttpStatus.TOO_MANY_REQUESTS,
      );
    }

    const row = await this.getRow(tenantId, id);
    if (row.connection_mode !== 'api_sync') {
      throw new BadRequestException('Backfill is only available for API key integrations');
    }
    if (row.sync_status === 'syncing') {
      throw new HttpException('Sync already in progress', HttpStatus.TOO_MANY_REQUESTS);
    }

    await this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query(
        `UPDATE integrations
         SET sync_cursor = NULL,
             last_sync_at = NULL,
             sync_status = 'idle',
             last_sync_error = NULL,
             updated_at = NOW()
         WHERE id = $1::uuid AND tenant_id = $2::uuid
           AND connection_mode = 'api_sync' AND is_active = TRUE`,
        [id, tenantId],
      );
      if (result.rowCount === 0) {
        throw new NotFoundException('API sync integration not found');
      }
    });

    this.lastBackfillAt.set(cooldownKey, Date.now());
    await this.callPlatformSync(tenantId, id);
    return { status: 'backfill_triggered' };
  }

  async updateSyncMode(
    tenantId: string,
    id: string,
    syncMode: SyncMode,
    initialLookbackDays?: number,
  ): Promise<IntegrationRecord> {
    if (syncMode !== 'mock' && syncMode !== 'real') {
      throw new BadRequestException('sync_mode must be mock or real');
    }
    const lookback =
      initialLookbackDays !== undefined
        ? this.normalizeLookbackDays(initialLookbackDays)
        : undefined;

    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<IntegrationRow>(
        lookback !== undefined
          ? `UPDATE integrations
             SET sync_mode = $3, initial_lookback_days = $4, updated_at = NOW()
             WHERE id = $1::uuid AND tenant_id = $2::uuid
               AND connection_mode = 'api_sync' AND is_active = TRUE
             RETURNING id::text, platform, name, connection_mode,
                       sync_mode, NULL AS webhook_token,
                       is_active, last_sync_at::text, sync_status, last_sync_error,
                       created_at::text, initial_lookback_days,
                       COALESCE(writeback_enabled, FALSE) AS writeback_enabled`
          : `UPDATE integrations
             SET sync_mode = $3, updated_at = NOW()
             WHERE id = $1::uuid AND tenant_id = $2::uuid
               AND connection_mode = 'api_sync' AND is_active = TRUE
             RETURNING id::text, platform, name, connection_mode,
                       sync_mode, NULL AS webhook_token,
                       is_active, last_sync_at::text, sync_status, last_sync_error,
                       created_at::text, initial_lookback_days,
                       COALESCE(writeback_enabled, FALSE) AS writeback_enabled`,
        lookback !== undefined
          ? [id, tenantId, syncMode, lookback]
          : [id, tenantId, syncMode],
      );
      if (result.rowCount === 0) {
        throw new NotFoundException('API sync integration not found');
      }
      return this.toRecord(result.rows[0]);
    });
  }

  async softDelete(tenantId: string, id: string, userId?: string): Promise<void> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query(
        `UPDATE integrations
         SET is_active = FALSE, deleted_at = NOW(), updated_at = NOW()
         WHERE id = $1::uuid AND tenant_id = $2::uuid AND deleted_at IS NULL`,
        [id, tenantId],
      );
      if (result.rowCount === 0) {
        throw new NotFoundException('Integration not found');
      }
      await this.insertSettingsAudit(client, tenantId, userId, id, 'delete');
    });
  }

  /** Pause (is_active false) or resume (is_active true). Does not change sync_mode. */
  async setActive(
    tenantId: string,
    userId: string,
    id: string,
    active: boolean,
  ): Promise<IntegrationRecord> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<IntegrationRow>(
        `UPDATE integrations
         SET is_active = $3, updated_at = NOW()
         WHERE id = $1::uuid AND tenant_id = $2::uuid AND deleted_at IS NULL
         RETURNING id::text, platform, name, connection_mode,
                   CASE WHEN connection_mode = 'api_sync' THEN sync_mode ELSE NULL END AS sync_mode,
                   CASE WHEN connection_mode = 'webhook' THEN webhook_token ELSE NULL END AS webhook_token,
                   is_active, last_sync_at::text, sync_status, last_sync_error,
                   created_at::text,
                   CASE WHEN connection_mode = 'api_sync' THEN initial_lookback_days ELSE NULL END AS initial_lookback_days,
                   COALESCE(writeback_enabled, FALSE) AS writeback_enabled`,
        [id, tenantId, active],
      );
      if (result.rowCount === 0) {
        throw new NotFoundException('Integration not found');
      }
      await this.insertSettingsAudit(client, tenantId, userId, id, active ? 'resume' : 'pause');
      return this.toRecord(result.rows[0]);
    });
  }

  private async insertSettingsAudit(
    client: { query: (sql: string, params: unknown[]) => Promise<unknown> },
    tenantId: string,
    userId: string | undefined,
    integrationId: string,
    action: 'pause' | 'resume' | 'delete',
  ): Promise<void> {
    await client.query(
      `INSERT INTO integration_settings_audit (tenant_id, user_id, integration_id, action)
       VALUES ($1::uuid, NULLIF($2, '')::uuid, $3::uuid, $4)`,
      [tenantId, userId ?? '', integrationId, action],
    );
  }

  listPlatformOptions(): {
    platform: string;
    supports_webhook: boolean;
    supports_api_sync: boolean;
    api_sync_fields: ReturnType<typeof apiSyncCredentialFields>;
  }[] {
    return API_SYNC_PLATFORMS.map((platform) => ({
      platform,
      supports_webhook: supportsWebhook(platform),
      supports_api_sync: supportsApiSync(platform),
      api_sync_fields: apiSyncCredentialFields(platform),
    }));
  }

  private async callPlatformSync(
    tenantId: string,
    id: string,
  ): Promise<{ status: string }> {
    const syncUrl = process.env.PLATFORM_SYNC_URL ?? 'http://localhost:8088';
    const token = process.env.WRITEBACK_INTERNAL_TOKEN ?? '';
    if (!token) {
      throw new ServiceUnavailableException('Internal sync token is not configured');
    }
    const res = await fetch(`${syncUrl}/sync`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ integration_id: id, tenant_id: tenantId }),
    });
    if (res.status === 401) {
      throw new ServiceUnavailableException('Platform sync unauthorized (internal token)');
    }
    if (!res.ok) {
      throw new ServiceUnavailableException(
        `Platform sync service unavailable (${res.status})`,
      );
    }
    return { status: 'sync_triggered' };
  }

  private normalizeLookbackDays(raw?: number): number {
    if (raw === undefined || raw === null || Number.isNaN(raw)) {
      return DEFAULT_LOOKBACK_DAYS;
    }
    const days = Math.trunc(raw);
    if (days < 1 || days > 365) {
      throw new BadRequestException('initial_lookback_days must be between 1 and 365');
    }
    return days;
  }

  private validateCreateInput(input: CreateIntegrationInput): void {
    if (!input.name?.trim()) {
      throw new BadRequestException('name is required');
    }
    if (!supportsWebhook(input.platform) && !supportsApiSync(input.platform)) {
      throw new BadRequestException(`unsupported platform: ${input.platform}`);
    }
    if (input.connection_mode === 'webhook') {
      if (!supportsWebhook(input.platform)) {
        throw new BadRequestException(
          `${input.platform} does not support webhook mode`,
        );
      }
      if (input.platform !== 'trackdrive' && !input.webhook_secret?.trim()) {
        throw new BadRequestException('webhook_secret is required for webhook mode');
      }
    }
    if (input.connection_mode === 'api_sync') {
      if (!supportsApiSync(input.platform)) {
        throw new BadRequestException(
          `${input.platform} does not support API key mode`,
        );
      }
      try {
        buildCredentialBlob(input.platform, {
          api_key: input.api_key,
          api_token: input.api_token,
          account_id: input.account_id,
          company_id: input.company_id,
          subdomain: input.subdomain,
          public_key: input.public_key,
          private_key: input.private_key,
          auth_token: input.auth_token,
        });
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : 'invalid credentials';
        throw new BadRequestException(msg);
      }
      if (input.sync_mode && input.sync_mode !== 'mock' && input.sync_mode !== 'real') {
        throw new BadRequestException('sync_mode must be mock or real');
      }
      if (input.initial_lookback_days !== undefined) {
        this.normalizeLookbackDays(input.initial_lookback_days);
      }
    }
  }

  private async getRow(
    tenantId: string,
    id: string,
  ): Promise<{
    connection_mode: ConnectionMode;
    platform: string;
    sync_mode: SyncMode | null;
    sync_status: string;
    api_credentials_blob: Buffer | null;
  }> {
    return this.getIntegrationRow(tenantId, id, true);
  }

  /** Includes paused (is_active=false) rows — write-back only. */
  private async getRowAny(
    tenantId: string,
    id: string,
  ): Promise<{
    connection_mode: ConnectionMode;
    platform: string;
    sync_mode: SyncMode | null;
    sync_status: string;
    api_credentials_blob: Buffer | null;
    writeback_enabled: boolean;
  }> {
    return this.getIntegrationRow(tenantId, id, false);
  }

  private async getIntegrationRow(
    tenantId: string,
    id: string,
    activeOnly: boolean,
  ): Promise<{
    connection_mode: ConnectionMode;
    platform: string;
    sync_mode: SyncMode | null;
    sync_status: string;
    api_credentials_blob: Buffer | null;
    writeback_enabled: boolean;
  }> {
    return this.postgres.withTenant(tenantId, async (client) => {
      const result = await client.query<{
        connection_mode: ConnectionMode;
        platform: string;
        sync_mode: SyncMode | null;
        sync_status: string;
        api_credentials_blob: Buffer | null;
        writeback_enabled: boolean;
      }>(
        activeOnly
          ? `SELECT connection_mode, platform, sync_mode, sync_status, api_credentials_blob,
                    COALESCE(writeback_enabled, FALSE) AS writeback_enabled
             FROM integrations WHERE id = $1::uuid AND is_active = TRUE`
          : `SELECT connection_mode, platform, sync_mode, sync_status, api_credentials_blob,
                    COALESCE(writeback_enabled, FALSE) AS writeback_enabled
             FROM integrations WHERE id = $1::uuid`,
        [id],
      );
      if (result.rowCount === 0) {
        throw new NotFoundException('Integration not found');
      }
      return result.rows[0];
    });
  }

  private toRecord(row: IntegrationRow): IntegrationRecord {
    return {
      id: row.id,
      platform: row.platform,
      name: row.name,
      connection_mode: row.connection_mode,
      sync_mode: row.sync_mode,
      webhook_token: row.webhook_token,
      is_active: row.is_active,
      last_sync_at: row.last_sync_at,
      sync_status: row.sync_status ?? 'idle',
      last_sync_error: row.last_sync_error,
      created_at: row.created_at,
      supports_api_sync: supportsApiSync(row.platform),
      supports_webhook: supportsWebhook(row.platform),
      initial_lookback_days:
        row.connection_mode === 'api_sync'
          ? (row.initial_lookback_days ?? DEFAULT_LOOKBACK_DAYS)
          : null,
      writeback_enabled: row.writeback_enabled ?? false,
    };
  }
}
