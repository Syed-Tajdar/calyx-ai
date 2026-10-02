import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { optionalIsoDate } from '../common/date-params';
import { AnalyticsService, MetricsResult, PnLResult, PlatformMixRow } from './analytics.service';

const showDemoField = z
  .union([z.literal('true'), z.literal('1'), z.literal('false'), z.literal('0')])
  .optional()
  .transform((v) => v === 'true' || v === '1');

const dateQuerySchema = z.object({
  from: optionalIsoDate,
  to: optionalIsoDate,
  show_demo: showDemoField,
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
    return this.analyticsService.getMetrics(
      tenantId,
      parsed.from,
      parsed.to,
      parsed.show_demo,
    );
  }

  @Get('pnl')
  async pnl(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<PnLResult> {
    const parsed = dateQuerySchema.parse(query);
    return this.analyticsService.getPnL(
      tenantId,
      parsed.from,
      parsed.to,
      parsed.show_demo,
    );
  }

  @Get('ping-series')
  async pingSeries(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<Array<{ date: string; count: number }>> {
    const parsed = dateQuerySchema.parse(query);
    return this.analyticsService.getPingSeries(
      tenantId,
      parsed.from,
      parsed.to,
      parsed.show_demo,
    );
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
      parsed.show_demo,
    );
  }
}
