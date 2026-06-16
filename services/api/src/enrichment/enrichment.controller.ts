import { Controller, Get, Param, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import { EnrichmentApiService, EnrichmentAttribute } from './enrichment.service';

@Controller('enrichment')
@UseGuards(AuthGuard('jwt'))
export class EnrichmentController {
  constructor(private readonly enrichmentService: EnrichmentApiService) {}

  @Get(':aniHash')
  async getAttributes(
    @CurrentTenant() tenantId: string,
    @Param('aniHash') aniHash: string,
    @Query('limit') limit?: string,
  ): Promise<{ data: EnrichmentAttribute[]; total: number }> {
    const data = await this.enrichmentService.getByAniHash(
      tenantId,
      aniHash,
      limit ? Number(limit) : 100,
    );
    return { data, total: data.length };
  }
}
