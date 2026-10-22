/**
 * v2 Phase 2B — Transcription settings read by the API. Everything is off unless
 * FEATURE_TRANSCRIPTION=true (the routes answer 404 and the enqueue job is idle).
 * Owner decisions: TRANSCRIBE_SHOULD_CONVERT off (converted calls only first), cap 800 min/day (Q5).
 */

type Env = Record<string, string | undefined>;

function num(v: string | undefined, fallback: number, min = 0): number {
  const n = Number(v);
  return Number.isFinite(n) && n >= min && v !== undefined && v !== '' ? n : fallback;
}

export function transcriptionEnabled(env: Env = process.env): boolean {
  return env.FEATURE_TRANSCRIPTION === 'true';
}

export interface TranscriptionSettings {
  /** The 15-minute job that queues converted calls by itself (TRANSCRIBE_AUTO, default off: button only). */
  auto: boolean;
  /**
   * TRANSCRIBE_AUTO_SINCE (ISO time, UTC): the automatic job never looks before this moment, so switching it
   * on never starts an unplanned backfill. Set it to the switch-on time. Empty or invalid: the API's start
   * time is used instead (autoSinceValid tells which).
   */
  autoSince: Date | null;
  autoSinceValid: boolean;
  /** Admins may start a backfill (TRANSCRIBE_BACKFILL, default off; the estimate always works). */
  backfill: boolean;
  /** Calls shorter than this are never transcribed (TRANSCRIBE_MIN_SECONDS, default 20). */
  minSeconds: number;
  /** Also auto-transcribe "should have converted" calls (TRANSCRIBE_SHOULD_CONVERT, default off). */
  shouldConvert: boolean;
  /** Hard stop per Eastern day, enforced by the worker (TRANSCRIBE_DAILY_MINUTES_CAP, default 800). */
  dailyCapMinutes: number;
  /** $ per minute for estimates: Nova-3 $0.0043 + redaction $0.0020 (TRANSCRIBE_PRICE_PER_MINUTE). */
  pricePerMinute: number;
  /** $ per GB-month for the archive estimate (B2 $6.95/TB; ARCHIVE_PRICE_PER_GB_MONTH). */
  archivePricePerGbMonth: number;
  /** How far back the enqueue job looks for calls that became converted (hours, default 48). */
  lookbackHours: number;
  /** Backfill: default and maximum days. */
  backfillDefaultDays: number;
  backfillMaxDays: number;
}

export function transcriptionSettings(env: Env = process.env): TranscriptionSettings {
  const rawSince = (env.TRANSCRIBE_AUTO_SINCE ?? '').trim();
  const since = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/.test(rawSince) ? new Date(rawSince) : null;
  const sinceOk = since !== null && Number.isFinite(since.getTime());
  return {
    auto: env.TRANSCRIBE_AUTO === 'true',
    autoSince: sinceOk ? since : null,
    autoSinceValid: sinceOk,
    backfill: env.TRANSCRIBE_BACKFILL === 'true',
    minSeconds: num(env.TRANSCRIBE_MIN_SECONDS, 20),
    shouldConvert: env.TRANSCRIBE_SHOULD_CONVERT === 'true',
    dailyCapMinutes: num(env.TRANSCRIBE_DAILY_MINUTES_CAP, 800),
    pricePerMinute: num(env.TRANSCRIBE_PRICE_PER_MINUTE, 0.0063),
    archivePricePerGbMonth: num(env.ARCHIVE_PRICE_PER_GB_MONTH, 0.00695),
    lookbackHours: num(env.TRANSCRIBE_LOOKBACK_HOURS, 48, 1),
    backfillDefaultDays: 7,
    backfillMaxDays: 30,
  };
}

/** Average recorded minutes assumed for a call whose duration is unknown (cost model §6). */
export const DEFAULT_CALL_MINUTES = 6;
