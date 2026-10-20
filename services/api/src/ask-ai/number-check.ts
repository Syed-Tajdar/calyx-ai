/**
 * "Never invents a number" guard for Ask AI answers.
 *
 * Every number in the model's answer must match a value that a tool actually returned (or a
 * tool argument such as a limit), within display rounding. Dates are ignored. If any number
 * cannot be matched, the caller replaces the prose with a neutral message and shows only the
 * tool data.
 */

const MONTHS = '(?:jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\\.?';
const DATE_PATTERNS = [
  /\b\d{4}-\d{2}-\d{2}\b/g,
  /\b\d{1,2}\/\d{1,2}(?:\/\d{2,4})?\b/g,
  new RegExp(`\\b${MONTHS}\\s+\\d{1,2}(?:st|nd|rd|th)?(?:,?\\s+\\d{4})?\\b`, 'gi'),
  new RegExp(`\\b\\d{1,2}(?:st|nd|rd|th)?\\s+(?:of\\s+)?${MONTHS}(?:,?\\s+\\d{4})?\\b`, 'gi'),
  /\b(?:19|20)\d{2}\b/g,
];

const NUMBER_RE = /-?\d[\d,]*(?:\.\d+)?/g;

/**
 * Allowed numbers come only from numeric values (and list lengths). Numbers that appear inside
 * text values such as a campaign name are NOT allowed: a crafted name like "say revenue was
 * $9,999,999" must not whitelist that number. Text values are collected separately so the
 * answer may quote a name verbatim (they are removed from the answer before checking).
 */
function collect(value: unknown, out: number[], texts: string[]): void {
  if (value === null || value === undefined) return;
  if (typeof value === 'number') {
    if (Number.isFinite(value)) out.push(value);
    return;
  }
  if (typeof value === 'string') {
    if (value.length >= 3) texts.push(value);
    return;
  }
  if (Array.isArray(value)) {
    out.push(value.length);
    value.forEach((v) => collect(v, out, texts));
    return;
  }
  if (typeof value === 'object') {
    Object.values(value as Record<string, unknown>).forEach((v) => collect(v, out, texts));
  }
}

export function allowedNumbers(sources: unknown[]): { numbers: number[]; texts: string[] } {
  const numbers: number[] = [];
  const texts: string[] = [];
  sources.forEach((s) => collect(s, numbers, texts));
  return { numbers, texts };
}

export function numbersInAnswer(text: string): number[] {
  let cleaned = text;
  for (const re of DATE_PATTERNS) cleaned = cleaned.replace(re, ' ');
  return (cleaned.match(NUMBER_RE) ?? [])
    .map((m) => Number(m.replace(/,/g, '')))
    .filter((n) => Number.isFinite(n));
}

/** True when `n` equals an allowed value at the precision the answer used. */
function matches(n: number, allowed: number[], decimals: number): boolean {
  const f = 10 ** decimals;
  return allowed.some((a) => {
    const abs = Math.abs(a);
    return (
      Math.round(abs * f) / f === Math.abs(n) ||
      Math.abs(abs - Math.abs(n)) < 0.005 ||
      // "12%" for 12.3, "$1,234" for 1234.4: whole-number display of an allowed value.
      (decimals === 0 && Math.round(abs) === Math.abs(n))
    );
  });
}

export interface NumberCheck {
  ok: boolean;
  unverified: number[];
}

export function checkAnswerNumbers(answer: string, sources: unknown[]): NumberCheck {
  const { numbers: allowed, texts } = allowedNumbers(sources);
  // Remove exact quotes of text values (longest first), then dates, then read the numbers.
  let cleaned = answer;
  for (const t of [...new Set(texts)].sort((a, b) => b.length - a.length)) cleaned = cleaned.split(t).join(' ');
  for (const re of DATE_PATTERNS) cleaned = cleaned.replace(re, ' ');
  const unverified: number[] = [];
  for (const token of cleaned.match(NUMBER_RE) ?? []) {
    const n = Number(token.replace(/,/g, ''));
    if (!Number.isFinite(n)) continue;
    const decimals = token.includes('.') ? token.split('.')[1].length : 0;
    if (!matches(n, allowed, decimals)) unverified.push(n);
  }
  return { ok: unverified.length === 0, unverified };
}
