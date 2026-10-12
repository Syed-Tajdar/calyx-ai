'use client';

import { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, Undo2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { Input } from '@/components/ui/input';
import {
  ApiError,
  convertCall,
  fetchIntegrations,
  fetchMe,
  fetchPlatformConnectionOptions,
  voidCall,
  type IntegrationRecord,
  type PlatformConnectionOption,
} from '@/lib/api';
import { isConfirmableConvertAmount } from '@/lib/writeback-amount';
import { canSeeWritebackActions } from '@/lib/writeback-role';
import {
  platformSupportsWriteback,
  writebackCopy,
  writebackView,
} from '@/lib/writeback-support';
import { getToken } from '@/lib/auth';
import { formatPlatformLabel } from '@/lib/platform-options';

interface ConversionActionsProps {
  callId: string;
  platform: string;
  hasConverted: boolean;
  /** Current call revenue — pre-fills the amount when it is above zero. */
  revenue: number;
  /** Called after a successful write so the parent can refetch the deep view. */
  onChanged: () => void;
}

type DialogMode = 'convert' | 'void' | null;

function parseAmount(raw: string): number | null {
  const n = Number(raw);
  if (!Number.isFinite(n)) return null;
  return n;
}

/**
 * Convert / Cancel controls for a single call.
 *
 * Deliberately conservative: the controls only appear when a live API-key integration for this
 * platform exists *and* write-back is enabled for the tenant. Anything else renders an explanation
 * instead of a button, so the operator never clicks into a 403.
 */
export function ConversionActions({
  callId,
  platform,
  hasConverted,
  revenue,
  onChanged,
}: ConversionActionsProps): JSX.Element | null {
  const [integrations, setIntegrations] = useState<IntegrationRecord[] | null>(null);
  const [role, setRole] = useState<string | null>(null);
  const [mode, setMode] = useState<DialogMode>(null);
  const [amountText, setAmountText] = useState('');
  const [busy, setBusy] = useState(false);

  const [options, setOptions] = useState<PlatformConnectionOption[] | null>(null);

  const allowed = canSeeWritebackActions(role);
  // Money actions only make sense for platforms with a write-back path. The backend says which
  // (Stage 4b registry: Ringba, Retreaver) — the web no longer keeps its own list.
  const supported = platformSupportsWriteback(options, platform);
  const label = formatPlatformLabel(platform);
  const copy = writebackCopy(platform, label);

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    let cancelled = false;
    fetchMe(token)
      .then((me) => {
        if (!cancelled) setRole(me.role);
      })
      .catch(() => {
        if (!cancelled) setRole(null);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!allowed) return;
    const token = getToken();
    if (!token) return;
    let cancelled = false;
    fetchPlatformConnectionOptions(token)
      .then((rows) => {
        if (!cancelled) setOptions(rows);
      })
      .catch(() => {
        if (!cancelled) setOptions([]);
      });
    return () => {
      cancelled = true;
    };
  }, [allowed]);

  useEffect(() => {
    if (!supported || !allowed) return;
    const token = getToken();
    if (!token) return;
    let cancelled = false;
    fetchIntegrations(token)
      .then((rows) => {
        if (!cancelled) setIntegrations(rows);
      })
      .catch(() => {
        if (!cancelled) setIntegrations([]);
      });
    return () => {
      cancelled = true;
    };
  }, [supported, allowed]);

  const view = useMemo(
    () =>
      writebackView<IntegrationRecord>({
        canSee: allowed,
        platform,
        platformLabel: label,
        options,
        integrations,
      }),
    [allowed, platform, label, options, integrations],
  );
  const integration = view.kind === 'ready' ? view.integration : null;

  const amount = parseAmount(amountText);
  const amountInvalid = mode === 'convert' && !isConfirmableConvertAmount(amount);

  if (view.kind === 'hidden') return null;

  function openConvert(): void {
    // Under the "should have converted" rule the list also contains revenue = 0 calls, and the API
    // rejects a non-positive amount — so only pre-fill when there is a real value to offer.
    setAmountText(revenue > 0 ? revenue.toFixed(2) : '');
    setMode('convert');
  }

  async function runAction(): Promise<void> {
    const token = getToken();
    if (!token || !integration) return;
    setBusy(true);
    try {
      const result =
        mode === 'convert'
          ? await convertCall(token, integration.id, callId, amount ?? 0)
          : await voidCall(token, integration.id, callId);

      const verb = mode === 'convert' ? 'Converted' : copy.cancelVerb;
      toast.success(
        `${verb} in ${formatPlatformLabel(platform)}`,
        {
          description:
            result.outcome === 'settled'
              ? `Confirmed by ${formatPlatformLabel(platform)} — now $${result.conversion_amount.toFixed(2)}`
              : `Sent. ${formatPlatformLabel(platform)} had not confirmed yet (${result.outcome}) — it may take a moment.`,
        },
      );
      setMode(null);
      onChanged();
    } catch (err: unknown) {
      const message =
        err instanceof ApiError && err.message
          ? err.message
          : 'The write failed. Nothing was changed.';
      toast.error(message);
    } finally {
      setBusy(false);
    }
  }

  // Explain, rather than showing a button that will fail.
  const unavailableReason = view.kind === 'unavailable' ? view.reason : '';

  return (
    <div className="calyx-card p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-calyx-muted">
            Conversion
          </p>
          <p className="mt-1 text-sm text-calyx-text">
            {hasConverted ? (
              <span className="text-calyx-teal">
                Converted in {formatPlatformLabel(platform)}
              </span>
            ) : (
              <span className="text-calyx-muted">
                Not converted in {formatPlatformLabel(platform)}
              </span>
            )}
          </p>
        </div>

        {unavailableReason ? (
          <p className="max-w-xs text-right text-xs text-calyx-muted">{unavailableReason}</p>
        ) : hasConverted ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-2"
            onClick={() => setMode('void')}
          >
            <Undo2 className="h-4 w-4" />
            {copy.cancelButton}
          </Button>
        ) : (
          <Button type="button" size="sm" className="gap-2" onClick={openConvert}>
            <CheckCircle2 className="h-4 w-4" />
            Convert
          </Button>
        )}
      </div>

      <ConfirmDialog
        open={mode === 'convert'}
        title="Convert this call"
        confirmLabel="Confirm convert"
        confirmDisabled={amountInvalid}
        busy={busy}
        onConfirm={() => {
          void runAction();
        }}
        onCancel={() => setMode(null)}
      >
        <p className="text-xs font-semibold uppercase tracking-wide text-calyx-muted">
          {formatPlatformLabel(platform)} · Convert
        </p>
        <p className="break-all font-mono text-xs text-calyx-muted">{callId}</p>
        {isConfirmableConvertAmount(amount) && (
          <p className="text-2xl font-semibold text-calyx-text">
            Convert this call for ${amount.toFixed(2)}?
          </p>
        )}
        {copy.convertWarning && (
          <p className="rounded-md border border-calyx-danger p-2 text-xs font-medium text-calyx-danger">
            {copy.convertWarning}
          </p>
        )}
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-calyx-text">Amount (USD)</span>
          <Input
            type="number"
            inputMode="decimal"
            step="0.01"
            min="0.01"
            value={amountText}
            autoFocus
            onChange={(e) => setAmountText(e.target.value)}
            placeholder={revenue > 0 ? revenue.toFixed(2) : 'e.g. 18.40'}
          />
        </label>
        {amountInvalid && amountText !== '' && (
          <p className="text-xs text-calyx-danger">Enter an amount greater than $0.00.</p>
        )}
        {amountText === '' && revenue <= 0 && (
          <p className="text-xs text-calyx-muted">
            This call has no revenue yet, so there is nothing to pre-fill — enter what the
            conversion is worth.
          </p>
        )}
      </ConfirmDialog>

      <ConfirmDialog
        open={mode === 'void'}
        title={copy.cancelTitle}
        confirmLabel={copy.cancelConfirmLabel}
        destructive
        busy={busy}
        onConfirm={() => {
          void runAction();
        }}
        onCancel={() => setMode(null)}
      >
        <p className="text-xs font-semibold uppercase tracking-wide text-calyx-muted">
          {formatPlatformLabel(platform)} · {copy.cancelTag}
        </p>
        <p className="break-all font-mono text-xs text-calyx-muted">{callId}</p>
        <p className="text-2xl font-semibold text-calyx-text">{copy.cancelHeading}</p>
        <p>{copy.cancelBody}</p>
      </ConfirmDialog>
    </div>
  );
}
