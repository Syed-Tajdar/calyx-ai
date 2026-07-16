/**
 * North America region allowlist — mirrors Go NormalizeState (ingestion + platform-sync).
 * "CA" = California (US). Canada uses province codes only.
 */

export type RegionCountry = 'US' | 'CA';

const US_CODES = new Set([
  'AL', 'AK', 'AZ', 'AR', 'CA', 'CO', 'CT', 'DE', 'FL', 'GA', 'HI', 'ID', 'IL', 'IN',
  'IA', 'KS', 'KY', 'LA', 'ME', 'MD', 'MA', 'MI', 'MN', 'MS', 'MO', 'MT', 'NE', 'NV',
  'NH', 'NJ', 'NM', 'NY', 'NC', 'ND', 'OH', 'OK', 'OR', 'PA', 'RI', 'SC', 'SD', 'TN',
  'TX', 'UT', 'VT', 'VA', 'WA', 'WV', 'WI', 'WY', 'DC', 'PR', 'GU', 'VI', 'AS', 'MP',
]);

const CA_CODES = new Set([
  'ON', 'QC', 'BC', 'AB', 'MB', 'SK', 'NS', 'NB', 'NL', 'PE', 'NT', 'YT', 'NU',
]);

export function countryForRegion(code: string): RegionCountry | null {
  const c = code.trim().toUpperCase();
  if (US_CODES.has(c)) return 'US';
  if (CA_CODES.has(c)) return 'CA';
  return null;
}

export function isKnownRegion(code: string): boolean {
  return countryForRegion(code) != null;
}
