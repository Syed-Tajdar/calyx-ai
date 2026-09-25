const CSRF_COOKIE = 'calyx_csrf';
/** Sentinel used by legacy call sites — JWT lives only in an httpOnly cookie. */
const COOKIE_SESSION = 'cookie-session';

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null;
  const parts = document.cookie.split(';');
  for (const part of parts) {
    const idx = part.indexOf('=');
    if (idx < 0) continue;
    const k = part.slice(0, idx).trim();
    if (k === name) {
      return decodeURIComponent(part.slice(idx + 1).trim());
    }
  }
  return null;
}

/** True when the non-httpOnly CSRF cookie is present (session established). */
export function hasSession(): boolean {
  return Boolean(readCookie(CSRF_COOKIE));
}

export function getCsrfToken(): string | null {
  return readCookie(CSRF_COOKIE);
}

/**
 * Compatibility shim: returns a sentinel when a cookie session exists.
 * The real JWT is never readable from JS (httpOnly).
 */
export function getToken(): string | null {
  return hasSession() ? COOKIE_SESSION : null;
}

/** No-op — session is set by the API via Set-Cookie. */
export function setToken(_token: string, _remember = true): void {
  // intentionally empty
}

export function clearToken(): void {
  // Server clears httpOnly + CSRF on /auth/logout; drop any stale CSRF hint locally.
  if (typeof document !== 'undefined') {
    document.cookie = `${CSRF_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`;
  }
}

export function requireToken(): string {
  const token = getToken();
  if (!token) {
    throw new Error('UNAUTHORIZED');
  }
  return token;
}

export function isCookieSession(token: string | null | undefined): boolean {
  return token === COOKIE_SESSION || (!token && hasSession());
}
