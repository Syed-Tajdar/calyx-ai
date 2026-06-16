import { Module } from '@nestjs/common';
import { EnrichmentController } from './enrichment.controller';
import { EnrichmentApiService } from './enrichment.service';

@Module({
  controllers: [EnrichmentController],
  providers: [EnrichmentApiService],
})
export class EnrichmentModule {}
