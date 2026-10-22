import { BadRequestException, ConflictException, Inject, Injectable, NotFoundException } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';
import { encryptCredentialJson } from '../integrations/credential-crypto';
import type { TenantRunner } from '../workflows/workflow-db';
import { Candidate, CandidateReason, TranscriptionCandidatesService } from './transcription-candidates.service';
import { DEFAULT_CALL_MINUTES, transcriptionSettings } from './transcription-config';
import { RECORDING_CLIENT, RecordingClient, RecordingFetch } from './recording.client';

export type Platform = 'ringba' | 'retreaver';
type JobReason = CandidateReason | 'on_demand' | 'backfill';

export interface TranscriptView {
  platform: string;
  external_call_id: string;
  /** none | queued | running | done | no_recording | failed | capped */
  state: string;
  no_recording_reason: string | null;
  error: string | null;
  reason: string | null;
  recording: null | {
    archive_state: string;
    sha256: string;
    bytes: number;
    duration_seconds: number | null;
    downloaded_at: string;
  };
  transcript: null | {
    version: number;
    language: string | null;
    model: string;
    deepgram_request_id: string | null;
    redaction: string;
    created_at: string;
    utterances: Array<{ idx: number; speaker: number | null; role: string; start_ms: number; end_ms: number; text: string }>;
  };
}

export interface UsageView {
  month: string; // YYYY-MM, Eastern
  minutes_month: number;
  calls_month: number;
  estimated_usd_month: number;
  minutes_today: number;
  daily_cap_minutes: number;
  archive: { recordings: number; bytes: number; estimated_usd_month: number; not_archived: number; expired: number };
  queue: Record<string, number>;
  worker: {
    reachable: boolean;
    enabled: boolean;
    deepgram_key_set: boolean;
    archive_configured: boolean;
    retention_months: number | null;
  };
  settings: { should_convert: boolean; min_seconds: number; archive_all: boolean };
}

const PLATFORMS: readonly Platform[] = ['ringba', 'retreaver'];

export function assertPlatform(p: string): Platform {
  if (!(PLATFORMS as readonly string[]).includes(p)) throw new BadRequestException('platform must be ringba or retreaver');
  return p as Platform;
}

/**
 * v2 Phase 2B: the API side of transcription. It decides WHICH calls are queued (the worker in
 * platform-sync does the fetching, transcribing and archiving), shows transcripts, and proxies the
 * archived audio to admins with an audit row per play or download. Every query runs under RLS.
 */
@Injectable()
export class TranscriptionService {
  constructor(
    @Inject(PostgresService) private readonly pg: TenantRunner,
    private readonly candidates: TranscriptionCandidatesService,
    @Inject(RECORDING_CLIENT) private readonly recordings: RecordingClient,
  ) {}

  // ---- queue ----

  /** Insert jobs, never twice per call (UNIQUE tenant+platform+call). Returns how many were new. */
  async enqueue(tenantId: string, calls: Candidate[], reasonOverride?: JobReason, userId?: string): Promise<number> {
    if (calls.length === 0) return 0;
    return this.pg.withTenant(tenantId, async (c) => {
      const ints = await c.query(
        `SELECT DISTINCT ON (platform) id::text, platform FROM integrations
          WHERE deleted_at IS NULL AND is_active AND connection_mode = 'api_sync' AND platform IN ('ringba', 'retreaver')
          ORDER BY platform, (sync_mode = 'real') DESC, created_at DESC`,
      );
      const byPlatform = new Map<string, string>((ints.rows as Array<{ id: string; platform: string }>).map((r) => [r.platform, r.id]));
      let added = 0;
      for (const call of calls) {
        const integrationId = byPlatform.get(call.platform);
        if (!integrationId) continue;
        const r = await c.query(
          `INSERT INTO transcription_jobs (tenant_id, integration_id, platform, external_call_id, reason,
                                           occurred_at, duration_seconds, campaign_name, requested_by)
           VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::timestamptz, $7, $8, $9::uuid)
           ON CONFLICT (tenant_id, platform, external_call_id) DO NOTHING`,
          [tenantId, integrationId, call.platform, call.external_call_id, reasonOverride ?? call.reason,
           call.occurred_at, call.duration_seconds, call.campaign_name, userId ?? null],
        );
        added += r.rowCount ?? 0;
      }
      return added;
    });
  }

