import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant } from '../common/tenant.decorator';
import { ScoreRecord, ScoringService, TranscriptRecord } from './scoring.service';

/**
 * Mock scoring reads (behind the qa / transcription flags). The old POST /scoring/deepgram/callback
 * (plain-string secret, skipped in mock mode, a made-up payload) was removed in v2 Phase 2B: Deepgram
 * is called synchronously by the platform-sync worker, so no public callback exists.
 */
@Controller('scoring')
export class ScoringController {
  constructor(private readonly scoringService: ScoringService) {}

  @Get('scores')
  @FeatureGate('qa')
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
  @FeatureGate('transcription')
  @UseGuards(AuthGuard('jwt'))
  async getTranscript(
    @CurrentTenant() tenantId: string,
    @Query('callId') callId: string,
  ): Promise<TranscriptRecord | null> {
    return this.scoringService.getTranscript(tenantId, callId);
  }
}
