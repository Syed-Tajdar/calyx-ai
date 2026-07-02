import { useId } from 'react';

interface MiniSparklineProps {
  data: number[];
  className?: string;
  width?: number;
  height?: number;
}

function buildSparklineGeometry(
  data: number[],
  width: number,
  height: number,
): { linePoints: string; areaPath: string } | null {
  if (!data.length) return null;

  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;
  const step = data.length > 1 ? width / (data.length - 1) : 0;
  const baseline = height - 2;

  const coords = data.map((value, index) => {
    const x = index * step;
    const y = height - ((value - min) / range) * (height - 4) - 2;
    return { x, y };
  });

  const linePoints = coords.map((p) => `${p.x},${p.y}`).join(' ');
  const linePath = coords.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');
  const areaPath = `${linePath} L ${coords[coords.length - 1].x} ${baseline} L ${coords[0].x} ${baseline} Z`;

  return { linePoints, areaPath };
}

export function MiniSparkline({
  data,
  className = '',
  width = 120,
  height = 32,
}: MiniSparklineProps): JSX.Element | null {
  const gradientId = useId().replace(/:/g, '');
  const geometry = buildSparklineGeometry(data, width, height);

  if (!geometry) return null;

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={className}
      aria-hidden
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--aurora-orb-1)" stopOpacity={0.28} />
          <stop offset="100%" stopColor="var(--aurora-orb-1)" stopOpacity={0} />
        </linearGradient>
      </defs>
      <path d={geometry.areaPath} fill={`url(#${gradientId})`} />
      <polyline
        fill="none"
        stroke="var(--calyx-teal)"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        points={geometry.linePoints}
      />
    </svg>
  );
}
