export type ConnectionMode = 'webhook' | 'api_sync';

/** Per-integration API sync adapter mode (mock demo vs real REST pull). */
export type SyncMode = 'mock' | 'real';

export const WEBHOOK_PLATFORMS = [
  'ringba',
  'retreaver',
  'trackdrive',
  'callgrid',
] as const;

export type WebhookPlatform = (typeof WEBHOOK_PLATFORMS)[number];

/** Platforms that support API-key sync (no callback setup). */
export const API_SYNC_PLATFORMS: readonly WebhookPlatform[] = [
  'ringba',
  'retreaver',
  'trackdrive',
  'callgrid',
];

export function supportsApiSync(platform: string): boolean {
  return (API_SYNC_PLATFORMS as readonly string[]).includes(platform);
}

export function supportsWebhook(platform: string): boolean {
  return (WEBHOOK_PLATFORMS as readonly string[]).includes(platform);
}
