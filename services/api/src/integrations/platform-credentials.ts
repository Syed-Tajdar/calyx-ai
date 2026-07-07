import type { WebhookPlatform } from './platform-connection';

export type ApiSyncCredentialField = {
  key: string;
  label: string;
  secret?: boolean;
  placeholder?: string;
};

const FIELDS: Record<WebhookPlatform, ApiSyncCredentialField[]> = {
  retreaver: [
    { key: 'api_key', label: 'API key', secret: true },
    { key: 'company_id', label: 'Company ID', placeholder: 'From Settings → My Company' },
  ],
  ringba: [
    { key: 'api_token', label: 'API access token', secret: true },
    { key: 'account_id', label: 'Account ID', placeholder: 'Ringba account UUID' },
  ],
  trackdrive: [
    { key: 'subdomain', label: 'Subdomain', placeholder: 'your-account' },
    { key: 'public_key', label: 'Public key', secret: true },
    { key: 'private_key', label: 'Private key', secret: true },
  ],
  callgrid: [{ key: 'api_key', label: 'API key', secret: true }],
};

export function apiSyncCredentialFields(platform: string): ApiSyncCredentialField[] {
  return FIELDS[platform as WebhookPlatform] ?? [{ key: 'api_key', label: 'API key', secret: true }];
}

export function buildCredentialBlob(
  platform: string,
  input: Record<string, string | undefined>,
): Record<string, string> {
  const fields = apiSyncCredentialFields(platform);
  const out: Record<string, string> = {};
  for (const field of fields) {
    const val = input[field.key]?.trim();
    if (!val) {
      throw new Error(`${field.label} is required for ${platform}`);
    }
    out[field.key] = val;
  }
  return out;
}

export function validateApiSyncCredentials(
  platform: string,
  creds: Record<string, string>,
): void {
  const fields = apiSyncCredentialFields(platform);
  for (const field of fields) {
    if (!creds[field.key]?.trim()) {
      throw new Error(`${field.label} is required`);
    }
  }
}
