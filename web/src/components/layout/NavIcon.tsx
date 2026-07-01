import type { LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

interface NavIconProps {
  icon: LucideIcon;
  className?: string;
}

export function NavIcon({ icon: Icon, className }: NavIconProps): JSX.Element {
  return (
    <Icon
      className={cn('size-4 shrink-0', className)}
      strokeWidth={2}
      aria-hidden
    />
  );
}
