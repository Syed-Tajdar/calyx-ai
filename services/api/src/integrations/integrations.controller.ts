import {
  Body,
  Controller,
  Delete,
  BadRequestException,
  ForbiddenException,
  Get,
  HttpCode,
  Param,
  Patch,
  Post,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant, CurrentUser } from '../common/tenant.decorator';
import { JwtPayload } from '../auth/jwt-payload.interface';
import {
  CreateIntegrationInput,
  IntegrationRecord,
  IntegrationsService,
  TestConnectionResult,
} from './integrations.service';
import { convertWritebackSchema, voidWritebackSchema } from './writeback.validation';

const createSchema = z.object({
  platform: z.enum(['ringba', 'retreaver', 'trackdrive', 'callgrid']),
  name: z.string().min(1).max(128),
  connection_mode: z.enum(['webhook', 'api_sync']),
  sync_mode: z.enum(['mock', 'real']).optional(),
  initial_lookback_days: z.number().int().min(1).max(365).optional(),
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
  initial_lookback_days: z.number().int().min(1).max(365).optional(),
});

function parseWriteback<S extends z.ZodTypeAny>(schema: S, body: unknown): z.output<S> {
  const parsed = schema.safeParse(body);
  if (!parsed.success) {
    const msg = parsed.error.issues[0]?.message ?? 'Validation failed';
    throw new BadRequestException(msg);
  }
  return parsed.data;
}

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

  @Post(':id/backfill')
  async backfill(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<{ status: string }> {
    return this.integrationsService.triggerBackfill(tenantId, id);
  }

  @Post(':id/writeback/convert')
  @HttpCode(200)
  async convertWriteback(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Param('id') id: string,
    @Body() body: unknown,
  ): Promise<unknown> {
    if (user.role !== 'admin') {
      throw new ForbiddenException('Write-back is admin-only');
    }
    const parsed = parseWriteback(convertWritebackSchema, body);
    return this.integrationsService.convertWriteback(
      tenantId,
      user.sub,
      id,
      parsed.inbound_call_id,
      parsed.amount,
    );
  }

  @Post(':id/writeback/void')
  @HttpCode(200)
  async voidWriteback(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Param('id') id: string,
    @Body() body: unknown,
  ): Promise<unknown> {
    if (user.role !== 'admin') {
      throw new ForbiddenException('Write-back is admin-only');
    }
    const parsed = parseWriteback(voidWritebackSchema, body);
    return this.integrationsService.voidWriteback(
      tenantId,
      user.sub,
      id,
      parsed.inbound_call_id,
    );
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
      parsed.initial_lookback_days,
    );
  }

  @Post(':id/pause')
  @HttpCode(200)
  async pause(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Param('id') id: string,
  ): Promise<IntegrationRecord> {
    if (user.role !== 'admin') {
      throw new ForbiddenException('Pause is admin-only');
    }
    return this.integrationsService.setActive(tenantId, user.sub, id, false);
  }

  @Post(':id/resume')
  @HttpCode(200)
  async resume(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Param('id') id: string,
  ): Promise<IntegrationRecord> {
    if (user.role !== 'admin') {
      throw new ForbiddenException('Resume is admin-only');
    }
    return this.integrationsService.setActive(tenantId, user.sub, id, true);
  }

  @Delete(':id')
  @HttpCode(204)
  async remove(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Param('id') id: string,
  ): Promise<void> {
    await this.integrationsService.softDelete(tenantId, id, user.sub);
  }
}
