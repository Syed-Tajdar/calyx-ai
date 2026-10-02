import type { ReactNode } from 'react';
import type { CallDeepView } from '@/lib/api';
import { conversionStateLabel } from '@/lib/conversion-label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';

interface CallDeepViewPanelProps {
  view: CallDeepView | null;
  loading?: boolean;
}

/** Prefer platform occurred_at; fall back to ingestion received_at. */
function formatCallTimestamp(occurredAt: string | undefined, receivedAt: string): string {
  const raw = (occurredAt && occurredAt.trim() !== '' ? occurredAt : receivedAt).trim();
  if (!raw) return '—';
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  return d.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short',
  });
}

function formatSeconds(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) return '—';
  const s = Math.floor(sec);
  const m = Math.floor(s / 60);
  const r = s % 60;
  if (m === 0) return `${r}s`;
  return `${m}m ${r.toString().padStart(2, '0')}s`;
}

const WEBHOOK_MISSING =
  'Not provided via webhook — available in API mode';

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

function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <div>
      <dt className="text-calyx-muted">{label}</dt>
      <dd className="text-calyx-text">{children}</dd>
    </div>
  );
}

function NamedId({
  name,
  id,
  syncPath,
}: {
  name: string;
  id: string;
  syncPath: string;
}): JSX.Element {
  const hasName = name.trim() !== '';
  const hasId = id.trim() !== '';
  if (!hasName && !hasId) {
    if (syncPath === 'webhook') {
      return <span className="text-calyx-muted">{WEBHOOK_MISSING}</span>;
    }
    return <span>—</span>;
  }
  if (hasName) {
    return (
      <span className="block truncate" title={hasId ? `ID: ${id}` : name}>
        {name}
        {hasId ? (
          <span className="mt-0.5 block truncate font-mono text-[10px] text-calyx-muted">
            {id}
          </span>
        ) : null}
      </span>
    );
  }
  return (
    <span className="truncate font-mono" title={id}>
      {id}
    </span>
  );
}

function PayoutValue({
  payout,
  syncPath,
}: {
  payout: number;
  syncPath: string;
}): JSX.Element {
  if (syncPath === 'webhook' && payout === 0) {
    return (
      <span>
        <span className="text-calyx-muted">{WEBHOOK_MISSING}</span>
        <span className="mt-0.5 block text-[10px] text-calyx-muted">
          If the postback included payout, it would show here.
        </span>
      </span>
    );
  }
  return (
    <span>
      ${payout.toFixed(2)}
      {payout === 0 && syncPath === 'api_sync' ? (
        <span className="mt-0.5 block text-[10px] text-calyx-muted">
          No payout — criteria may not have been met
        </span>
      ) : null}
    </span>
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

  const ping = view.ping;
  const syncPath = ping?.sync_path ?? '';

  return (
    <div className="space-y-3">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
        Native Deep View
      </p>
      {ping && (
        <Section title="Ping / RTB">
          <dl className="grid grid-cols-2 gap-2 text-xs">
            <div className="col-span-2">
              <dt className="text-calyx-muted">Call time</dt>
              <dd className="font-mono text-sm text-calyx-text">
                {formatCallTimestamp(ping.occurred_at, ping.received_at)}
              </dd>
            </div>
            <div className="col-span-2">
              <dt className="text-calyx-muted">Call ID</dt>
              <dd className="break-all font-mono text-calyx-text" title={view.call_id}>
                {view.call_id || '—'}
              </dd>
            </div>
            <Field label="Platform">{ping.platform}</Field>
            <Field label="State">{ping.state || '—'}</Field>
            <Field label="Revenue">${ping.revenue.toFixed(2)}</Field>
            <Field label="Payout">
              <PayoutValue payout={ping.payout} syncPath={syncPath} />
            </Field>
            {((ping.profit_gross ?? 0) !== 0 || syncPath === 'api_sync') ? (
              <Field label="Profit (gross)">${(ping.profit_gross ?? 0).toFixed(2)}</Field>
            ) : null}
            <Field label="Connected length">
              {(ping.duration_seconds ?? 0) > 0 ? (
                formatSeconds(ping.duration_seconds ?? 0)
              ) : syncPath === 'webhook' ? (
                <span className="text-calyx-muted">{WEBHOOK_MISSING}</span>
              ) : (
                '—'
              )}
            </Field>
            <Field label="Total call length">
              {(ping.call_length_seconds ?? 0) > 0 ? (
                formatSeconds(ping.call_length_seconds ?? 0)
              ) : syncPath === 'webhook' ? (
                <span className="text-calyx-muted">{WEBHOOK_MISSING}</span>
              ) : (
                '—'
              )}
            </Field>
            <Field label="Converted">
              <span className={ping.conversion_state === 'converted' ? 'text-calyx-teal' : undefined}>
                {conversionStateLabel(ping.conversion_state)}
              </span>
            </Field>
            <Field label="Recording">
              {ping.recording_known ? (
                ping.has_recording ? 'Yes' : 'No'
              ) : syncPath === 'webhook' ? (
                <span className="text-calyx-muted">{WEBHOOK_MISSING}</span>
              ) : (
                '—'
              )}
            </Field>
            <Field label="Campaign">
              <NamedId
                name={ping.campaign_name ?? ''}
                id={ping.campaign_id}
                syncPath={syncPath}
              />
            </Field>
            <Field label="Target">
              <NamedId name={ping.target_name ?? ''} id={ping.target_id} syncPath={syncPath} />
            </Field>
            <Field label="Publisher">
              <NamedId
                name={ping.publisher_name ?? ''}
                id={ping.publisher_id}
                syncPath={syncPath}
              />
            </Field>
            <Field label="Buyer">
              <NamedId name={ping.buyer_name ?? ''} id={ping.buyer_id ?? ''} syncPath={syncPath} />
            </Field>
          </dl>
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
    </div>
  );
}
