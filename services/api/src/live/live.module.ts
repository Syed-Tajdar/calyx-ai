import { Module } from '@nestjs/common';
import { AuthModule } from '../auth/auth.module';
import { KafkaLiveConsumerService } from './kafka-live-consumer.service';
import { LiveGateway } from './live.gateway';
import { MockLiveEmitterService } from './mock-live-emitter.service';

@Module({
  imports: [AuthModule],
  providers: [LiveGateway, MockLiveEmitterService, KafkaLiveConsumerService],
  exports: [LiveGateway],
})
export class LiveModule {}
