import { Module } from '@nestjs/common';
import { EventLoopLagService } from './event-loop-lag.service';
import { HealthController } from './health.controller';

@Module({
  controllers: [HealthController],
  providers: [EventLoopLagService],
  exports: [EventLoopLagService],
})
export class HealthModule {}
