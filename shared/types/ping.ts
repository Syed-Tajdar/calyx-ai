/** Canonical ping shape shared across Go and TypeScript services. */
export interface Ping {
  tenantId: string;
  platform: string;
  receivedAt: string;
}
