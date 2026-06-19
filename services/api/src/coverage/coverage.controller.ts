import { Controller, Get, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import { CoverageService, HeatmapResult } from './coverage.service';

@Controller('coverage')
@UseGuards(AuthGuard('jwt'))
export class CoverageController {
  constructor(private readonly coverageService: CoverageService) {}

  @Get('heatmap')
  async heatmap(@CurrentTenant() tenantId: string): Promise<HeatmapResult> {
    return this.coverageService.getHeatmap(tenantId);
  }
}
