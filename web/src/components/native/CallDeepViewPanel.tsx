import type { ReactNode } from 'react';
import type { CallDeepView } from '@/lib/api';

interface CallDeepViewPanelProps {
  view: CallDeepView | null;
  loading?: boolean;
}

function Section({ title, children }: { title: string; children: ReactNode }): JSX.Element {
  return (
    <div className="loom-card p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">{title}</p>
      <div className="mt-2">{children}</div>
    </div>
  );
}

export function CallDeepViewPanel({ view, loading }: CallDeepViewPanelProps): JSX.Element {
  if (loading) {
    return (
      <div className="loom-card p-4">
        <p className="text-sm text-loom-muted">Loading native deep view…</p>
      </div>
    );
  }

  if (!view) {
    return (
      <div className="loom-card p-4">
        <p className="text-sm text-loom-muted">Deep view unavailable for this call.</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">
        Native Deep View
      </p>
      {view.ping && (
        <Section title="Ping / RTB">
          <dl className="grid grid-cols-2 gap-2 text-xs">
            <div>
              <dt className="text-loom-muted">Platform</dt>
              <dd className="text-white">{view.ping.platform}</dd>
            </div>
            <div>
              <dt className="text-loom-muted">State</dt>
              <dd className="text-white">{view.ping.state}</dd>
            </div>
            <div>
              <dt className="text-loom-muted">Revenue</dt>
              <dd className="text-white">${view.ping.revenue.toFixed(2)}</dd>
            </div>
            <div>
              <dt className="text-loom-muted">Publisher</dt>
              <dd className="truncate text-white">{view.ping.publisher_id || '—'}</dd>
            </div>
          </dl>
        </Section>
      )}
      {view.scoring && (
        <Section title="Call Quality">
          <p className="text-2xl font-bold text-white">{view.scoring.overall_score.toFixed(1)}</p>
          <p className="text-xs capitalize text-loom-muted">{view.scoring.sentiment}</p>
        </Section>
      )}
      {view.compliance && (
        <Section title="TCPA Risk">
          <p className="text-2xl font-bold text-amber-400">{view.compliance.risk_score}</p>
          <p className="text-xs text-loom-muted">
            Attestation {view.compliance.attestation_level} · $
            {view.compliance.exposure_usd.toFixed(2)} exposure
          </p>
        </Section>
      )}
      {view.attribution && (
        <Section title="Ad Attribution">
          <p className="text-sm capitalize text-white">
            {view.attribution.platform} · {view.attribution.status}
          </p>
          <p className="text-xs text-loom-muted truncate">{view.attribution.click_id}</p>
        </Section>
      )}
      {view.enrichment && view.enrichment.attribute_count > 0 && (
        <Section title="Enrichment">
          <p className="text-sm text-white">{view.enrichment.attribute_count} attributes</p>
        </Section>
      )}
    </div>
  );
}
