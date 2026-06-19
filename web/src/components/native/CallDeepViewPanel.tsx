import type { ReactNode } from 'react';
import type { CallDeepView } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

interface CallDeepViewPanelProps {
  view: CallDeepView | null;
  loading?: boolean;
}

function Section({ title, children }: { title: string; children: ReactNode }): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-xs font-bold uppercase tracking-wider text-calyx-accent">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export function CallDeepViewPanel({ view, loading }: CallDeepViewPanelProps): JSX.Element {
  if (loading) {
    return (
      <Card>
        <CardContent className="pt-6">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="mt-3 h-20 w-full" />
        </CardContent>
      </Card>
    );
  }

  if (!view) {
    return (
      <Card>
        <CardContent className="pt-6">
          <p className="text-sm text-calyx-muted">Deep view unavailable for this call.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
        Native Deep View
      </p>
      {view.ping && (
        <Section title="Ping / RTB">
          <dl className="grid grid-cols-2 gap-2 text-xs">
            <div>
              <dt className="text-calyx-muted">Platform</dt>
              <dd className="text-calyx-text">{view.ping.platform}</dd>
            </div>
            <div>
              <dt className="text-calyx-muted">State</dt>
              <dd className="text-calyx-text">{view.ping.state}</dd>
            </div>
            <div>
              <dt className="text-calyx-muted">Revenue</dt>
              <dd className="text-calyx-text">${view.ping.revenue.toFixed(2)}</dd>
            </div>
            <div>
              <dt className="text-calyx-muted">Publisher</dt>
              <dd className="truncate text-calyx-text">{view.ping.publisher_id || '—'}</dd>
            </div>
          </dl>
        </Section>
      )}
      {view.scoring && (
        <Section title="Call Quality">
          <p className="text-2xl font-bold text-calyx-text">{view.scoring.overall_score.toFixed(1)}</p>
          <p className="text-xs capitalize text-calyx-muted">{view.scoring.sentiment}</p>
        </Section>
      )}
      {view.compliance && (
        <Section title="TCPA Risk">
          <p className="text-2xl font-bold text-calyx-warning">{view.compliance.risk_score}</p>
          <p className="text-xs text-calyx-muted">
            Attestation {view.compliance.attestation_level} · $
            {view.compliance.exposure_usd.toFixed(2)} exposure
          </p>
        </Section>
      )}
      {view.attribution && (
        <Section title="Ad Attribution">
          <p className="text-sm capitalize text-calyx-text">
            {view.attribution.platform} · {view.attribution.status}
          </p>
          <p className="truncate text-xs text-calyx-muted">{view.attribution.click_id}</p>
        </Section>
      )}
      {view.enrichment && view.enrichment.attribute_count > 0 && (
        <Section title="Enrichment">
          <p className="text-sm text-calyx-text">{view.enrichment.attribute_count} attributes</p>
        </Section>
      )}
    </div>
  );
}
