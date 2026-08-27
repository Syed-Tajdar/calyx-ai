import { Controller, Get, Param, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import { EnrichmentApiService, EnrichmentListResult } from './enrichment.service';

@Controller('enrichment')
@UseGuards(AuthGuard('jwt'))
export class EnrichmentController {
  constructor(private readonly enrichmentService: EnrichmentApiService) {}

  @Get(':aniHash')
  async getAttributes(
    @CurrentTenant() tenantId: string,
    @Param('aniHash') aniHash: string,
    @Query('limit') limit?: string,
  ): Promise<EnrichmentListResult> {
    return this.enrichmentService.getByAniHash(
      tenantId,
      aniHash,
      limit ? Number(limit) : 100,
    );
  }
}
