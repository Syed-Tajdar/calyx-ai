import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { AnalyticsService, MetricsResult, PnLResult, PlatformMixRow } from './analytics.service';

const dateQuerySchema = z.object({
  from: z.string().optional(),
  to: z.string().optional(),
});

const platformMixQuerySchema = dateQuerySchema.extend({
  platform: z.string().optional(),
});

@Controller('analytics')
@UseGuards(AuthGuard('jwt'))
export class AnalyticsController {
  constructor(private readonly analyticsService: AnalyticsService) {}

  @Get('metrics')
  async metrics(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<MetricsResult> {
    const parsed = dateQuerySchema.parse(query);
    return this.analyticsService.getMetrics(tenantId, parsed.from, parsed.to);
  }

  @Get('pnl')
  async pnl(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<PnLResult> {
    const parsed = dateQuerySchema.parse(query);
    return this.analyticsService.getPnL(tenantId, parsed.from, parsed.to);
  }

  @Get('platform-mix')
  async platformMix(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<PlatformMixRow[]> {
    const parsed = platformMixQuerySchema.parse(query);
    return this.analyticsService.getPlatformMix(
      tenantId,
      parsed.from,
      parsed.to,
      parsed.platform,
    );
  }
}
