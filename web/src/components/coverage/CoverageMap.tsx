'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { geoCentroid } from 'd3-geo';
import { scaleLinear } from 'd3-scale';
import { ComposableMap, Geographies, Geography, useMapContext } from 'react-simple-maps';
import type { GeographyType } from 'react-simple-maps';
import type { HeatmapStateRow } from '@/lib/api';
import { codeFromFips, stateNameFromCode } from '@/lib/us-states';
import { usePrefersReducedMotion } from '@/lib/use-count-up';

const GEO_URL = '/geo/us-states-10m.json';

/** Neutral fill — lighter than page bg so the US outline reads clearly. */
const COLOR_NEUTRAL = '#1a2535';
const COLOR_DIM = '#013d3a';
const COLOR_DEEP = '#015a58';
const COLOR_MID = '#008f82';
const COLOR_BRIGHT = '#11ceb8';

const SCALE_STOPS = [COLOR_DIM, COLOR_DEEP, COLOR_MID, COLOR_BRIGHT] as const;

/** Pixel nudges for crowded or tiny state labels (projected SVG coords). */
const STATE_LABEL_OFFSETS: Record<string, [number, number]> = {
  CT: [0, 5],
  DC: [2, 3],
  DE: [3, 0],
  MA: [0, -2],
  MD: [5, 3],
  NH: [0, 5],
  NJ: [0, 0],
  RI: [2, 0],
  VT: [0, -3],
};

const STATE_LABEL_FONT_SIZE: Record<string, number> = {
  CT: 7,
  DC: 7,
  DE: 7,
  HI: 8,
  MA: 8,
  MD: 7,
  NH: 7,
  NJ: 8,
  RI: 6,
  VT: 7,
};

const DEFAULT_LABEL_FONT_SIZE = 10;

interface StateLabelsProps {
  geographies: GeographyType[];
  hoveredCode: string | null;
  visible: boolean;
  reducedMotion: boolean;
}

function StateLabels({
  geographies,
  hoveredCode,
  visible,
  reducedMotion,
}: StateLabelsProps): JSX.Element | null {
  const { projection } = useMapContext();
  if (!projection) return null;

  return (
    <>
      {geographies.map((geo) => {
        const code = codeFromFips(String(geo.id ?? ''));
        if (!code) return null;

        const projected = projection(geoCentroid(geo));
        if (!projected) return null;

        const [x, y] = projected;
        const [labelDx, labelDy] = STATE_LABEL_OFFSETS[code] ?? [0, 0];
        const labelSize = STATE_LABEL_FONT_SIZE[code] ?? DEFAULT_LABEL_FONT_SIZE;
        const dimmed = hoveredCode !== null && hoveredCode !== code && !reducedMotion;
        const labelOpacity = visible ? (dimmed ? 0.45 : 0.92) : 0;

        return (
          <text
            key={`label-${geo.rsmKey}`}
            x={x + labelDx}
            y={y + labelDy}
            textAnchor="middle"
            dominantBaseline="middle"
            className="calyx-coverage-map-label"
            style={{
              fontSize: labelSize,
              opacity: labelOpacity,
              transition: reducedMotion ? undefined : 'opacity 200ms ease',
            }}
            aria-hidden
          >
            {code}
          </text>
        );
      })}
    </>
  );
}

interface CoverageMapProps {
  data: HeatmapStateRow[];
  selectedState: string | null;
  onSelectState: (code: string | null) => void;
  totalPings: number;
}

interface TooltipState {
  code: string;
  name: string;
  pings: number;
  revenue: number;
  x: number;
  y: number;
}

function buildDataMap(rows: HeatmapStateRow[]): Map<string, { pings: number; revenue: number }> {
  const map = new Map<string, { pings: number; revenue: number }>();
  for (const row of rows) {
    const code = row.state.trim().toUpperCase();
    if (!code) continue;
    map.set(code, { pings: row.ping_count, revenue: row.revenue });
  }
  return map;
}

