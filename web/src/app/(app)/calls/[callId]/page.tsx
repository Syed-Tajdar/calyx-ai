'use client';

import Link from 'next/link';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { EnrichmentPanel } from '@/components/enrichment/EnrichmentPanel';
import { ScoreCard } from '@/components/scoring/ScoreCard';
import { TranscriptPanel } from '@/components/scoring/TranscriptPanel';
import { CallDeepViewPanel } from '@/components/native/CallDeepViewPanel';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  ApiError,
  CallDeepView,
  fetchCallDeepView,
  fetchScores,
  fetchTranscript,
  ScoreRecord,
  TranscriptRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

function CallDetailContent(): JSX.Element {
  const params = useParams();
  const searchParams = useSearchParams();
  const router = useRouter();
  const callId = decodeURIComponent(params.callId as string);
  const aniHash = searchParams.get('aniHash') ?? '';

  const [score, setScore] = useState<ScoreRecord | null>(null);
  const [transcript, setTranscript] = useState<TranscriptRecord | null>(null);
  const [deepView, setDeepView] = useState<CallDeepView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([
      fetchScores(token, { callId }),
      fetchTranscript(token, callId),
      fetchCallDeepView(token, callId).catch(() => null),
    ])
      .then(([scoreRes, transcriptRes, deepRes]) => {
        if (scoreRes && !('data' in scoreRes)) {
          setScore(scoreRes);
        }
        setTranscript(transcriptRes);
        setDeepView(deepRes);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load call details');
      })
      .finally(() => setLoading(false));
  }, [router, callId]);

  return (
    <>
      <AppHeader />
      <div className="mb-4">
        <Link href="/pings" className="text-xs text-loom-accent hover:underline">
          ← Back to pings
        </Link>
        <h2 className="mt-2 font-mono text-sm text-white">{callId}</h2>
      </div>

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-4">
          <CallDeepViewPanel view={deepView} />
          <ScoreCard score={score} />
          <TranscriptPanel transcript={transcript} />
          {aniHash ? (
            <EnrichmentPanel aniHash={aniHash} />
          ) : (
            <div className="loom-card p-4 text-sm text-loom-muted">
              No ANI hash provided. Open this call from the pings table to view enrichment.
            </div>
          )}
        </div>
      )}
    </>
  );
}

export default function CallDetailPage(): JSX.Element {
  return (
    <Suspense fallback={<PageLoader />}>
      <CallDetailContent />
    </Suspense>
  );
}
