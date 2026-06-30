/** Webhook + native platforms supported by Calyx ingestion. */
export const KNOWN_PLATFORMS = [
  'ringba',
  'retreaver',
  'trackdrive',
  'callgrid',
  'callloom-native',
] as const;

export type KnownPlatform = (typeof KNOWN_PLATFORMS)[number];

const PLATFORM_LABELS: Record<string, string> = {
  ringba: 'Ringba',
  retreaver: 'Retreaver',
  trackdrive: 'TrackDrive',
  callgrid: 'CallGrid',
  'callloom-native': 'Call Loom',
  mock: 'Mock',
};

export function formatPlatformLabel(platform: string): string {
  if (PLATFORM_LABELS[platform]) return PLATFORM_LABELS[platform];
  if (!platform) return platform;
  return platform.charAt(0).toUpperCase() + platform.slice(1);
}

export function mergePlatformOptions(fromApi: string[]): string[] {
  const seen = new Set<string>();
  const merged: string[] = [];
  for (const platform of [...KNOWN_PLATFORMS, ...fromApi]) {
    if (!platform || seen.has(platform)) continue;
    seen.add(platform);
    merged.push(platform);
  }
  return merged;
}
