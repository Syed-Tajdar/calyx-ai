'use client';

import Link from 'next/link';
import { ChevronRight } from 'lucide-react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useMemo, useState } from 'react';
import {
  buildJourneySteps,
  CallJourneyTimeline,
} from '@/components/calyx/CallJourneyTimeline';
import { EnrichmentPanel } from '@/components/enrichment/EnrichmentPanel';
import { ScoreCard } from '@/components/scoring/ScoreCard';
import { TranscriptPanel } from '@/components/scoring/TranscriptPanel';
import { CallDeepViewPanel } from '@/components/native/CallDeepViewPanel';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
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

    setLoading(true);
    setError('');

    Promise.all([
      fetchScores(token, { callId }).catch(() => null),
      fetchTranscript(token, callId).catch(() => null),
      fetchCallDeepView(token, callId).catch(() => null),
    ])
      .then(([scoreRes, transcriptRes, deepRes]) => {
        const scoreRecord =
          scoreRes && !('data' in scoreRes) ? scoreRes : null;
        const hasScore = scoreRecord != null;
        const hasTranscript = transcriptRes != null;
        const hasDeepView =
          deepRes != null &&
          (deepRes.ping != null ||
            deepRes.scoring != null ||
            deepRes.compliance != null ||
            deepRes.attribution != null);

        if (!hasScore && !hasTranscript && !hasDeepView) {
          setError('No data found for this call ID');
          return;
        }

        setScore(scoreRecord);
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

  const displayScore = useMemo((): ScoreRecord | null => {
    if (score) return score;
    if (!deepView?.scoring) return null;
    return {
      tenant_id: deepView.tenant_id,
      call_id: deepView.call_id,
      platform: deepView.ping?.platform ?? 'unknown',
      overall_score: deepView.scoring.overall_score,
      sentiment: deepView.scoring.sentiment,
      rules_passed: deepView.scoring.rules_passed,
      rules_failed: deepView.scoring.rules_failed,
      created_at: deepView.scoring.scored_at,
    };
  }, [score, deepView]);

  const journeySteps = useMemo(
    () =>
      buildJourneySteps({
        hasPing: deepView?.ping != null,
        hasEnrichment: Boolean(aniHash),
        hasScore: deepView?.scoring != null || score != null,
        hasCompliance: deepView?.compliance != null,
        complianceRisk: deepView?.compliance?.risk_score,
        hasAttribution: deepView?.attribution != null,
      }),
    [deepView, score, aniHash],
  );

  return (
    <>
      <nav className="mb-4 flex items-center gap-1 text-sm text-calyx-muted">
        <Link href="/pings" className="hover:text-calyx-accent">
          Pings
        </Link>
        <ChevronRight className="h-4 w-4" />
        <span className="truncate font-mono text-calyx-text">{callId}</span>
      </nav>

      <PageHeader
        title="Call detail"
        description={callId}
      />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-8">
          <CallJourneyTimeline steps={journeySteps} />
          <div className="grid gap-6 lg:grid-cols-2">
            <div className="space-y-4">
              <CallDeepViewPanel view={deepView} />
              <ScoreCard score={displayScore} />
            </div>
            <div className="space-y-4">
              <TranscriptPanel transcript={transcript} />
              {aniHash ? (
                <EnrichmentPanel aniHash={aniHash} />
              ) : (
                <div className="calyx-card p-4 text-sm text-calyx-muted">
                  No ANI hash provided. Open this call from the pings table to view enrichment.
                </div>
              )}
            </div>
          </div>
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
