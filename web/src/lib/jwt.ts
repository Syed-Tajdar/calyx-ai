export interface JwtPayload {
  sub: string;
  tenant_id: string;
  email: string;
  role: string;
  iat?: number;
  exp?: number;
}

export function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const json = atob(base64);
    return JSON.parse(json) as JwtPayload;
  } catch {
    return null;
  }
}

export function truncateTenantId(tenantId: string): string {
  if (tenantId.length <= 12) return tenantId;
  return `${tenantId.slice(0, 8)}…`;
}
