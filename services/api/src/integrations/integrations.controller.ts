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

const convertWritebackSchema = z.object({
  inbound_call_id: z.string().min(8).max(128),
  amount: z.number({
    required_error: 'amount must be a positive number',
    invalid_type_error: 'amount must be a positive number',
  }).positive({ message: 'amount must be a positive number' }),
});

const voidWritebackSchema = z.object({
  inbound_call_id: z.string().min(8).max(128),
});

function writebackEnabled(): boolean {
  const v = process.env.WRITEBACK_ENABLED;
  return v === 'true' || v === '1';
}

function parseWriteback<S extends z.ZodTypeAny>(schema: S, body: unknown): z.output<S> {
  const parsed = schema.safeParse(body);
  if (!parsed.success) {
    const msg = parsed.error.issues[0]?.message ?? 'Validation failed';
    throw new BadRequestException(msg);
  }
  return parsed.data;
}

/** Registers write-back routes only when WRITEBACK_ENABLED is true/1. */
function ConditionalWritebackConvert(): MethodDecorator {
  if (writebackEnabled()) {
    return Post(':id/writeback/convert');
  }
  return () => undefined;
}

function ConditionalWritebackVoid(): MethodDecorator {
  if (writebackEnabled()) {
    return Post(':id/writeback/void');
  }
  return () => undefined;
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

  @ConditionalWritebackConvert()
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

  @ConditionalWritebackVoid()
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

  @Delete(':id')
  @HttpCode(204)
  async remove(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<void> {
    await this.integrationsService.softDelete(tenantId, id);
  }
}
