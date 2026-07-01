import { cn } from '@/lib/utils';

interface PillStyle {
  bg: string;
  text: string;
}

const QUALITY_STYLES: Record<string, PillStyle> = {
  Good: { bg: '#E1F5EE', text: '#0F6E56' },
  Watch: { bg: '#FAEEDA', text: '#854F0B' },
  Risk: { bg: '#FCEBEB', text: '#A32D2D' },
};

const CHANNEL_STYLES: Record<string, PillStyle> = {
  Email: { bg: '#E6F1FB', text: '#185FA5' },
  Slack: { bg: '#EEEDFE', text: '#3C3489' },
  SMS: { bg: '#FAEEDA', text: '#854F0B' },
};

const CHANNEL_SLUG_MAP: Record<string, string> = {
  mock: 'Email',
  email: 'Email',
  slack: 'Slack',
  sms: 'SMS',
};

const EVENT_STYLES: Record<string, PillStyle> = {
  Scored: { bg: '#E1F5EE', text: '#0F6E56' },
  Ping: { bg: '#E6F1FB', text: '#185FA5' },
  Conversion: { bg: '#FBEAF0', text: '#993556' },
  Violation: { bg: '#FCEBEB', text: '#A32D2D' },
};

function StatusPillBase({
  label,
  style,
  className,
}: {
  label: string;
  style: PillStyle;
  className?: string;
}): JSX.Element {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium leading-none',
        className,
      )}
      style={{ backgroundColor: style.bg, color: style.text }}
    >
      {label}
    </span>
  );
}

export type PublisherQuality = 'Good' | 'Watch' | 'Risk';

export function QualityBadge({
  quality,
  className,
}: {
  quality: PublisherQuality;
  className?: string;
}): JSX.Element {
  const style = QUALITY_STYLES[quality] ?? QUALITY_STYLES.Good;
  return <StatusPillBase label={quality} style={style} className={className} />;
}

export function ChannelBadge({
  channel,
  className,
}: {
  channel: string;
  className?: string;
}): JSX.Element {
  const label = CHANNEL_SLUG_MAP[channel.toLowerCase()] ?? formatChannelLabel(channel);
  const style = CHANNEL_STYLES[label] ?? CHANNEL_STYLES.Email;
  return <StatusPillBase label={label} style={style} className={className} />;
}

function formatChannelLabel(channel: string): string {
  if (!channel) return 'Email';
  return channel.charAt(0).toUpperCase() + channel.slice(1);
}

export function ProviderStatusBadge({
  active,
  className,
}: {
  active: boolean;
  className?: string;
}): JSX.Element {
  const style = active
    ? { bg: '#E1F5EE', text: '#0F6E56' }
    : { bg: '#F1EFE8', text: '#5F5E5A' };
  return (
    <StatusPillBase label={active ? 'Active' : 'Mock'} style={style} className={className} />
  );
}

export function LiveEventBadge({
  eventType,
  className,
}: {
  eventType: string;
  className?: string;
}): JSX.Element {
  const label = liveEventLabel(eventType);
  const style = EVENT_STYLES[label] ?? EVENT_STYLES.Ping;
  return <StatusPillBase label={label} style={style} className={className} />;
}

function liveEventLabel(eventType: string): string {
  switch (eventType) {
    case 'call_scored':
      return 'Scored';
    case 'ping_received':
      return 'Ping';
    case 'conversion_attributed':
      return 'Conversion';
    case 'compliance_violation':
      return 'Violation';
    default:
      return eventType;
  }
}
