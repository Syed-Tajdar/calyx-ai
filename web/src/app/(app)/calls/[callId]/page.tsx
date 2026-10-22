'use client';

import Link from 'next/link';
import { Check, ChevronRight, Copy, ExternalLink } from 'lucide-react';
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
import { ConversionActions } from '@/components/native/ConversionActions';
import { Button } from '@/components/ui/button';
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
import { buildPlatformCallUrl, platformDeepLinkCopiesCallId } from '@/lib/platform-deep-links';
import { formatPlatformLabel } from '@/lib/platform-options';
import { CallTranscriptPanel } from '@/components/transcription/CallTranscriptPanel';
import { isFeatureEnabled, SHOW_MOCK_PANELS } from '@/lib/v1-features';

import { toast } from 'sonner';

/** v2 Phase 2B: real transcripts (Deepgram) — separate from the mock panels. */
const SHOW_TRANSCRIPTION = isFeatureEnabled('transcription');

function CallDetailContent(): JSX.Element {
  const params = useParams();
  const searchParams = useSearchParams();
  const router = useRouter();
  const callId = decodeURIComponent(params.callId as string);
  const platformParam = searchParams.get('platform') ?? '';

  const [score, setScore] = useState<ScoreRecord | null>(null);
  const [transcript, setTranscript] = useState<TranscriptRecord | null>(null);
  const [deepView, setDeepView] = useState<CallDeepView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  /** Bumped after a write-back so the deep view refetches and the status reflects Ringba. */
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    setError('');

    Promise.all([
      SHOW_MOCK_PANELS ? fetchScores(token, { callId }).catch(() => null) : Promise.resolve(null),
      SHOW_MOCK_PANELS
        ? fetchTranscript(token, callId).catch(() => null)
        : Promise.resolve(null),
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
  }, [router, callId, reloadKey]);

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

  const aniHash = deepView?.ping?.ani_hash ?? '';
  const hasEnrichment =
    Boolean(aniHash) ||
    (deepView?.enrichment != null && deepView.enrichment.attribute_count > 0);

  const journeySteps = useMemo(
    () =>
      buildJourneySteps({
        hasPing: deepView?.ping != null,
        hasEnrichment,
        hasScore: deepView?.scoring != null || score != null,
        hasCompliance: deepView?.compliance != null,
        complianceRisk: deepView?.compliance?.risk_score,
        hasAttribution: deepView?.attribution != null,
        includeLaterSteps: SHOW_MOCK_PANELS,
      }),
    [deepView, score, hasEnrichment],
  );

  const platform =
    deepView?.ping?.platform ?? score?.platform ?? displayScore?.platform ?? platformParam;
  const platformCallUrl = buildPlatformCallUrl(platform, callId);

  async function handleCopyCallId(): Promise<void> {
    try {
      await navigator.clipboard.writeText(callId);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // title / labelled field still show full id
    }
  }

  async function handleOpenInPlatform(): Promise<void> {
    if (!platformCallUrl) return;
    const copiesId = platformDeepLinkCopiesCallId(platform);
    if (copiesId) {
      try {
        await navigator.clipboard.writeText(callId);
      } catch {
        toast.error('Could not copy Call ID — copy it manually from the header');
        return;
      }
    }
    // Open after clipboard write so the id is ready the moment the tab appears.
    window.open(platformCallUrl, '_blank', 'noopener,noreferrer');
    if (copiesId) {
      toast.message(
        'Call ID copied — paste into the Inbound Call ID filter in Ringba.',
      );
    }
  }

  return (
    <>
      <nav className="mb-4 flex items-center gap-1 text-sm text-calyx-muted">
        <Link href="/pings" className="hover:text-calyx-accent">
          Pings
        </Link>
        <ChevronRight className="h-4 w-4" />
        <span className="truncate font-mono text-calyx-text" title={callId}>
          {callId}
        </span>
      </nav>

      <PageHeader
        title="Call detail"
        description={
          <span className="flex flex-wrap items-center gap-2">
            <span className="text-calyx-muted">Call ID</span>
            <span className="break-all font-mono text-calyx-text" title={callId}>
              {callId}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-calyx-muted hover:text-calyx-text"
              aria-label={copied ? 'Copied call ID' : 'Copy call ID'}
              title={copied ? 'Copied' : 'Copy call ID'}
              onClick={() => {
                void handleCopyCallId();
              }}
            >
              {copied ? (
                <Check className="h-3.5 w-3.5 text-calyx-success" />
              ) : (
                <Copy className="h-3.5 w-3.5" />
              )}
            </Button>
          </span>
        }
        action={
          platformCallUrl ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-2"
              onClick={() => {
                void handleOpenInPlatform();
              }}
            >
              <ExternalLink className="h-4 w-4" />
              Open in {formatPlatformLabel(platform)}
            </Button>
          ) : undefined
        }
      />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-8">
          <CallJourneyTimeline steps={journeySteps} />
          <div className={SHOW_MOCK_PANELS ? 'grid gap-6 lg:grid-cols-2' : 'space-y-4'}>
            <div className="space-y-4">
              <ConversionActions
                callId={callId}
                platform={platform}
                hasConverted={deepView?.ping?.has_converted ?? false}
                revenue={deepView?.ping?.revenue ?? 0}
                onChanged={() => setReloadKey((k) => k + 1)}
              />
              <CallDeepViewPanel view={deepView} />
              {SHOW_TRANSCRIPTION && (platform === 'ringba' || platform === 'retreaver') && (
                <CallTranscriptPanel platform={platform} callId={callId} />
              )}
              {SHOW_MOCK_PANELS && <ScoreCard score={displayScore} />}
            </div>
            {SHOW_MOCK_PANELS && (
              <div className="space-y-4">
                <TranscriptPanel transcript={transcript} />
                {aniHash ? (
                  <EnrichmentPanel aniHash={aniHash} />
                ) : (
                  <div className="calyx-card p-4 text-sm text-calyx-muted">
                    No enrichment available for this call yet.
                  </div>
                )}
              </div>
            )}
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
