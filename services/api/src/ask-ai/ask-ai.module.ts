import { Module } from '@nestjs/common';
import { AskAiController } from './ask-ai.controller';
import { AskAiService } from './ask-ai.service';
import { AskQueriesService } from './ask-queries.service';

@Module({
  controllers: [AskAiController],
  providers: [AskAiService, AskQueriesService],
})
export class AskAiModule {}
