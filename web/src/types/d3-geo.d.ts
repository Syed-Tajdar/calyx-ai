declare module 'd3-geo' {
  export function geoCentroid(object: {
    type?: string;
    geometry?: { type: string; coordinates: unknown };
  }): [number, number];
}
