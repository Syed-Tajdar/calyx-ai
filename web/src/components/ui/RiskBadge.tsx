import { Badge } from '@/components/ui/badge';

export type RiskLevel = 'low' | 'medium' | 'high';

export function riskLevelFromScore(score: number): RiskLevel {
  if (score >= 70) return 'high';
  if (score >= 40) return 'medium';
  return 'low';
}

const VARIANT_MAP = {
  low: 'success' as const,
  medium: 'warning' as const,
  high: 'danger' as const,
};

const LABEL_MAP = {
  low: 'Low risk',
  medium: 'Medium risk',
  high: 'High risk',
};

interface RiskBadgeProps {
  score: number;
  className?: string;
}

export function RiskBadge({ score, className }: RiskBadgeProps): JSX.Element {
  const level = riskLevelFromScore(score);
  return (
    <Badge variant={VARIANT_MAP[level]} className={className}>
      {LABEL_MAP[level]} · {score}
    </Badge>
  );
}
