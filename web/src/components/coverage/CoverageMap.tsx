'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { geoCentroid } from 'd3-geo';
import { scaleLinear } from 'd3-scale';
import { ComposableMap, Geographies, Geography, useMapContext } from 'react-simple-maps';
import type { GeographyType } from 'react-simple-maps';
import type { HeatmapStateRow } from '@/lib/api';
import {
  countryForRegion,
  countryLabel,
  regionDisplayName,
  type RegionCountry,
} from '@/lib/na-regions';
import { codeFromFips, stateNameFromCode } from '@/lib/us-states';
import { usePrefersReducedMotion } from '@/lib/use-count-up';
import { cn } from '@/lib/utils';

const US_GEO_URL = '/geo/us-states-10m.json';
const CA_GEO_URL = '/geo/canada-provinces.geojson';

/** Neutral fill — lighter than page bg so outlines read clearly. */
const COLOR_NEUTRAL = '#1a2535';
const COLOR_DIM = '#013d3a';
const COLOR_DEEP = '#015a58';
const COLOR_MID = '#008f82';
const COLOR_BRIGHT = '#11ceb8';

const SCALE_STOPS = [COLOR_DIM, COLOR_DEEP, COLOR_MID, COLOR_BRIGHT] as const;

/** Albers tuned for continental US + Canada (geoAlbersUsa clips Canada). */
const NA_PROJECTION_CONFIG: {
  rotate: [number, number, number];
  center: [number, number];
  parallels: [number, number];
  scale: number;
} = {
  // d3 geoAlbers US convention: rotate[+96] — negative values push NA off-frame.
  rotate: [96, 0, 0],
  center: [-0.6, 52],
  parallels: [29.5, 60],
  scale: 520,
};

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
  PE: [4, 0],
  NS: [4, 2],
  NB: [2, 0],
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
  PE: 7,
  NS: 7,
  NB: 7,
  NL: 8,
};

const DEFAULT_LABEL_FONT_SIZE = 10;

/**
 * Tiny offshore US regions are nearly invisible at true Albers scale.
 * Park real geo shapes in quiet bottom margins (zoomed) — All + US filters only.
 * Anchors are SVG coords for the default ~800×600 ComposableMap frame.
 */
const TERRITORY_INSETS: Record<
  string,
  {
    scale: number;
    anchor: [number, number];
    labelDx?: number;
    labelDy?: number;
    labelSize?: number;
  }
> = {
  HI: { scale: 3, anchor: [155, 470], labelDx: 0, labelDy: 14, labelSize: 11 },
  // Keep PR and VI clearly separated in the SE inset lane.
  PR: { scale: 4, anchor: [640, 448], labelDx: 0, labelDy: 16, labelSize: 11 },
  VI: { scale: 8, anchor: [780, 482], labelDx: 12, labelDy: 4, labelSize: 10 },
};

function buildInsetTransform(
  projection: (coords: [number, number]) => [number, number] | null,
  geo: GeographyType,
  inset: { scale: number; anchor: [number, number] },
): string | null {
  const projected = projection(geoCentroid(geo) as [number, number]);
  if (!projected) return null;
  const [cx, cy] = projected;
  const [ax, ay] = inset.anchor;
  return `translate(${ax} ${ay}) scale(${inset.scale}) translate(${-cx} ${-cy})`;
}

export type CountryFilter = 'all' | 'US' | 'CA';

function regionCodeFromGeo(geo: GeographyType, layer: 'US' | 'CA'): string | null {
  if (layer === 'CA') {
    const props = geo.properties as { code?: string } | null;
    const code = props?.code?.trim().toUpperCase();
    return code && countryForRegion(code) === 'CA' ? code : null;
  }
  return codeFromFips(String(geo.id ?? ''));
}

function displayName(code: string): string {
  return regionDisplayName(code, stateNameFromCode);
}

interface RegionLabelsProps {
  geographies: GeographyType[];
  layer: 'US' | 'CA';
  hoveredCode: string | null;
  visible: boolean;
  reducedMotion: boolean;
  countryFilter: CountryFilter;
}

