import { Body, Controller, Get, Headers, Post, Query, UnauthorizedException, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { randomUUID } from 'crypto';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { loadEnv } from '../config/env';
import { KafkaService } from '../kafka/kafka.service';
import { ScoreRecord, ScoringService, TranscriptRecord } from './scoring.service';

const callbackSchema = z.object({
  tenant_id: z.string().uuid(),
  call_id: z.string().min(1),
  platform: z.string().optional(),
  transcript_text: z.string().min(1),
  duration_seconds: z.number().optional(),
});

@Controller('scoring')
export class ScoringController {
  constructor(
    private readonly scoringService: ScoringService,
    private readonly kafka: KafkaService,
  ) {}

  @Get('scores')
  @UseGuards(AuthGuard('jwt'))
  async listScores(
    @CurrentTenant() tenantId: string,
    @Query('callId') callId?: string,
    @Query('limit') limit?: string,
    @Query('offset') offset?: string,
    @Query('show_demo') showDemoRaw?: string,
  ): Promise<{ data: ScoreRecord[] } | ScoreRecord | null> {
    if (callId) {
      const score = await this.scoringService.getScore(tenantId, callId);
      return score;
    }
    const showDemo = showDemoRaw === 'true' || showDemoRaw === '1';
    const data = await this.scoringService.listScores(
      tenantId,
      limit ? Number(limit) : 50,
      offset ? Number(offset) : 0,
      showDemo,
    );
    return { data };
  }

  @Get('transcript')
  @UseGuards(AuthGuard('jwt'))
  async getTranscript(
    @CurrentTenant() tenantId: string,
    @Query('callId') callId: string,
  ): Promise<TranscriptRecord | null> {
    return this.scoringService.getTranscript(tenantId, callId);
  }

  @Post('deepgram/callback')
  async deepgramCallback(
    @Headers('x-deepgram-callback-secret') secret: string | undefined,
    @Body() body: unknown,
  ): Promise<{ status: string }> {
    const env = loadEnv();
    if (!env.MOCK_MODE) {
      if (!secret || secret !== env.DEEPGRAM_CALLBACK_SECRET) {
        throw new UnauthorizedException('invalid callback secret');
      }
    }

    const parsed = callbackSchema.parse(body);
    await this.kafka.publish(env.KAFKA_TOPIC_CALL_TRANSCRIPT_READY, parsed.tenant_id, {
      schema_version: '1',
      event_id: randomUUID(),
      tenant_id: parsed.tenant_id,
      call_id: parsed.call_id,
      platform: parsed.platform ?? 'ringba',
      transcript_text: parsed.transcript_text,
      duration_seconds: parsed.duration_seconds ?? 0,
      received_at_unix_ms: Date.now(),
    });

    return { status: 'ok' };
  }
}
