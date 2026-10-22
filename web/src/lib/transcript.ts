/**
 * v2 Phase 2B — transcript panel helpers (pure, tested in transcript.test.ts).
 */

export interface TranscriptUtterance {
  idx: number;
  speaker: number | null;
  role: string;
  start_ms: number;
  end_ms: number;
  text: string;
}

/** 0:07, 1:05, 1:02:03 */
export function formatOffset(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const ss = String(s).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

/**
 * Caller / Agent once roles are labelled (2C); until then the diarization speaker number.
 * Speaker 0 is whoever spoke first, not necessarily the agent.
 */
export function speakerLabel(u: Pick<TranscriptUtterance, 'role' | 'speaker'>): string {
  if (u.role === 'caller') return 'Caller';
  if (u.role === 'agent') return 'Agent';
  return u.speaker === null || u.speaker < 0 ? 'Speaker' : `Speaker ${u.speaker + 1}`;
}

/** Case-insensitive search within the transcript; an empty query keeps everything. */
export function filterUtterances(us: TranscriptUtterance[], query: string): TranscriptUtterance[] {
  const q = query.trim().toLowerCase();
  if (!q) return us;
  return us.filter((u) => u.text.toLowerCase().includes(q) || speakerLabel(u).toLowerCase().includes(q));
}

export type TranscriptState = 'none' | 'queued' | 'running' | 'done' | 'no_recording' | 'failed' | 'capped';

const NO_RECORDING: Record<string, string> = {
  not_answered: 'the call was not answered',
  recording_disabled: 'recording is off for this call',
  blocked_or_cut: 'the call was blocked or cut',
  fetch_failed: 'the recording could not be fetched',
  no_access: 'Calyx has no access to the recording (Ringba: set the Recording Access credentials)',
};

/** One line for the panel header. */
export function stateLabel(state: string, noRecordingReason: string | null = null): string {
  switch (state) {
    case 'done':
      return 'Ready';
    case 'queued':
      return 'Pending — queued';
    case 'running':
      return 'Pending — transcribing';
    case 'capped':
      return 'Pending — daily minutes cap reached, continues tomorrow (Eastern)';
    case 'no_recording':
      return `No recording — ${NO_RECORDING[noRecordingReason ?? ''] ?? 'none available'}`;
    case 'failed':
      return 'Failed';
    default:
      return 'Not transcribed';
  }
}

/** Whether the admin "Transcribe" button applies (nothing yet, or a final non-success state). */
export function canRequestTranscription(state: string): boolean {
  return state === 'none' || state === 'failed' || state === 'no_recording';
}

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = n;
  let i = 0;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}

export function formatUsd(n: number): string {
  return `$${(Number.isFinite(n) ? n : 0).toFixed(2)}`;
}
