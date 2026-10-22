import {
  Body,
  Controller,
  Delete,
  ForbiddenException,
  Get,
  HttpCode,
  HttpException,
  Param,
  ParseUUIDPipe,
  Post,
  Put,
  Query,
  Res,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import type { Response } from 'express';
import { z } from 'zod';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant, CurrentUser } from '../common/tenant.decorator';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { assertPlatform, TranscriptionService } from './transcription.service';

const callIdSchema = z.string().trim().min(1).max(200);
const backfillSchema = z.object({ days: z.number().int().min(1).max(30) }).strict();
const settingsSchema = z.object({ archive_all: z.boolean() }).strict();
const recordingAccessSchema = z
  .object({ username: z.string().trim().min(1).max(200), password: z.string().min(1).max(500) })
  .strict();
const uuid = new ParseUUIDPipe();

function parse<T>(schema: z.ZodType<T>, v: unknown): T {
  const r = schema.safeParse(v);
  if (!r.success) throw new HttpException(r.error.issues[0]?.message ?? 'invalid request', 400);
  return r.data;
}

/**
 * v2 Phase 2B — Transcription and the recording archive. Behind FEATURE_TRANSCRIPTION (404 while it is
 * off, before auth). Reading a transcript is for any signed-in user of the tenant; everything else is
 * admin only. Audio is proxied (verified against its SHA-256 by platform-sync) and every play,
 * download and evidence pack is written to recording_access_audit — refused attempts too.
 */
@Controller('transcription')
@FeatureGate('transcription')
@UseGuards(AuthGuard('jwt'))
export class TranscriptionController {
  constructor(private readonly transcription: TranscriptionService) {}

  private admin(user: JwtPayload): void {
    if (user?.role !== 'admin') throw new ForbiddenException('Admins only');
  }

  @Get('calls/:platform/:callId')
  view(@CurrentTenant() t: string, @Param('platform') platform: string, @Param('callId') callId: string) {
    return this.transcription.view(t, assertPlatform(platform), parse(callIdSchema, callId));
  }

  @Post('calls/:platform/:callId/transcribe')
  @HttpCode(202)
  transcribe(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('platform') platform: string, @Param('callId') callId: string) {
    this.admin(u);
    return this.transcription.transcribeNow(t, u.sub, assertPlatform(platform), parse(callIdSchema, callId));
  }

  @Get('calls/:platform/:callId/audio')
  async audio(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Param('platform') platform: string,
    @Param('callId') callId: string,
    @Query('download') download: string | undefined,
    @Res() res: Response,
  ): Promise<void> {
    const p = assertPlatform(platform);
    const id = parse(callIdSchema, callId);
    const kind = download === '1' ? 'download' : 'play';
    if (u?.role !== 'admin') {
      await this.transcription.auditRefused(t, u.sub, kind, p, id);
      throw new ForbiddenException('Recordings are for admins');
    }
    const r = await this.transcription.recording(t, u.sub, kind, p, id);
    if (!r.ok) throw new HttpException(r.error ?? 'Recording not available', r.status === 404 ? 404 : 502);
    const body = r.body ?? Buffer.alloc(0);
    const contentType = r.contentType ?? 'application/octet-stream';
    res.setHeader('Content-Type', contentType);
    res.setHeader('Content-Length', String(body.length));
    res.setHeader('Cache-Control', 'no-store');
    if (r.sha256) res.setHeader('X-Recording-SHA256', r.sha256);
    if (kind === 'download') {
      const ext = contentType.includes('wav') ? 'wav' : 'mp3';
      res.setHeader('Content-Disposition', `attachment; filename="calyx-recording-${p}-${id.replace(/[^A-Za-z0-9_-]/g, '')}.${ext}"`);
    }
    res.end(body);
  }

  @Get('calls/:platform/:callId/evidence')
  async evidence(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Param('platform') platform: string,
    @Param('callId') callId: string,
    @Res() res: Response,
  ): Promise<void> {
    const p = assertPlatform(platform);
    const id = parse(callIdSchema, callId);
    if (u?.role !== 'admin') {
      await this.transcription.auditRefused(t, u.sub, 'evidence_pack', p, id);
      throw new ForbiddenException('Evidence packs are for admins');
    }
    const r = await this.transcription.recording(t, u.sub, 'evidence_pack', p, id);
    if (!r.ok) throw new HttpException(r.error ?? 'Evidence pack not available', r.status === 404 ? 404 : 502);
    const body = r.body ?? Buffer.alloc(0);
    res.setHeader('Content-Type', 'application/zip');
    res.setHeader('Content-Length', String(body.length));
    res.setHeader('Cache-Control', 'no-store');
    res.setHeader('Content-Disposition', `attachment; filename="${r.filename ?? 'calyx-evidence.zip'}"`);
    res.end(body);
  }

  @Get('backfill/estimate')
  backfillEstimate(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Query('days') days: string | undefined) {
    this.admin(u);
    return this.transcription.backfillEstimate(t, days ? Number(days) : 7);
  }

  @Post('backfill')
  @HttpCode(202)
  backfill(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Body() body: unknown) {
    this.admin(u);
    return this.transcription.backfillStart(t, u.sub, parse(backfillSchema, body).days);
  }

  @Get('usage')
  usage(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return this.transcription.usage(t);
  }

  @Put('settings')
  settings(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Body() body: unknown) {
    this.admin(u);
    return this.transcription.setArchiveAll(t, u.sub, parse(settingsSchema, body).archive_all);
  }

  /** Ringba Recording Access credentials: write-only, stored encrypted, never returned. */
  @Put('integrations/:id/recording-access')
  setRecordingAccess(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string, @Body() body: unknown) {
    this.admin(u);
    const b = parse(recordingAccessSchema, body);
    return this.transcription.setRecordingAccess(t, id, b.username, b.password);
  }

  @Delete('integrations/:id/recording-access')
  clearRecordingAccess(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string) {
    this.admin(u);
    return this.transcription.setRecordingAccess(t, id, null, null);
  }
}
