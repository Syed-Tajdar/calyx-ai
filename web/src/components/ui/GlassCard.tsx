import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface GlassCardProps {
  children: ReactNode;
  className?: string;
  as?: 'div' | 'section' | 'article';
}

export function GlassCard({
  children,
  className,
  as: Tag = 'div',
}: GlassCardProps): JSX.Element {
  return <Tag className={cn('calyx-glass calyx-glass-lit', className)}>{children}</Tag>;
}
