import { Controller, Get, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import { AnalyticsService, MetricsResult, PnLResult } from './analytics.service';

@Controller('analytics')
@UseGuards(AuthGuard('jwt'))
export class AnalyticsController {
  constructor(private readonly analyticsService: AnalyticsService) {}

  @Get('metrics')
  async metrics(@CurrentTenant() tenantId: string): Promise<MetricsResult> {
    return this.analyticsService.getMetrics(tenantId);
  }

  @Get('pnl')
  async pnl(@CurrentTenant() tenantId: string): Promise<PnLResult> {
    return this.analyticsService.getPnL(tenantId);
  }
}
