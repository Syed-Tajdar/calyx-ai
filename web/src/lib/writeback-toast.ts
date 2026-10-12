/**
 * Toast text after a successful write-back request (Ringba and Retreaver).
 *
 * platform-sync reports `outcome: "confirmed"` when the read-back matched within the settle
 * window, and `"pending"` (HTTP 202) when the write was accepted but not confirmed yet. The UI
 * used to test for `"settled"`, a value the backend never sends, so every confirmed write
 * showed "had not confirmed yet". `"settled"` is still accepted as a synonym.
 */
export interface WritebackToast {
  title: string;
  description: string;
  confirmed: boolean;
}

export function isConfirmedOutcome(outcome: string | null | undefined): boolean {
  return outcome === 'confirmed' || outcome === 'settled';
}

export function writebackToast(input: {
  /** "Converted", "Cancelled" (Ringba) or "Voided" (Retreaver). */
  verb: string;
  platformLabel: string;
  outcome: string | null | undefined;
  conversionAmount: number | null | undefined;
}): WritebackToast {
  const { verb, platformLabel, outcome } = input;
  const title = `${verb} in ${platformLabel}`;
  if (isConfirmedOutcome(outcome)) {
    const amount = Number.isFinite(input.conversionAmount) ? Number(input.conversionAmount) : 0;
    return {
      title,
      description: `Confirmed by ${platformLabel} — now $${amount.toFixed(2)}`,
      confirmed: true,
    };
  }
  return {
    title,
    description: `Sent. ${platformLabel} had not confirmed yet (${outcome ?? 'unknown'}) — it may take a moment.`,
    confirmed: false,
  };
}
