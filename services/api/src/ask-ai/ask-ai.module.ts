import { Module } from '@nestjs/common';
import { AskAiController } from './ask-ai.controller';
import { AskAiService } from './ask-ai.service';

@Module({
  controllers: [AskAiController],
  providers: [AskAiService],
})
export class AskAiModule {}
