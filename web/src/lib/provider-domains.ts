import type { ProviderBinding } from '@/lib/api';

export interface ProviderDomainRow {
  domain: string;
  label: string;
  providerName: string;
  /** API domain keys that map to this row (e.g. enrichment → Transcription). */
  bindingDomains: string[];
}

export const PROVIDER_DOMAIN_ROWS: ProviderDomainRow[] = [
  {
    domain: 'attribution',
    label: 'Attribution',
    providerName: 'Meta CAPI',
    bindingDomains: ['attribution'],
  },
  {
    domain: 'transcription',
    label: 'Transcription',
    providerName: 'Deepgram',
    bindingDomains: ['transcription', 'enrichment'],
  },
  {
    domain: 'dnc',
    label: 'DNC / Compliance',
    providerName: 'DNC.com',
    bindingDomains: ['dnc'],
  },
  {
    domain: 'llm',
    label: 'Ask AI / LLM',
    providerName: 'OpenAI',
    bindingDomains: ['llm'],
  },
];

export function isMockModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_MOCK_MODE !== 'false';
}

export function findBindingForRow(
  row: ProviderDomainRow,
  bindings: ProviderBinding[],
): ProviderBinding | undefined {
  return bindings.find((b) => row.bindingDomains.includes(b.domain));
}

export function bindingSecretsLabel(binding: ProviderBinding | undefined): 'Configured' | 'Not set' {
  if (!binding) return 'Not set';
  if (binding.provider_id === 'mock') return 'Not set';
  const hasConfig = Object.keys(binding.config_json ?? {}).length > 0;
  if (binding.is_active || hasConfig) return 'Configured';
  return 'Not set';
}

export function bindingIsActive(binding: ProviderBinding | undefined): boolean {
  if (!binding) return false;
  if (isMockModeEnabled()) return false;
  return binding.provider_id !== 'mock' && binding.is_active;
}
