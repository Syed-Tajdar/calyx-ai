/**
 * North America region allowlist (US states/DC/territories + Canadian provinces).
 * Mirrors Go NormalizeState in ingestion + platform-sync.
 * "CA" = California (US). Canada is province codes only (ON, BC, …) — never bare CA.
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

const CA_NAMES: Record<string, string> = {
  ON: 'Ontario',
  QC: 'Quebec',
  BC: 'British Columbia',
  AB: 'Alberta',
  MB: 'Manitoba',
  SK: 'Saskatchewan',
  NS: 'Nova Scotia',
  NB: 'New Brunswick',
  NL: 'Newfoundland and Labrador',
  PE: 'Prince Edward Island',
  NT: 'Northwest Territories',
  YT: 'Yukon',
  NU: 'Nunavut',
};

export function countryForRegion(code: string): RegionCountry | null {
  const c = code.trim().toUpperCase();
  if (US_CODES.has(c)) return 'US';
  if (CA_CODES.has(c)) return 'CA';
  return null;
}

export function isKnownRegion(code: string): boolean {
  return countryForRegion(code) != null;
}

export function countryLabel(country: RegionCountry | null | undefined): string {
  if (country === 'CA') return 'Canada';
  if (country === 'US') return 'United States';
  return '';
}

export function regionDisplayName(
  code: string,
  usNameFromCode: (c: string) => string,
): string {
  const c = code.trim().toUpperCase();
  if (CA_NAMES[c]) return CA_NAMES[c];
  return usNameFromCode(c);
}

export function countByCountry(codes: string[]): { us: number; ca: number } {
  let us = 0;
  let ca = 0;
  for (const raw of codes) {
    const country = countryForRegion(raw);
    if (country === 'US') us += 1;
    else if (country === 'CA') ca += 1;
  }
  return { us, ca };
}
