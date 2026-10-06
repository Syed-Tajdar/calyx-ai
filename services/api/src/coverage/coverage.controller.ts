import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { optionalIsoDate } from '../common/date-params';
import { CurrentTenant } from '../common/tenant.decorator';
import { parseZod } from '../common/zod-parse';
import { CoverageService, HeatmapResult } from './coverage.service';

const heatmapQuerySchema = z.object({
  show_demo: z
    .union([z.literal('true'), z.literal('1'), z.literal('false'), z.literal('0')])
    .optional()
    .transform((v) => v === 'true' || v === '1'),
  // Same period semantics as /analytics/metrics so the map and the dashboard agree.
  from: optionalIsoDate,
  to: optionalIsoDate,
  platform: z.string().optional(),
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
    const parsed = parseZod(heatmapQuerySchema, query);
    return this.coverageService.getHeatmap(
      tenantId,
      parsed.show_demo,
      parsed.from,
      parsed.to,
      parsed.platform,
    );
  }
}
