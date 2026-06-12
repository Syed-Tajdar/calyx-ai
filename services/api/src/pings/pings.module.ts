import { Module } from '@nestjs/common';
import { PingsController } from './pings.controller';
import { PingsService } from './pings.service';

@Module({
  controllers: [PingsController],
  providers: [PingsService],
})
export class PingsModule {}
