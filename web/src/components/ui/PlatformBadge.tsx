import { formatPlatformColor, formatPlatformLabel } from '@/lib/platform-options';
import { cn } from '@/lib/utils';

interface PlatformBadgeProps {
  platform: string;
  className?: string;
}

export function PlatformBadge({ platform, className }: PlatformBadgeProps): JSX.Element {
  const colors = formatPlatformColor(platform);
  const label = formatPlatformLabel(platform);

  return (
    <span
      data-platform={platform}
      className={cn(
        'inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium leading-none',
        className,
      )}
      style={{ backgroundColor: colors.badgeBg, color: colors.badgeText }}
    >
      <span
        className="h-2 w-2 shrink-0 rounded-full"
        style={{ backgroundColor: colors.dot }}
        aria-hidden
      />
      {label}
    </span>
  );
}
