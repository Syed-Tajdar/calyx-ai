/**
 * Stage 4b — whether, and how, the call-detail Conversion card is shown.
 *
 * The platform list is the backend's, not the web app's: `supports_writeback` comes from
 * GET /integrations/platform-options (and is repeated on each integration record). The web no
 * longer keeps its own `WRITEBACK_PLATFORMS`. Ringba's states and wording are unchanged.
 */

export interface WritebackPlatformOption {
  platform: string;
  supports_writeback?: boolean;
}

export interface WritebackIntegration {
  id: string;
  platform: string;
  connection_mode: string;
  sync_mode: string | null;
  writeback_enabled?: boolean;
  supports_writeback?: boolean;
}

export type WritebackView<I extends WritebackIntegration = WritebackIntegration> =
  | { kind: 'hidden' }
  | { kind: 'unavailable'; reason: string }
  | { kind: 'ready'; integration: I };

/** True when the backend reports a write-back path for this platform. */
export function platformSupportsWriteback(
  options: WritebackPlatformOption[] | null,
  platform: string,
): boolean {
  if (!options) return false;
  const p = (platform ?? '').toLowerCase();
  return options.some((o) => o.platform.toLowerCase() === p && o.supports_writeback === true);
}

/** The live API-key integration write-back goes through, or null. */
export function pickWritebackIntegration<I extends WritebackIntegration>(
  integrations: I[],
  platform: string,
): I | null {
  return (
    integrations.find(
      (i) => i.platform === platform && i.connection_mode === 'api_sync' && i.sync_mode === 'real',
    ) ?? null
  );
}

/**
 * Decide what the Conversion card shows. Hidden for non-admins and for platforms the backend
 * does not support; otherwise an explanation until a live, write-back-enabled integration exists.
 */
export function writebackView<I extends WritebackIntegration>(input: {
  /** canSeeWritebackActions(role) — admin only. */
  canSee: boolean;
  platform: string;
  platformLabel: string;
  options: WritebackPlatformOption[] | null;
  integrations: I[] | null;
}): WritebackView<I> {
  if (!input.canSee) return { kind: 'hidden' };
  if (!platformSupportsWriteback(input.options, input.platform)) return { kind: 'hidden' };
  if (input.integrations === null) return { kind: 'unavailable', reason: 'Checking connection…' };
  const integration = pickWritebackIntegration(input.integrations, input.platform);
  if (!integration) {
    return {
      kind: 'unavailable',
      reason: `No live ${input.platformLabel} API connection — add one in Settings and set it to Live.`,
    };
  }
  if (integration.supports_writeback === false) return { kind: 'hidden' };
  if (!integration.writeback_enabled) {
    return { kind: 'unavailable', reason: 'Write-back is turned off for this account.' };
  }
  return { kind: 'ready', integration };
}

export interface WritebackCopy {
  /** Extra line in the convert confirmation, or null. */
  convertWarning: string | null;
  cancelButton: string;
  cancelTitle: string;
  cancelConfirmLabel: string;
  cancelHeading: string;
  cancelBody: string;
  /** Short tag in the dialog header and the toast verb. */
  cancelTag: string;
  cancelVerb: string;
}

export const RETREAVER_PIXEL_WARNING =
  "Retreaver may fire this call's conversion pixels/postbacks to the buyer or publisher.";

/** Platform wording for the Convert / Cancel controls. Ringba's is the pre-4b wording. */
export function writebackCopy(platform: string, platformLabel: string): WritebackCopy {
  if ((platform ?? '').toLowerCase() === 'retreaver') {
    return {
      convertWarning: RETREAVER_PIXEL_WARNING,
      cancelButton: 'Void conversion (sets revenue to $0)',
      cancelTitle: 'Void this conversion',
      cancelConfirmLabel: 'Confirm void',
      cancelHeading: 'Void conversion (sets revenue to $0)?',
      cancelBody:
        'This sets the call’s revenue to $0 in Retreaver. Pixels or postbacks that already fired are not recalled. It is reversible — the call can be converted again afterwards.',
      cancelTag: 'Void',
      cancelVerb: 'Voided',
    };
  }
  return {
    convertWarning: null,
    cancelButton: 'Cancel conversion',
    cancelTitle: 'Cancel this conversion',
    cancelConfirmLabel: 'Confirm cancel',
    cancelHeading: 'Cancel this conversion?',
    cancelBody: `This voids the conversion and its payout in ${platformLabel}. It is reversible — the call can be converted again afterwards.`,
    cancelTag: 'Cancel',
    cancelVerb: 'Cancelled',
  };
}
