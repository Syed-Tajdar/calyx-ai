import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant } from '../common/tenant.decorator';
import {
  AttributionEventRecord,
  AttributionService,
  PlatformStatusSummary,
} from './attribution.service';

@Controller('attribution')
@FeatureGate('attribution')
@UseGuards(AuthGuard('jwt'))
export class AttributionController {
  constructor(private readonly attributionService: AttributionService) {}

  @Get('events')
  async events(
    @CurrentTenant() tenantId: string,
    @Query('limit') limit?: string,
  ): Promise<{ data: AttributionEventRecord[] }> {
    return this.attributionService.listEvents(
      tenantId,
      limit ? Number(limit) : 50,
    );
  }

  @Get('status')
  async status(
    @CurrentTenant() tenantId: string,
  ): Promise<{ data: PlatformStatusSummary[] }> {
    return this.attributionService.getPlatformStatus(tenantId);
  }
}
