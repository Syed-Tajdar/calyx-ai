declare module 'react-simple-maps' {
  import type { ComponentType, CSSProperties, KeyboardEvent, MouseEvent, ReactNode } from 'react';

  export interface GeographyType {
    rsmKey: string;
    id?: string | number;
    properties?: Record<string, unknown>;
    type?: string;
    geometry?: {
      type: string;
      coordinates: unknown;
    };
  }

  export interface ComposableMapProps {
    projection?: string;
    className?: string;
    style?: CSSProperties;
    children?: ReactNode;
    onMouseLeave?: () => void;
  }

  export interface GeographiesProps {
    geography: string | object;
    children: (args: { geographies: GeographyType[] }) => ReactNode;
  }

  export interface GeographyProps {
    geography: GeographyType;
    key?: string;
    tabIndex?: number;
    'aria-label'?: string;
    className?: string;
    onMouseEnter?: () => void;
    onMouseMove?: (event: MouseEvent<SVGPathElement>) => void;
    onMouseLeave?: () => void;
    onClick?: () => void;
    onKeyDown?: (event: KeyboardEvent<SVGPathElement>) => void;
    onFocus?: () => void;
    onBlur?: () => void;
    style?: {
      default?: CSSProperties;
      hover?: CSSProperties;
      pressed?: CSSProperties;
    };
  }

  export const ComposableMap: ComponentType<ComposableMapProps>;
  export const Geographies: ComponentType<GeographiesProps>;
  export const Geography: ComponentType<GeographyProps>;

  export interface MapContextValue {
    projection: ((coords: [number, number]) => [number, number] | null) | null;
  }

  export function useMapContext(): MapContextValue;

  export interface MarkerProps {
    coordinates: [number, number];
    children?: ReactNode;
  }

  export const Marker: ComponentType<MarkerProps>;
}
