import { Body, Controller, Get, Post, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import {
  AskAiService,
  AskHistoryRecord,
  AskResponse,
} from './ask-ai.service';

const askBodySchema = z.object({
  question: z.string().min(1).max(2000),
});

@Controller('ask')
@UseGuards(AuthGuard('jwt'))
export class AskAiController {
  constructor(private readonly askAiService: AskAiService) {}

  @Post()
  async ask(
    @CurrentTenant() tenantId: string,
    @Body() body: unknown,
  ): Promise<AskResponse> {
    const parsed = askBodySchema.parse(body);
    return this.askAiService.ask(tenantId, parsed.question);
  }

  @Get('history')
  async history(
    @CurrentTenant() tenantId: string,
    @Query('limit') limit?: string,
  ): Promise<{ data: AskHistoryRecord[] }> {
    return this.askAiService.listHistory(
      tenantId,
      limit ? Number(limit) : 50,
    );
  }
}
