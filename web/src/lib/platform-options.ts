/** Webhook + native platforms supported by Calyx ingestion. */
export const KNOWN_PLATFORMS = [
  'ringba',
  'retreaver',
  'trackdrive',
  'callgrid',
  'callloom-native',
] as const;

export type KnownPlatform = (typeof KNOWN_PLATFORMS)[number];

export interface PlatformColorSet {
  dot: string;
  badgeBg: string;
  badgeText: string;
}

const PLATFORM_LABELS: Record<string, string> = {
  ringba: 'Ringba',
  retreaver: 'Retreaver',
  trackdrive: 'TrackDrive',
  callgrid: 'CallGrid',
  'callloom-native': 'Call Loom',
};

const PLATFORM_COLORS: Record<string, PlatformColorSet> = {
  ringba: { dot: '#378ADD', badgeBg: '#E6F1FB', badgeText: '#185FA5' },
  retreaver: { dot: '#D85A30', badgeBg: '#FAECE7', badgeText: '#993C1D' },
  trackdrive: { dot: '#BA7517', badgeBg: '#FAEEDA', badgeText: '#854F0B' },
  callgrid: { dot: '#639922', badgeBg: '#EAF3DE', badgeText: '#3B6D11' },
  'callloom-native': { dot: '#1D9E75', badgeBg: '#E1F5EE', badgeText: '#0F6E56' },
};

const DEFAULT_PLATFORM_COLOR: PlatformColorSet = {
  dot: '#5F5E5A',
  badgeBg: '#F1EFE8',
  badgeText: '#5F5E5A',
};

const HIDDEN_PLATFORMS = new Set(['mock']);

export function formatPlatformLabel(platform: string): string {
  if (PLATFORM_LABELS[platform]) return PLATFORM_LABELS[platform];
  if (!platform) return platform;
  return platform.charAt(0).toUpperCase() + platform.slice(1);
}

export function formatPlatformColor(platform: string): PlatformColorSet {
  return PLATFORM_COLORS[platform] ?? DEFAULT_PLATFORM_COLOR;
}

export function mergePlatformOptions(fromApi: string[]): string[] {
  const seen = new Set<string>();
  const merged: string[] = [];
  for (const platform of [...KNOWN_PLATFORMS, ...fromApi]) {
    if (!platform || seen.has(platform) || HIDDEN_PLATFORMS.has(platform)) continue;
    seen.add(platform);
    merged.push(platform);
  }
  return merged;
}
