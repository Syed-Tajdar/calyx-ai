import {
  Body,
  Controller,
  ForbiddenException,
  Get,
  Post,
  Query,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant, CurrentUser } from '../common/tenant.decorator';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { AskAiService, AskHistoryRecord, AskResponse } from './ask-ai.service';

const askBodySchema = z.object({ question: z.string().trim().min(3).max(1000) }).strict();

/**
 * Ask AI on numbers (v2 Phase 2A). Admin only. Behind FEATURE_ASK_AI (default off): the global
 * FeatureGateGuard (common/feature-gate.ts) answers 404 while the feature is closed, before auth.
 * Without OPENAI_API_KEY on the server the service answers 503 even when the feature is open.
 */
@Controller('ask')
@FeatureGate('ask_ai')
@UseGuards(AuthGuard('jwt'))
export class AskAiController {
  constructor(private readonly askAiService: AskAiService) {}

  private gate(user: JwtPayload): void {
    if (user.role !== 'admin') throw new ForbiddenException('Ask AI is for admins');
  }

  @Post()
  async ask(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Body() body: unknown,
  ): Promise<AskResponse> {
    this.gate(user);
    const parsed = askBodySchema.parse(body);
    return this.askAiService.ask(tenantId, user.sub ?? null, parsed.question);
  }

  @Get('history')
  async history(
    @CurrentTenant() tenantId: string,
    @CurrentUser() user: JwtPayload,
    @Query('limit') limit?: string,
  ): Promise<{ data: AskHistoryRecord[] }> {
    this.gate(user);
    return this.askAiService.listHistory(tenantId, limit ? Number(limit) : 20);
  }
}
