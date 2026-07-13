/** Capitalize first letter of each word; preserve short acronyms (CPA, DNC, LLM). */
export function formatTitleCase(text: string): string {
  if (!text) return text;
  return text
    .split(/\s+/)
    .map((word) => {
      if (!word) return word;
      if (word.length <= 4 && /^[A-Z0-9]+$/.test(word)) {
        return word;
      }
      return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
    })
    .join(' ');
}

const PROVIDER_DOMAIN_LABELS: Record<string, string> = {
  attribution: 'Attribution',
  dnc: 'DNC',
  llm: 'LLM',
  enrichment: 'Enrichment',
  transcription: 'Transcription',
};

export function formatProviderDomainLabel(domain: string): string {
  return PROVIDER_DOMAIN_LABELS[domain] ?? formatTitleCase(domain.replace(/_/g, ' '));
}
