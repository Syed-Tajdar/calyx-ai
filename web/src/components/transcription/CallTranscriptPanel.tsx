'use client';

import { Download, FileArchive, Mic, Play, Search } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  ApiError,
  CallTranscriptView,
  fetchCallTranscript,
  fetchMe,
  fetchRecordingBlob,
  requestTranscription,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import {
  canRequestTranscription,
  filterUtterances,
  formatBytes,
  formatOffset,
  speakerLabel,
  stateLabel,
} from '@/lib/transcript';

interface Props {
  platform: string;
  callId: string;
}

function stateTone(state: string): string {
  switch (state) {
    case 'done':
      return 'bg-emerald-500/15 text-emerald-400';
    case 'failed':
      return 'bg-red-500/15 text-red-400';
    case 'no_recording':
      return 'bg-white/5 text-calyx-muted';
    case 'none':
      return 'bg-white/5 text-calyx-muted';
    default:
      return 'bg-amber-500/15 text-amber-400';
  }
}

function saveBlob(blob: Blob, name: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

/**
 * v2 Phase 2B — Transcript panel on call detail (behind NEXT_PUBLIC_FEATURE_TRANSCRIPTION).
 * Caller/agent (or speaker numbers until 2C labels roles), timestamps, search, and the state.
 * Admins also get Transcribe, the audio player and the dispute evidence pack — all through the API
 * proxy, every play/download audited; the page only ever holds a blob: URL of the verified bytes.
 */
export function CallTranscriptPanel({ platform, callId }: Props): JSX.Element {
  const [view, setView] = useState<CallTranscriptView | null>(null);
  const [error, setError] = useState('');
  const [role, setRole] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [busy, setBusy] = useState<'' | 'transcribe' | 'play' | 'download' | 'evidence'>('');
  const isAdmin = role === 'admin';

  const load = useCallback(async () => {
    const token = getToken();
    if (!token) return;
    try {
      setView(await fetchCallTranscript(token, platform, callId));
      setError('');
    } catch (err: unknown) {
      setError(err instanceof ApiError ? err.message : 'Transcript could not be loaded');
    }
  }, [platform, callId]);

  useEffect(() => {
    void load();
    const token = getToken();
    if (token) fetchMe(token).then((me) => setRole(me.role)).catch(() => setRole(null));
  }, [load]);

  useEffect(() => () => {
    if (audioUrl) URL.revokeObjectURL(audioUrl);
  }, [audioUrl]);

  const utterances = useMemo(() => filterUtterances(view?.transcript?.utterances ?? [], query), [view, query]);
  const archived = view?.recording?.archive_state === 'archived';

  async function transcribe(): Promise<void> {
    const token = getToken();
    if (!token) return;
    setBusy('transcribe');
    try {
      await requestTranscription(token, platform, callId);
      toast.success('Queued for transcription');
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Could not queue the call');
    } finally {
      setBusy('');
    }
  }

  async function fetchAudio(kind: 'play' | 'download' | 'evidence'): Promise<void> {
    const token = getToken();
    if (!token) return;
    setBusy(kind);
    try {
      const blob = await fetchRecordingBlob(token, platform, callId, kind);
      const safeId = callId.replace(/[^A-Za-z0-9_-]/g, '');
      if (kind === 'play') {
        if (audioUrl) URL.revokeObjectURL(audioUrl);
        setAudioUrl(URL.createObjectURL(blob));
      } else if (kind === 'download') {
        saveBlob(blob, `calyx-recording-${platform}-${safeId}.${blob.type.includes('wav') ? 'wav' : 'mp3'}`);
      } else {
        saveBlob(blob, `calyx-evidence-${platform}-${safeId}.zip`);
      }
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Recording not available');
    } finally {
      setBusy('');
    }
  }

  const state = view?.state ?? 'none';

  return (
    <div className="calyx-card overflow-hidden" data-testid="call-transcript-panel">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-calyx-border px-4 py-3">
        <div>
          <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Transcript</p>
          <span
            className={`mt-1 inline-block rounded-full px-2 py-0.5 text-[10px] font-medium ${stateTone(state)}`}
            data-testid="transcript-state"
          >
            {stateLabel(state, view?.no_recording_reason ?? null)}
          </span>
          {view?.transcript && (
            <p className="mt-1 text-[10px] text-calyx-muted">
              {view.transcript.language ? `${view.transcript.language.toUpperCase()} · ` : ''}
              {view.transcript.model} · v{view.transcript.version} · redaction {view.transcript.redaction}
            </p>
          )}
          {state === 'failed' && view?.error && <p className="mt-1 text-[10px] text-red-400">{view.error}</p>}
        </div>
        {isAdmin && (
          <div className="flex flex-wrap gap-1">
            {canRequestTranscription(state) && (
              <Button type="button" size="sm" variant="outline" disabled={busy !== ''} onClick={() => void transcribe()}>
                <Mic className="mr-1 h-3.5 w-3.5" /> Transcribe
              </Button>
            )}
            {archived && (
              <>
                <Button type="button" size="sm" variant="ghost" disabled={busy !== ''} onClick={() => void fetchAudio('play')} title="Play (audited)">
                  <Play className="mr-1 h-3.5 w-3.5" /> Play
                </Button>
                <Button type="button" size="sm" variant="ghost" disabled={busy !== ''} onClick={() => void fetchAudio('download')} title="Download (audited)">
                  <Download className="h-3.5 w-3.5" />
                </Button>
                <Button type="button" size="sm" variant="ghost" disabled={busy !== ''} onClick={() => void fetchAudio('evidence')} title="Dispute evidence pack (audited)">
                  <FileArchive className="mr-1 h-3.5 w-3.5" /> Evidence pack
                </Button>
              </>
            )}
          </div>
        )}
      </div>

      {isAdmin && audioUrl && (
        <div className="border-b border-calyx-border px-4 py-2">
          {/* eslint-disable-next-line jsx-a11y/media-has-caption -- the transcript below is the caption */}
          <audio controls autoPlay src={audioUrl} className="w-full" data-testid="recording-player" />
        </div>
      )}
      {isAdmin && view?.recording && (
        <p className="border-b border-calyx-border px-4 py-2 font-mono text-[10px] text-calyx-muted" title="SHA-256 of the archived audio">
          {view.recording.archive_state === 'archived' ? 'Archived' : view.recording.archive_state === 'expired' ? 'Expired (retention)' : 'Not archived'}
          {' · '}
          {formatBytes(view.recording.bytes)} · sha256 {view.recording.sha256.slice(0, 16)}…
        </p>
      )}

      {error && <p className="px-4 py-3 text-sm text-red-400">{error}</p>}

      {view?.transcript && (
        <>
          <div className="flex items-center gap-2 border-b border-calyx-border px-4 py-2">
            <Search className="h-3.5 w-3.5 text-calyx-muted" />
            <input
              aria-label="Search in transcript"
              className="calyx-input h-7 w-full text-xs"
              placeholder="Search in transcript"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <ol className="max-h-96 space-y-2 overflow-y-auto px-4 py-3" data-testid="transcript-utterances">
            {utterances.map((u) => (
              <li key={u.idx} className="text-sm leading-relaxed">
                <span className="mr-2 font-mono text-[10px] text-calyx-muted">{formatOffset(u.start_ms)}</span>
                <span className={`mr-2 text-xs font-semibold ${u.role === 'caller' || (u.role === 'unknown' && (u.speaker ?? 0) % 2 === 1) ? 'text-sky-400' : 'text-calyx-teal'}`}>
                  {speakerLabel(u)}
                </span>
                <span className="text-calyx-text/90">{u.text}</span>
              </li>
            ))}
            {utterances.length === 0 && <li className="text-xs text-calyx-muted">No match.</li>}
          </ol>
        </>
      )}
      {!view?.transcript && !error && state !== 'none' && state !== 'no_recording' && state !== 'failed' && (
        <p className="px-4 py-3 text-xs text-calyx-muted">The transcript appears here when it is ready.</p>
      )}
    </div>
  );
}
