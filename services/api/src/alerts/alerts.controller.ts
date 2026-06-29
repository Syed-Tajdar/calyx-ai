import { Body, Controller, Delete, Get, HttpCode, Param, Post, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { AlertRule, AlertsService } from './alerts.service';

const createRuleSchema = z.object({
  name: z.string().min(1),
  condition: z.record(z.unknown()),
  threshold: z.number(),
});

@Controller('alerts')
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