  /** The admin "Transcribe" button. A call already transcribed is refused; a failed one is retried. */
  async transcribeNow(tenantId: string, userId: string, platform: Platform, externalCallId: string) {
    const id = externalCallId.trim();
    if (!id || id.length > 200) throw new BadRequestException('call id required');
    return this.pg.withTenant(tenantId, async (c) => {
      const existing = await c.query(
        `SELECT id::text, state FROM transcription_jobs WHERE platform = $1 AND external_call_id = $2`,
        [platform, id],
      );
      const row = existing.rows[0] as { id: string; state: string } | undefined;
      if (row && ['done', 'queued', 'running'].includes(row.state)) {
        throw new ConflictException(row.state === 'done' ? 'This call is already transcribed' : 'This call is already queued');
      }
      if (row) {
        await c.query(
          `UPDATE transcription_jobs SET state = 'queued', reason = 'on_demand', attempts = 0, next_try_at = NOW(),
                  last_error = NULL, no_recording_reason = NULL, requested_by = $2::uuid, updated_at = NOW()
            WHERE id = $1::uuid`,
          [row.id, userId],
        );
        return { state: 'queued', retried: true };
      }
      const integ = await c.query(
        `SELECT id::text FROM integrations WHERE deleted_at IS NULL AND is_active AND connection_mode = 'api_sync'
            AND platform = $1 ORDER BY (sync_mode = 'real') DESC, created_at DESC LIMIT 1`,
        [platform],
      );
      if (integ.rowCount === 0) throw new NotFoundException(`No active ${platform} API integration`);
      // Same facts as an automatic job: the Ringba lookup window, the cap estimate, the Spanish rule.
      const call = await this.candidates.one(tenantId, platform, id);
      await c.query(
        `INSERT INTO transcription_jobs (tenant_id, integration_id, platform, external_call_id, reason, requested_by,
                                         occurred_at, duration_seconds, campaign_name)
         VALUES ($1::uuid, $2::uuid, $3, $4, 'on_demand', $5::uuid, $6::timestamptz, $7, $8)`,
        [tenantId, (integ.rows[0] as { id: string }).id, platform, id, userId,
         call?.occurred_at ?? null, call?.duration_seconds ?? null, call?.campaign_name ?? null],
      );
      return { state: 'queued', retried: false };
    });
  }

  /** Backfill preview: how many calls, minutes and $ — shown before anything is queued. */
  async backfillEstimate(tenantId: string, days: number, now = new Date()) {
    const s = transcriptionSettings();
    const d = this.backfillDays(days);
    const calls = await this.backfillCandidates(tenantId, d, now);
    const already = await this.alreadyQueued(tenantId, calls);
    const fresh = calls.filter((c) => !already.has(`${c.platform}|${c.external_call_id}`));
    const minutes = fresh.reduce((m, c) => m + (c.duration_seconds > 0 ? c.duration_seconds / 60 : DEFAULT_CALL_MINUTES), 0);
    const rounded = Math.round(minutes * 10) / 10;
    return {
      days: d,
      calls: fresh.length,
      already_queued_or_done: calls.length - fresh.length,
      minutes: rounded,
      estimated_usd: Math.round(rounded * s.pricePerMinute * 100) / 100,
      price_per_minute: s.pricePerMinute,
      daily_cap_minutes: s.dailyCapMinutes,
      days_at_cap: s.dailyCapMinutes > 0 ? Math.ceil(rounded / s.dailyCapMinutes) : null,
      includes_should_convert: s.shouldConvert,
    };
  }

  async backfillStart(tenantId: string, userId: string, days: number, now = new Date()) {
    const d = this.backfillDays(days);
    const calls = await this.backfillCandidates(tenantId, d, now);
    const queued = await this.enqueue(tenantId, calls.filter((c) => c.reason !== 'archive_only'), 'backfill', userId);
    return { days: d, queued };
  }

  private backfillDays(days: number): number {
    const s = transcriptionSettings();
    const d = Number.isFinite(days) && days > 0 ? Math.floor(days) : s.backfillDefaultDays;
    if (d > s.backfillMaxDays) throw new BadRequestException(`Backfill is limited to ${s.backfillMaxDays} days`);
    return d;
  }

  private async backfillCandidates(tenantId: string, days: number, now: Date): Promise<Candidate[]> {
    const s = transcriptionSettings();
    return this.candidates.find({
      tenantId,
      eventFrom: new Date(now.getTime() - days * 86_400_000),
      eventTo: now,
      minSeconds: s.minSeconds,
      shouldConvert: s.shouldConvert,
      archiveAll: false,
      limit: 20_000,
    });
  }

