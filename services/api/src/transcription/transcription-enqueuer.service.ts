import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';
import { TranscriptionCandidatesService } from './transcription-candidates.service';
import { transcriptionEnabled, transcriptionSettings } from './transcription-config';
import { TranscriptionService } from './transcription.service';

export const ENQUEUE_INTERVAL_MS = 15 * 60 * 1000;
export const ENQUEUE_LOCK_KEY = 728_340_236;

interface LockClient {
  query(sql: string, params?: unknown[]): Promise<{ rows: Array<Record<string, unknown>> }>;
  release(): void;
}
interface LockPool {
  connect(): Promise<LockClient>;
}

export interface EnqueueSummary {
  ran: boolean;
  reason?: string;
  tenants: number;
  candidates: number;
  queued: number;
}

/**
 * Every 15 minutes: queue the calls that are (or BECAME, through a later sweep) converted — and, only
 * with TRANSCRIBE_SHOULD_CONVERT=true, should-have-converted — since the last TRANSCRIBE_LOOKBACK_HOURS.
 * A call is queued once (UNIQUE), so re-seeing it in the next window is harmless. With "Archive all
 * recorded calls" on, recorded calls that are not transcribed are queued archive-only. Idle while
 * FEATURE_TRANSCRIPTION is off. One instance at a time (advisory lock).
 */
@Injectable()
export class TranscriptionEnqueuerService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(TranscriptionEnqueuerService.name);
  private timer: NodeJS.Timeout | null = null;
  private firstRun: NodeJS.Timeout | null = null;
  private running = false;

  constructor(
    @Inject(PostgresService) private readonly pg: { getPool?: () => LockPool },
    private readonly candidates: TranscriptionCandidatesService,
    private readonly transcription: TranscriptionService,
  ) {}

  onModuleInit(): void {
    if (process.env.NODE_ENV === 'test') return;
    this.timer = setInterval(() => void this.tick(), ENQUEUE_INTERVAL_MS);
    this.timer.unref();
    this.firstRun = setTimeout(() => void this.tick(), 90_000);
    this.firstRun.unref();
  }

  onModuleDestroy(): void {
    if (this.timer) clearInterval(this.timer);
    if (this.firstRun) clearTimeout(this.firstRun);
  }

  async tick(now = new Date()): Promise<EnqueueSummary> {
    const summary: EnqueueSummary = { ran: false, tenants: 0, candidates: 0, queued: 0 };
    if (!transcriptionEnabled()) return { ...summary, reason: 'FEATURE_TRANSCRIPTION is off' };
    if (this.running) return { ...summary, reason: 'previous tick still running' };
    const pool = this.pg.getPool?.();
    if (!pool) return { ...summary, reason: 'no pool' };
    this.running = true;
    const lock = await pool.connect();
    try {
      const got = await lock.query('SELECT pg_try_advisory_lock($1) AS locked', [ENQUEUE_LOCK_KEY]);
      if (!got.rows[0]?.locked) return { ...summary, reason: 'another instance holds the lock' };
      try {
        summary.ran = true;
        const s = transcriptionSettings();
        const tenants = await lock.query('SELECT tenant_id::text AS tenant_id FROM list_transcription_tenants()');
        for (const t of tenants.rows) {
          const tenantId = String(t.tenant_id);
          summary.tenants++;
          try {
            const calls = await this.candidates.find({
              tenantId,
              receivedSince: new Date(now.getTime() - s.lookbackHours * 3_600_000),
              minSeconds: s.minSeconds,
              shouldConvert: s.shouldConvert,
              archiveAll: await this.transcription.archiveAll(tenantId),
              limit: 5_000,
            });
            summary.candidates += calls.length;
            summary.queued += await this.transcription.enqueue(tenantId, calls);
          } catch (err: unknown) {
            this.logger.error(`transcription enqueue failed for tenant ${tenantId}: ${(err as Error).message}`);
          }
        }
      } finally {
        await lock.query('SELECT pg_advisory_unlock($1)', [ENQUEUE_LOCK_KEY]);
      }
      this.logger.log(`transcription enqueue: tenants=${summary.tenants} candidates=${summary.candidates} queued=${summary.queued}`);
      return summary;
    } finally {
      lock.release();
      this.running = false;
    }
  }
}
