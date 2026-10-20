import { Body, Controller, Delete, Get, HttpCode, Param, Post, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { FeatureGate } from '../common/feature-gate';
import { AlertRule, AlertsService } from './alerts.service';

const createRuleSchema = z.object({
  name: z.string().min(1),
  condition: z.record(z.unknown()),
  threshold: z.number(),
});

/** Mock alert rules (the engine only logs to stdout). Closed in v1; v2 Phase 2D re-opens it. */
@Controller('alerts')
@FeatureGate('workflows')
@UseGuards(AuthGuard('jwt'))
export class AlertsController {
  constructor(private readonly alertsService: AlertsService) {}

  @Get('rules')
  async list(@CurrentTenant() tenantId: string): Promise<{ data: AlertRule[] }> {
    const data = await this.alertsService.list(tenantId);
    return { data };
  }

  @Post('rules')
  async create(
    @CurrentTenant() tenantId: string,
    @Body() body: unknown,
  ): Promise<AlertRule> {
    const parsed = createRuleSchema.parse(body);
    return this.alertsService.create(tenantId, parsed);
  }

  @Delete('rules/:id')
  @HttpCode(204)
  async remove(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<void> {
    await this.alertsService.softDelete(tenantId, id);
  }
}
