import { Module } from '@nestjs/common';
import { DeepQueryController } from './deep-query.controller';
import { DeepQueryService } from './deep-query.service';

@Module({
  controllers: [DeepQueryController],
  providers: [DeepQueryService],
})
export class NativeIntegrationModule {}
