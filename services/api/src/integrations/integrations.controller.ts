import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  Patch,
  Post,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import {
  CreateIntegrationInput,
  IntegrationRecord,
  IntegrationsService,
  TestConnectionResult,
} from './integrations.service';

const createSchema = z.object({
  platform: z.enum(['ringba', 'retreaver', 'trackdrive', 'callgrid']),
  name: z.string().min(1).max(128),
  connection_mode: z.enum(['webhook', 'api_sync']),
  sync_mode: z.enum(['mock', 'real']).optional(),
  webhook_secret: z.string().min(1).max(512).optional(),
  api_key: z.string().min(1).max(512).optional(),
  api_token: z.string().min(1).max(512).optional(),
  account_id: z.string().min(1).max(128).optional(),
  company_id: z.string().min(1).max(128).optional(),
  subdomain: z.string().min(1).max(128).optional(),
  public_key: z.string().min(1).max(512).optional(),
  private_key: z.string().min(1).max(512).optional(),
  auth_token: z.string().min(1).max(512).optional(),
});

const syncModeSchema = z.object({
  sync_mode: z.enum(['mock', 'real']),
});

@Controller('integrations')
@UseGuards(AuthGuard('jwt'))
export class IntegrationsController {
  constructor(private readonly integrationsService: IntegrationsService) {}

  @Get()
  async list(@CurrentTenant() tenantId: string): Promise<IntegrationRecord[]> {
    return this.integrationsService.list(tenantId);
  }

  @Get('platform-options')
  listPlatformOptions(): ReturnType<IntegrationsService['listPlatformOptions']> {
    return this.integrationsService.listPlatformOptions();
  }

  @Post()
  async create(
    @CurrentTenant() tenantId: string,
    @Body() body: unknown,
  ): Promise<IntegrationRecord> {
    const parsed = createSchema.parse(body) as CreateIntegrationInput;
    return this.integrationsService.create(tenantId, parsed);
  }

  @Post(':id/test')
  async test(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<TestConnectionResult> {
    return this.integrationsService.testConnection(tenantId, id);
  }

  @Post(':id/sync')
  async sync(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<{ status: string }> {
    return this.integrationsService.triggerSync(tenantId, id);
  }

  @Patch(':id/sync-mode')
  async updateSyncMode(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
    @Body() body: unknown,
  ): Promise<IntegrationRecord> {
    const parsed = syncModeSchema.parse(body);
    return this.integrationsService.updateSyncMode(
      tenantId,
      id,
      parsed.sync_mode,
    );
  }

  @Delete(':id')
  @HttpCode(204)
  async remove(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<void> {
    await this.integrationsService.softDelete(tenantId, id);
  }
}