  private async alreadyQueued(tenantId: string, calls: Candidate[]): Promise<Set<string>> {
    if (calls.length === 0) return new Set();
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT platform || '|' || external_call_id AS k FROM transcription_jobs
          WHERE external_call_id = ANY($1::text[])`,
        [calls.map((x) => x.external_call_id)],
      );
      return new Set((r.rows as Array<{ k: string }>).map((x) => x.k));
    });
  }

  // ---- read ----

  async view(tenantId: string, platform: Platform, externalCallId: string): Promise<TranscriptView> {
    return this.pg.withTenant(tenantId, async (c) => {
      const job = (await c.query(
        `SELECT state, reason, no_recording_reason, last_error FROM transcription_jobs WHERE platform = $1 AND external_call_id = $2`,
        [platform, externalCallId],
      )).rows[0] as { state: string; reason: string; no_recording_reason: string | null; last_error: string | null } | undefined;
      const rec = (await c.query(
        `SELECT archive_state, sha256, bytes::float8 AS bytes, duration_seconds::float8 AS duration_seconds,
                to_json(downloaded_at) #>> '{}' AS downloaded_at
           FROM call_recordings WHERE platform = $1 AND external_call_id = $2`,
        [platform, externalCallId],
      )).rows[0] as TranscriptView['recording'] | undefined;
      const tr = (await c.query(
        `SELECT id::text, transcript_version AS version, language, model, deepgram_request_id, redaction,
                to_json(created_at) #>> '{}' AS created_at
           FROM call_transcripts WHERE platform = $1 AND external_call_id = $2
          ORDER BY transcript_version DESC LIMIT 1`,
        [platform, externalCallId],
      )).rows[0] as (NonNullable<TranscriptView['transcript']> & { id: string }) | undefined;
      let utterances: NonNullable<TranscriptView['transcript']>['utterances'] = [];
      if (tr) {
        utterances = (await c.query(
          `SELECT idx, speaker, role, start_ms, end_ms, text FROM call_transcript_utterances
            WHERE transcript_id = $1::uuid ORDER BY idx`,
          [tr.id],
        )).rows as typeof utterances;
      }
      return {
        platform,
        external_call_id: externalCallId,
        state: tr ? 'done' : (job?.state ?? 'none'),
        no_recording_reason: job?.no_recording_reason ?? null,
        error: job?.state === 'failed' ? (job.last_error ?? null) : null,
        reason: job?.reason ?? null,
        recording: rec ? { ...rec, bytes: Number(rec.bytes), duration_seconds: rec.duration_seconds === null ? null : Number(rec.duration_seconds) } : null,
        transcript: tr
          ? { version: tr.version, language: tr.language, model: tr.model, deepgram_request_id: tr.deepgram_request_id,
              redaction: tr.redaction, created_at: tr.created_at, utterances }
          : null,
      };
    });
  }

  async usage(tenantId: string): Promise<UsageView> {
    const s = transcriptionSettings();
    const worker = await this.recordings.status();
    return this.pg.withTenant(tenantId, async (c) => {
      const m = (await c.query(
        `SELECT to_char((NOW() AT TIME ZONE 'America/New_York')::date, 'YYYY-MM') AS month,
                COALESCE(SUM(minutes), 0)::float8 AS minutes, COALESCE(SUM(calls), 0)::int AS calls,
                COALESCE(SUM(minutes) FILTER (WHERE usage_date = (NOW() AT TIME ZONE 'America/New_York')::date), 0)::float8 AS today
           FROM transcription_usage_daily
          WHERE usage_date >= date_trunc('month', (NOW() AT TIME ZONE 'America/New_York'))::date`,
      )).rows[0] as { month: string; minutes: number; calls: number; today: number };
      const a = (await c.query(
        `SELECT COUNT(*) FILTER (WHERE archive_state = 'archived')::int AS recordings,
                COALESCE(SUM(bytes) FILTER (WHERE archive_state = 'archived'), 0)::float8 AS bytes,
                COUNT(*) FILTER (WHERE archive_state = 'not_archived')::int AS not_archived,
                COUNT(*) FILTER (WHERE archive_state = 'expired')::int AS expired
           FROM call_recordings`,
      )).rows[0] as { recordings: number; bytes: number; not_archived: number; expired: number };
      const q = (await c.query(`SELECT state, COUNT(*)::int AS n FROM transcription_jobs GROUP BY state`)).rows as Array<{ state: string; n: number }>;
      const st = (await c.query(`SELECT archive_all FROM transcription_settings`)).rows[0] as { archive_all: boolean } | undefined;
      const minutes = Math.round(Number(m.minutes) * 10) / 10;
      const gb = Number(a.bytes) / 1e9;
      return {
        month: m.month,
        minutes_month: minutes,
        calls_month: Number(m.calls),
        estimated_usd_month: Math.round(minutes * s.pricePerMinute * 100) / 100,
        minutes_today: Math.round(Number(m.today) * 10) / 10,
        daily_cap_minutes: worker.daily_cap_minutes ?? s.dailyCapMinutes,
        archive: {
          recordings: a.recordings,
          bytes: Number(a.bytes),
          // B2: first 10 GB free, then $/GB-month.
          estimated_usd_month: Math.round(Math.max(0, gb - 10) * s.archivePricePerGbMonth * 100) / 100,
          not_archived: a.not_archived,
          expired: a.expired,
        },
        queue: Object.fromEntries(q.map((r) => [r.state, r.n])),
        worker: {
          reachable: worker.reachable,
          enabled: worker.enabled,
          deepgram_key_set: worker.deepgram_key_set,
          archive_configured: worker.archive_configured,
          retention_months: worker.retention_months,
        },
        settings: { should_convert: s.shouldConvert, min_seconds: s.minSeconds, archive_all: st?.archive_all ?? false },
      };
    });
  }

  async setArchiveAll(tenantId: string, userId: string, archiveAll: boolean) {
    return this.pg.withTenant(tenantId, async (c) => {
      await c.query(
        `INSERT INTO transcription_settings (tenant_id, archive_all, updated_by, updated_at)
         VALUES ($1::uuid, $2, $3::uuid, NOW())
         ON CONFLICT (tenant_id) DO UPDATE SET archive_all = EXCLUDED.archive_all, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
        [tenantId, archiveAll, userId],
      );
      return { archive_all: archiveAll };
    });
  }

  async archiveAll(tenantId: string): Promise<boolean> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = (await c.query(`SELECT archive_all FROM transcription_settings`)).rows[0] as { archive_all: boolean } | undefined;
      return r?.archive_all ?? false;
    });
  }

  // ---- Ringba Recording Access credentials (write-only) ----

  async setRecordingAccess(tenantId: string, integrationId: string, username: string | null, password: string | null) {
    const clear = username === null;
    const blob = clear ? null : encryptCredentialJson({ username: username!, password: password! });
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `UPDATE integrations SET recording_access_blob = $2, recording_access_set_at = CASE WHEN $2::bytea IS NULL THEN NULL ELSE NOW() END,
                updated_at = NOW()
          WHERE id = $1::uuid AND deleted_at IS NULL AND connection_mode = 'api_sync' AND platform = 'ringba'`,
        [integrationId, blob],
      );
      if (r.rowCount === 0) throw new NotFoundException('Ringba API integration not found');
      return { recording_access_set: !clear };
    });
  }

  // ---- playback (admin, audited) ----

  async recording(
    tenantId: string,
    userId: string,
    kind: 'play' | 'download' | 'evidence_pack',
    platform: Platform,
    externalCallId: string,
  ): Promise<RecordingFetch> {
    const res = await this.recordings.fetch(kind === 'evidence_pack' ? 'evidence' : 'audio', {
      tenant_id: tenantId,
      platform,
      external_call_id: externalCallId,
    });
    await this.pg.withTenant(tenantId, async (c) => {
      const rec = (await c.query(`SELECT id::text FROM call_recordings WHERE platform = $1 AND external_call_id = $2`, [platform, externalCallId]))
        .rows[0] as { id: string } | undefined;
      await c.query(
        `INSERT INTO recording_access_audit (tenant_id, user_id, recording_id, platform, external_call_id, action, outcome)
         VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)`,
        [tenantId, userId, rec?.id ?? null, platform, externalCallId, kind, res.ok ? 'served' : 'failed'],
      );
    });
    return res;
  }

  /** Audit a refused attempt (a non-admin) without touching the archive. */
  async auditRefused(tenantId: string, userId: string, kind: 'play' | 'download' | 'evidence_pack', platform: Platform, externalCallId: string) {
    await this.pg.withTenant(tenantId, async (c) => {
      await c.query(
        `INSERT INTO recording_access_audit (tenant_id, user_id, platform, external_call_id, action, outcome)
         VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'refused')`,
        [tenantId, userId, platform, externalCallId, kind],
      );
    });
  }
}