function RegionLabels({
  geographies,
  layer,
  hoveredCode,
  visible,
  reducedMotion,
  countryFilter,
}: RegionLabelsProps): JSX.Element | null {
  const { projection } = useMapContext();
  if (!projection) return null;
  if (countryFilter !== 'all' && countryFilter !== layer) return null;

  return (
    <>
      {geographies.map((geo) => {
        const code = regionCodeFromGeo(geo, layer);
        if (!code) return null;

        const inset = layer === 'US' ? TERRITORY_INSETS[code] : undefined;
        let x: number;
        let y: number;
        let labelSize: number;

        if (inset) {
          x = inset.anchor[0] + (inset.labelDx ?? 0);
          y = inset.anchor[1] + (inset.labelDy ?? 0);
          labelSize = inset.labelSize ?? STATE_LABEL_FONT_SIZE[code] ?? DEFAULT_LABEL_FONT_SIZE;
        } else {
          const projected = projection(geoCentroid(geo) as [number, number]);
          if (!projected) return null;
          const [labelDx, labelDy] = STATE_LABEL_OFFSETS[code] ?? [0, 0];
          x = projected[0] + labelDx;
          y = projected[1] + labelDy;
          labelSize = STATE_LABEL_FONT_SIZE[code] ?? DEFAULT_LABEL_FONT_SIZE;
        }

        const dimmed = hoveredCode !== null && hoveredCode !== code && !reducedMotion;
        const labelOpacity = visible ? (dimmed ? 0.45 : 0.92) : 0;

        return (
          <text
            key={`label-${geo.rsmKey}`}
            x={x}
            y={y}
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
  countryFilter?: CountryFilter;
  onCountryFilterChange?: (filter: CountryFilter) => void;
}

interface TooltipState {
  code: string;
  name: string;
  country: RegionCountry;
  pings: number;
  revenue: number;
  x: number;
  y: number;
}

function buildDataMap(rows: HeatmapStateRow[]): Map<string, { pings: number; revenue: number }> {
  const map = new Map<string, { pings: number; revenue: number }>();
  for (const row of rows) {
    const code = row.state.trim().toUpperCase();
    if (!code || !countryForRegion(code)) continue;
    map.set(code, { pings: row.ping_count, revenue: row.revenue });
  }
  return map;
}

interface MapRegionLayerProps {
  geographies: GeographyType[];
  layer: 'US' | 'CA';
  countryFilter: CountryFilter;
  dataMap: Map<string, { pings: number; revenue: number }>;
  getDataFill: (pings: number) => string;
  brightenFill: (fill: string) => string;
  selectedState: string | null;
  hoveredCode: string | null;
  mapVisible: boolean;
  reducedMotion: boolean;
  onSelectState: (code: string | null) => void;
  setHoveredCode: (code: string | null) => void;
  onMouseMove: (code: string, pings: number, revenue: number, event: React.MouseEvent) => void;
  setTooltipNull: () => void;
}

function MapRegionLayer({
  geographies,
  layer,
  countryFilter,
  dataMap,
  getDataFill,
  brightenFill,
  selectedState,
  hoveredCode,
  mapVisible,
  reducedMotion,
  onSelectState,
  setHoveredCode,
  onMouseMove,
  setTooltipNull,
}: MapRegionLayerProps): JSX.Element | null {
  const { projection } = useMapContext();
  if (countryFilter !== 'all' && countryFilter !== layer) return null;
  if (!projection) return null;

  return (
    <>
      {geographies.map((geo, index) => {
        const code = regionCodeFromGeo(geo, layer);
        if (!code) return null;

        const entry = dataMap.get(code);
        const pings = entry?.pings ?? 0;
        const revenue = entry?.revenue ?? 0;
        const hasData = pings > 0;
        const isSelected = selectedState === code;
        const dimmed = hoveredCode !== null && hoveredCode !== code && !reducedMotion;
        const visible = mapVisible || reducedMotion;
        const baseFill = getDataFill(pings);
        const inset = layer === 'US' ? TERRITORY_INSETS[code] : undefined;
        const insetTransform =
          inset != null ? buildInsetTransform(projection, geo, inset) : null;

        const geoPulse =
          !reducedMotion && !isSelected && visible
            ? `calyx-geo-stroke-pulse 2s cubic-bezier(0.4, 0, 0.6, 1) ${index * 30}ms infinite`
            : undefined;
        const enterAnim =
          !reducedMotion && !visible
            ? `calyx-map-enter 400ms ease ${index * 8}ms forwards`
            : undefined;

        // Inset paths already scale via <g>; keep hover scale gentle so they stay tidy.
        const hoverScale = inset ? (reducedMotion ? 'scale(1)' : 'scale(1.015)') : reducedMotion ? 'scale(1)' : 'scale(1.02)';
        const selectedScale =
          isSelected && !reducedMotion ? (inset ? 'scale(1.02)' : 'scale(1.03)') : 'scale(1)';

        const geography = (
          <Geography
            geography={geo}
            tabIndex={0}
            aria-label={`${displayName(code)}, ${countryLabel(layer)}${hasData ? `, ${pings} pings` : ', no data'}`}
            onMouseEnter={() => setHoveredCode(code)}
            onMouseMove={(event) => {
              onMouseMove(code, pings, revenue, event);
            }}
            onMouseLeave={setTooltipNull}
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
                stroke: isSelected ? 'var(--calyx-teal)' : 'rgba(17, 206, 184, 0.4)',
                // Transform scales strokes — keep inset strokes visually thin.
                strokeWidth: inset
                  ? isSelected
                    ? 2.0 / inset.scale
                    : 0.5 / inset.scale
                  : isSelected
                    ? 2.5
                    : hasData
                      ? 0.65
                      : 0.75,
                outline: 'none',
                opacity: visible ? (dimmed ? 0.5 : 1) : 0,
                transform: selectedScale,
                transformOrigin: 'center',
                vectorEffect: 'non-scaling-stroke',
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
                strokeWidth: inset ? 1.4 / inset.scale : isSelected ? 2.5 : 1.5,
                outline: 'none',
                opacity: 1,
                transform: hoverScale,
                cursor: 'pointer',
                vectorEffect: 'non-scaling-stroke',
                filter: 'drop-shadow(0 0 8px rgba(17, 206, 184, 0.45))',
              },
              pressed: {
                fill: COLOR_MID,
                outline: 'none',
              },
            }}
          />
        );

        // Zoomed real geo shapes only — no rounded plates / circles.
        if (insetTransform) {
          return (
            <g key={geo.rsmKey} className="calyx-coverage-inset" transform={insetTransform}>
              {geography}
            </g>
          );
        }

        return <g key={geo.rsmKey}>{geography}</g>;
      })}
    </>
  );
}

export function CoverageMap({
  data,
  selectedState,
  onSelectState,
  totalPings,
  countryFilter = 'all',
  onCountryFilterChange,
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

  const filteredData = useMemo(() => {
    if (countryFilter === 'all') return data;
    return data.filter((r) => (r.country || countryForRegion(r.state)) === countryFilter);
  }, [data, countryFilter]);

  const dataMap = useMemo(() => buildDataMap(filteredData), [filteredData]);

  const maxPings = useMemo(
    () => Math.max(...filteredData.map((r) => r.ping_count), 1),
    [filteredData],
  );

  const minPings = useMemo(() => {
    const positive = filteredData.filter((r) => r.ping_count > 0).map((r) => r.ping_count);
    return positive.length > 0 ? Math.min(...positive) : 1;
  }, [filteredData]);

  const filteredTotalPings = useMemo(
    () => filteredData.reduce((sum, r) => sum + r.ping_count, 0),
    [filteredData],
  );

  /** Shared US+CA graded scale — min→max maps dim teal→bright teal. */
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
      const country = countryForRegion(code);
      if (!country) return;
      setTooltip({
        code,
        name: displayName(code),
        country,
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
      <div
        className="pointer-events-none absolute right-0 top-0 h-64 w-64 -translate-y-1/4 translate-x-1/4 rounded-full opacity-[0.08] blur-3xl"
        style={{ background: 'radial-gradient(circle, var(--aurora-orb-1) 0%, transparent 70%)' }}
        aria-hidden
      />

      {onCountryFilterChange && (
        <div className="mb-2 flex flex-wrap gap-1.5 px-1" role="group" aria-label="Country filter">
          {([
            ['all', 'All'],
            ['US', 'US'],
            ['CA', 'Canada'],
          ] as const).map(([value, label]) => (
            <button
              key={value}
              type="button"
              onClick={() => onCountryFilterChange(value)}
              className={cn(
                'rounded-md px-2.5 py-1 text-[11px] font-medium motion-safe:transition-colors',
                countryFilter === value
                  ? 'bg-calyx-teal/20 text-calyx-teal'
                  : 'text-calyx-muted hover:text-calyx-text',
              )}
              aria-pressed={countryFilter === value}
            >
              {label}
            </button>
          ))}
        </div>
      )}

      <div className="relative min-h-[420px] w-full py-2 sm:py-4">
        <ComposableMap
          projection="geoAlbers"
          projectionConfig={NA_PROJECTION_CONFIG}
          className="calyx-coverage-map-svg relative h-auto w-full"
          style={{ minHeight: 400, maxHeight: 520 }}
          onMouseLeave={() => {
            setHoveredCode(null);
            setTooltip(null);
          }}
        >
          <Geographies geography={US_GEO_URL}>
            {({ geographies }) => (
              <>
                <MapRegionLayer
                  geographies={geographies}
                  layer="US"
                  countryFilter={countryFilter}
                  dataMap={dataMap}
                  getDataFill={getDataFill}
                  brightenFill={brightenFill}
                  selectedState={selectedState}
                  hoveredCode={hoveredCode}
                  mapVisible={mapVisible}
                  reducedMotion={reducedMotion}
                  onSelectState={onSelectState}
                  setHoveredCode={setHoveredCode}
                  onMouseMove={handleMouseMove}
                  setTooltipNull={() => setTooltip(null)}
                />
                <RegionLabels
                  geographies={geographies}
                  layer="US"
                  hoveredCode={hoveredCode}
                  visible={mapVisible || reducedMotion}
                  reducedMotion={reducedMotion}
                  countryFilter={countryFilter}
                />
              </>
            )}
          </Geographies>
          <Geographies geography={CA_GEO_URL}>
            {({ geographies }) => (
              <>
                <MapRegionLayer
                  geographies={geographies}
                  layer="CA"
                  countryFilter={countryFilter}
                  dataMap={dataMap}
                  getDataFill={getDataFill}
                  brightenFill={brightenFill}
                  selectedState={selectedState}
                  hoveredCode={hoveredCode}
                  mapVisible={mapVisible}
                  reducedMotion={reducedMotion}
                  onSelectState={onSelectState}
                  setHoveredCode={setHoveredCode}
                  onMouseMove={handleMouseMove}
                  setTooltipNull={() => setTooltip(null)}
                />
                <RegionLabels
                  geographies={geographies}
                  layer="CA"
                  hoveredCode={hoveredCode}
                  visible={mapVisible || reducedMotion}
                  reducedMotion={reducedMotion}
                  countryFilter={countryFilter}
                />
              </>
            )}
          </Geographies>
        </ComposableMap>
      </div>

      <div className="mt-2 px-1 pb-2">
        <p className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-calyx-muted">
          Ping demand (US + Canada)
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
          <p className="text-[10px] text-calyx-muted">
            {tooltip.code} · {countryLabel(tooltip.country)}
          </p>
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
          {filteredTotalPings > 0 && tooltip.pings > 0 && (
            <div className="mt-2">
              <div className="h-1 overflow-hidden rounded-full bg-calyx-surface">
                <div
                  className="h-full rounded-full bg-calyx-teal"
                  style={{ width: `${(tooltip.pings / filteredTotalPings) * 100}%` }}
                />
              </div>
              <p className="mt-1 text-[9px] text-calyx-dim">
                {((tooltip.pings / filteredTotalPings) * 100).toFixed(1)}% of total
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
