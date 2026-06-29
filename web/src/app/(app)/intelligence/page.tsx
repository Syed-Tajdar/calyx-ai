'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Phone } from 'lucide-react';
import { CallJourneyTimeline, type JourneyStep } from '@/components/calyx/CallJourneyTimeline';
import { SubsectionHeader } from '@/components/dashboard/SubsectionHeader';
import { IntelligenceHubCards } from '@/components/intelligence/IntelligenceHubCards';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { PageHeader } from '@/components/ui/PageHeader';
import { ApiError, fetchScores } from '@/lib/api';
import { getToken } from '@/lib/auth';

const SAMPLE_JOURNEY: JourneyStep[] = [
  { id: 'ping', label: 'Ping', state: 'complete' },
  { id: 'enrich', label: 'Enrich', state: 'complete' },
  { id: 'score', label: 'Score', state: 'complete' },
  { id: 'comp', label: 'Compliance', state: 'active' },
  { id: 'attr', label: 'Attribution', state: 'pending' },
];

export default function IntelligencePage(): JSX.Element {
  const router = useRouter();
  const [latestCallId, setLatestCallId] = useState<string | null>(null);

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    fetchScores(token, { limit: 1, offset: 0 })
      .then((res) => {
        if (res && 'data' in res && res.data[0]) {
          setLatestCallId(res.data[0].call_id);
        }
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
        }
      });
  }, [router]);

  return (
    <div className="space-y-8">
      <PageHeader
        title="Calyx Intelligence"
        description="Ask AI, TCPA compliance, and ad attribution — your call data, interrogated."
      />
      <IntelligenceSubNav />
      <IntelligenceHubCards />
      <section>
        <SubsectionHeader title="Call Journey" icon={Phone} />
        {latestCallId ? (
          <div className="space-y-3">
            <CallJourneyTimeline steps={SAMPLE_JOURNEY} />
            <p className="text-center text-sm text-calyx-muted">
              Latest scored call:{' '}
              <Link
                href={`/calls/${encodeURIComponent(latestCallId)}`}
                className="font-mono text-calyx-teal hover:underline"
              >
                {latestCallId}
              </Link>
            </p>
          </div>
        ) : (
          <p className="text-sm text-calyx-muted">No scored calls yet — journey preview unavailable.</p>
        )}
      </section>
      <p className="text-center">
        <Link href="/reports" className="text-xs text-calyx-muted transition hover:text-calyx-teal">
          Financial reports →
        </Link>
      </p>
    </div>
  );
}
