import type { TranscriptRecord } from '@/lib/api';

interface TranscriptPanelProps {
  transcript: TranscriptRecord | null;
  loading?: boolean;
}

export function TranscriptPanel({ transcript, loading }: TranscriptPanelProps): JSX.Element {
  if (loading) {
    return (
      <div className="calyx-card p-4">
        <p className="text-sm text-calyx-muted">Loading transcript…</p>
      </div>
    );
  }

  if (!transcript) {
    return (
      <div className="calyx-card p-4">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Transcript</p>
        <p className="mt-2 text-sm text-calyx-muted">No transcript available for this call yet.</p>
      </div>
    );
  }

  return (
    <div className="calyx-card overflow-hidden">
      <div className="border-b border-calyx-border px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Transcript</p>
        <p className="text-xs text-calyx-muted">
          {transcript.duration_seconds}s · {transcript.word_count} words
        </p>
      </div>
      <div className="max-h-64 overflow-y-auto px-4 py-3">
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-calyx-text/90">
          {transcript.transcript_text}
        </p>
      </div>
    </div>
  );
}