export function CoverageMap({
  data,
  selectedState,
  onSelectState,
  totalPings,
}: CoverageMapProps): JSX.Element {
  const reducedMotion = usePrefersReducedMotion();
  const [hoveredCode, setHoveredCode] = useState<string | null>(null);
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const [mapVisible, setMapVisible] = useState(reducedMotion);

  useEffect(() => {
    if (reducedMotion) return;
    const timer = setTimeout(() => setMapVisible(true), 60);
    return () => clearTimeout(timer);
  }, [reducedMotion]);

  const dataMap = useMemo(() => buildDataMap(data), [data]);

  const maxPings = useMemo(
    () => Math.max(...data.map((r) => r.ping_count), 1),
    [data],
  );

  const minPings = useMemo(() => {
    const positive = data.filter((r) => r.ping_count > 0).map((r) => r.ping_count);
    return positive.length > 0 ? Math.min(...positive) : 1;
  }, [data]);

  /** Graded scale for states WITH data — min→max maps dim teal→bright teal. */
  const dataColorScale = useMemo(
    () =>
      scaleLinear<string>()
        .domain([minPings, maxPings * 0.3, maxPings * 0.65, maxPings])
        .range([...SCALE_STOPS])
        .clamp(true),
    [minPings, maxPings],
  );

  const getDataFill = useCallback(
    (pings: number): string => {
      if (pings <= 0) return COLOR_NEUTRAL;
      return dataColorScale(pings);
    },
    [dataColorScale],
  );

  const handleMouseMove = useCallback(
    (code: string, pings: number, revenue: number, event: React.MouseEvent) => {
      setTooltip({
        code,
        name: stateNameFromCode(code),
        pings,
        revenue,
        x: event.clientX,
        y: event.clientY,
      });
    },
    [],
  );

  const brightenFill = useCallback((fill: string): string => {
    if (fill === COLOR_NEUTRAL) return '#243044';
    return COLOR_BRIGHT;
  }, []);

  return (
    <div className="calyx-coverage-map relative w-full">
      {/* Subtle blurred aurora — never a solid disc */}
      <div
        className="pointer-events-none absolute right-0 top-0 h-64 w-64 -translate-y-1/4 translate-x-1/4 rounded-full opacity-[0.08] blur-3xl"
        style={{ background: 'radial-gradient(circle, var(--aurora-orb-1) 0%, transparent 70%)' }}
        aria-hidden
      />

      <div className="relative min-h-[420px] w-full py-2 sm:py-4">
        <ComposableMap
          projection="geoAlbersUsa"
          className="calyx-coverage-map-svg relative h-auto w-full"
          style={{ minHeight: 400, maxHeight: 480 }}
          onMouseLeave={() => {
            setHoveredCode(null);
            setTooltip(null);
          }}
        >
          <Geographies geography={GEO_URL}>
            {({ geographies }) => (
              <>
                {geographies.map((geo, index) => {
                const fips = String(geo.id ?? '');
                const code = codeFromFips(fips);
                if (!code) return null;

                const entry = dataMap.get(code);
                const pings = entry?.pings ?? 0;
                const revenue = entry?.revenue ?? 0;
                const hasData = pings > 0;
                const isSelected = selectedState === code;
                const dimmed =
                  hoveredCode !== null && hoveredCode !== code && !reducedMotion;
                const visible = mapVisible || reducedMotion;
                const baseFill = getDataFill(pings);

                const geoPulse =
                  !reducedMotion && !isSelected && visible
                    ? `calyx-geo-stroke-pulse 2s cubic-bezier(0.4, 0, 0.6, 1) ${index * 30}ms infinite`
                    : undefined;
                const enterAnim =
                  !reducedMotion && !visible
                    ? `calyx-map-enter 400ms ease ${index * 8}ms forwards`
                    : undefined;

                return (
                  <Geography
                    key={geo.rsmKey}
                    geography={geo}
                    tabIndex={0}
                    aria-label={`${stateNameFromCode(code)}${hasData ? `, ${pings} pings` : ', no data'}`}
                    onMouseEnter={() => setHoveredCode(code)}
                    onMouseMove={(event) => {
                      handleMouseMove(code, pings, revenue, event);
                    }}
                    onMouseLeave={() => setTooltip(null)}
                    onClick={() => onSelectState(isSelected ? null : code)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        onSelectState(isSelected ? null : code);
                      }
                    }}
                    onFocus={() => setHoveredCode(code)}
                    onBlur={() => setHoveredCode(null)}
                    style={{
                      default: {
                        fill: baseFill,
                        stroke: isSelected ? 'var(--calyx-teal)' : undefined,
                        strokeWidth: isSelected ? 2.5 : hasData ? 0.6 : 0.75,
                        outline: 'none',
                        opacity: visible ? (dimmed ? 0.5 : 1) : 0,
                        transform: isSelected && !reducedMotion ? 'scale(1.03)' : 'scale(1)',
                        transformOrigin: 'center',
                        transition: reducedMotion
                          ? undefined
                          : 'opacity 200ms ease, transform 200ms ease, fill 200ms ease, stroke 200ms ease',
                        filter: isSelected
                          ? 'drop-shadow(0 0 10px rgba(17, 206, 184, 0.55))'
                          : undefined,
                        animation: enterAnim ?? geoPulse,
                      },
                      hover: {
                        fill: brightenFill(baseFill),
                        stroke: 'var(--calyx-teal)',
                        strokeWidth: isSelected ? 2.5 : 1.5,
                        outline: 'none',
                        opacity: 1,
                        transform: reducedMotion ? 'scale(1)' : 'scale(1.02)',
                        cursor: 'pointer',
                        filter: 'drop-shadow(0 0 8px rgba(17, 206, 184, 0.45))',
                      },
                      pressed: {
                        fill: COLOR_MID,
                        outline: 'none',
                      },
                    }}
                  />
                );
                })}
                <StateLabels
                  geographies={geographies}
                  hoveredCode={hoveredCode}
                  visible={mapVisible || reducedMotion}
                  reducedMotion={reducedMotion}
                />
              </>
            )}
          </Geographies>
        </ComposableMap>
      </div>

      <div className="mt-2 px-1 pb-2">
        <p className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-calyx-muted">
          Ping demand
        </p>
        <div className="flex items-center gap-2">
          <span className="w-6 shrink-0 text-right text-[10px] font-medium tabular-nums text-calyx-dim">
            0
          </span>
          <div className="relative h-2.5 flex-1 overflow-hidden rounded-full">
            <div
              className="h-full w-full"
              style={{
                background: `linear-gradient(to right, ${COLOR_NEUTRAL}, ${COLOR_DIM}, ${COLOR_DEEP}, ${COLOR_MID}, ${COLOR_BRIGHT})`,
              }}
            />
          </div>
          <span className="w-8 shrink-0 text-[10px] font-medium tabular-nums text-calyx-dim">
            {maxPings}
          </span>
        </div>
      </div>

      {tooltip && (
        <div
          className="calyx-coverage-tooltip pointer-events-none fixed z-50 min-w-[140px] rounded-lg border border-[var(--glass-border)] px-3 py-2.5 shadow-lg motion-safe:animate-[calyx-tooltip-in_120ms_ease-out]"
          style={{
            left: tooltip.x + 14,
            top: tooltip.y + 14,
            background: 'var(--glass-bg)',
            backdropFilter: 'blur(12px)',
          }}
          role="tooltip"
        >
          <p className="text-xs font-bold text-calyx-text">{tooltip.name}</p>
          <div className="my-1.5 h-px bg-calyx-teal/30" />
          <div className="space-y-1 text-[11px]">
            <p>
              <span className="text-calyx-muted">Pings </span>
              <span className="font-bold tabular-nums text-calyx-text">
                {tooltip.pings.toLocaleString()}
              </span>
            </p>
            <p>
              <span className="text-calyx-muted">Revenue </span>
              <span className="font-semibold tabular-nums text-calyx-teal">
                ${tooltip.revenue.toFixed(2)}
              </span>
            </p>
          </div>
          {totalPings > 0 && tooltip.pings > 0 && (
            <div className="mt-2">
              <div className="h-1 overflow-hidden rounded-full bg-calyx-surface">
                <div
                  className="h-full rounded-full bg-calyx-teal"
                  style={{ width: `${(tooltip.pings / totalPings) * 100}%` }}
                />
              </div>
              <p className="mt-1 text-[9px] text-calyx-dim">
                {((tooltip.pings / totalPings) * 100).toFixed(1)}% of total
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
