import type { TranscriptRecord } from '@/lib/api';

interface TranscriptPanelProps {
  transcript: TranscriptRecord | null;
  loading?: boolean;
}

export function TranscriptPanel({ transcript, loading }: TranscriptPanelProps): JSX.Element {
  if (loading) {
    return (
      <div className="loom-card p-4">
        <p className="text-sm text-loom-muted">Loading transcript…</p>
      </div>
    );
  }

  if (!transcript) {
    return (
      <div className="loom-card p-4">
        <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">Transcript</p>
        <p className="mt-2 text-sm text-loom-muted">No transcript available for this call yet.</p>
      </div>
    );
  }

  return (
    <div className="loom-card overflow-hidden">
      <div className="border-b border-loom-border px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">Transcript</p>
        <p className="text-xs text-loom-muted">
          {transcript.duration_seconds}s · {transcript.word_count} words
        </p>
      </div>
      <div className="max-h-64 overflow-y-auto px-4 py-3">
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-white/90">
          {transcript.transcript_text}
        </p>
      </div>
    </div>
  );
}
