/**
 * Retreaver has no buyer field: Calyx maps the buyer from the target (owner decision
 * 2026-10-20), so the label says so for Retreaver calls.
 */
export function buyerLabel(platform: string | null | undefined): string {
  return (platform ?? '').toLowerCase() === 'retreaver' ? 'Buyer (target)' : 'Buyer';
}
