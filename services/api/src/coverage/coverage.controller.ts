import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { CoverageService, HeatmapResult } from './coverage.service';

const heatmapQuerySchema = z.object({
  show_demo: z
    .union([z.literal('true'), z.literal('1'), z.literal('false'), z.literal('0')])
    .optional()
    .transform((v) => v === 'true' || v === '1'),
});

@Controller('coverage')
@UseGuards(AuthGuard('jwt'))
export class CoverageController {
  constructor(private readonly coverageService: CoverageService) {}

  @Get('heatmap')
  async heatmap(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<HeatmapResult> {
    const parsed = heatmapQuerySchema.parse(query);
    return this.coverageService.getHeatmap(tenantId, parsed.show_demo);
  }
}
