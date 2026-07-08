import {

  BadRequestException,

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

}



export interface CreateIntegrationInput {

  platform: string;

  name: string;

  connection_mode: ConnectionMode;

  sync_mode?: SyncMode;

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

}



@Injectable()

export class IntegrationsService {

  constructor(private readonly postgres: PostgresService) {}



  async list(tenantId: string): Promise<IntegrationRecord[]> {

    return this.postgres.withTenant(tenantId, async (client) => {

      const result = await client.query<IntegrationRow>(

        `SELECT id::text, platform, name, connection_mode,
                CASE WHEN connection_mode = 'api_sync' THEN sync_mode ELSE NULL END AS sync_mode,
                CASE WHEN connection_mode = 'webhook' THEN webhook_token ELSE NULL END AS webhook_token,
                is_active, last_sync_at::text, sync_status, last_sync_error,
                created_at::text

         FROM integrations

         WHERE is_active = TRUE

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

    return this.postgres.withTenant(tenantId, async (client) => {

      const result = await client.query<IntegrationRow>(

        `INSERT INTO integrations (

           tenant_id, platform, name, connection_mode, sync_mode,

           webhook_token, webhook_secret, api_credentials_blob

         ) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)

         RETURNING id::text, platform, name, connection_mode,
                   CASE WHEN connection_mode = 'api_sync' THEN sync_mode ELSE NULL END AS sync_mode,
                   CASE WHEN connection_mode = 'webhook' THEN webhook_token ELSE NULL END AS webhook_token,
                   is_active, last_sync_at::text, sync_status, last_sync_error,
                   created_at::text`,

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

    const res = await fetch(`${syncUrl}/test-auth`, {

      method: 'POST',

      headers: { 'Content-Type': 'application/json' },

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



  async triggerSync(tenantId: string, id: string): Promise<{ status: string }> {

    const row = await this.getRow(tenantId, id);

    if (row.connection_mode !== 'api_sync') {

      throw new BadRequestException('Sync is only available for API key integrations');

    }

    const syncUrl = process.env.PLATFORM_SYNC_URL ?? 'http://localhost:8088';

    const res = await fetch(`${syncUrl}/sync`, {

      method: 'POST',

      headers: { 'Content-Type': 'application/json' },

      body: JSON.stringify({ integration_id: id, tenant_id: tenantId }),

    });

    if (!res.ok) {

      throw new ServiceUnavailableException(

        `Platform sync service unavailable (${res.status})`,

      );

    }

    return { status: 'sync_triggered' };

  }



  async updateSyncMode(

    tenantId: string,

    id: string,

    syncMode: SyncMode,

  ): Promise<IntegrationRecord> {

    if (syncMode !== 'mock' && syncMode !== 'real') {

      throw new BadRequestException('sync_mode must be mock or real');

    }

    return this.postgres.withTenant(tenantId, async (client) => {

      const result = await client.query<IntegrationRow>(

        `UPDATE integrations

         SET sync_mode = $3, updated_at = NOW()

         WHERE id = $1::uuid AND tenant_id = $2::uuid

           AND connection_mode = 'api_sync' AND is_active = TRUE

         RETURNING id::text, platform, name, connection_mode,

                   sync_mode, NULL AS webhook_token,

                   is_active, last_sync_at::text, sync_status, last_sync_error,

                   created_at::text`,

        [id, tenantId, syncMode],

      );

      if (result.rowCount === 0) {

        throw new NotFoundException('API sync integration not found');

      }

      return this.toRecord(result.rows[0]);

    });

  }



  async softDelete(tenantId: string, id: string): Promise<void> {

    return this.postgres.withTenant(tenantId, async (client) => {

      const result = await client.query(

        `UPDATE integrations

         SET is_active = FALSE, updated_at = NOW()

         WHERE id = $1::uuid AND tenant_id = $2::uuid AND is_active = TRUE`,

        [id, tenantId],

      );

      if (result.rowCount === 0) {

        throw new NotFoundException('Integration not found');

      }

    });

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

    }

  }



  private async getRow(

    tenantId: string,

    id: string,

  ): Promise<{

    connection_mode: ConnectionMode;

    platform: string;

    sync_mode: SyncMode | null;

    api_credentials_blob: Buffer | null;

  }> {

    return this.postgres.withTenant(tenantId, async (client) => {

      const result = await client.query<{

        connection_mode: ConnectionMode;

        platform: string;

        sync_mode: SyncMode | null;

        api_credentials_blob: Buffer | null;

      }>(

        `SELECT connection_mode, platform, sync_mode, api_credentials_blob

         FROM integrations WHERE id = $1::uuid AND is_active = TRUE`,

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

    };

  }

}


