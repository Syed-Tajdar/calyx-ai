import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const badgeVariants = cva(
  'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-semibold transition-colors',
  {
    variants: {
      variant: {
        default: 'border-calyx-accent/30 bg-calyx-accent-muted text-calyx-accent',
        secondary: 'border-calyx-border bg-calyx-surface text-calyx-muted',
        success: 'border-calyx-success/30 bg-calyx-success/10 text-calyx-success',
        warning: 'border-calyx-warning/30 bg-calyx-warning/10 text-calyx-warning',
        danger: 'border-calyx-danger/30 bg-calyx-danger/10 text-calyx-danger',
        teal: 'border-calyx-teal/30 bg-calyx-teal/10 text-calyx-teal',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps): JSX.Element {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };
