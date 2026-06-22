'use client';

import Link from 'next/link';
import { Phone } from 'lucide-react';
import { CallJourneyTimeline, type JourneyStep } from '@/components/calyx/CallJourneyTimeline';
import { SubsectionHeader } from '@/components/dashboard/SubsectionHeader';
import { IntelligenceHubCards } from '@/components/intelligence/IntelligenceHubCards';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { PageHeader } from '@/components/ui/PageHeader';

const SAMPLE_JOURNEY: JourneyStep[] = [
  { id: 'ping', label: 'Ping', state: 'complete' },
  { id: 'enrich', label: 'Enrich', state: 'complete' },
  { id: 'score', label: 'Score', state: 'complete' },
  { id: 'comp', label: 'Compliance', state: 'active' },
  { id: 'attr', label: 'Attribution', state: 'pending' },
];

export default function IntelligencePage(): JSX.Element {
  return (
    <div className="space-y-8">
      <PageHeader
        title="Calyx Intelligence"
        description="Ask AI, TCPA compliance, and ad attribution — your call data, interrogated."
      />
      <IntelligenceSubNav />
      <IntelligenceHubCards />
      <section>
        <SubsectionHeader title="Call Journey · sample" icon={Phone} />
        <CallJourneyTimeline steps={SAMPLE_JOURNEY} />
      </section>
      <p className="text-center">
        <Link href="/reports" className="text-xs text-calyx-muted transition hover:text-calyx-accent">
          Financial reports →
        </Link>
      </p>
    </div>
  );
}
