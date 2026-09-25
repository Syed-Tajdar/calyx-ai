import { Injectable, Logger } from '@nestjs/common';
import { createHash, randomBytes, timingSafeEqual } from 'crypto';
import { Request, Response } from 'express';
import { loadEnv, parseWebOrigins } from '../config/env';
import { RedisService } from '../redis/redis.service';

/** Failed logins per email before 429. Short window limits credential stuffing. */
export const LOGIN_EMAIL_MAX = 5;
export const LOGIN_EMAIL_WINDOW_SEC = 15 * 60;

/**
 * Failed logins per IP — higher than per-email so NAT / office shares are not
 * locked out as easily, still bounds spray from one address.
 */
export const LOGIN_IP_MAX = 30;
export const LOGIN_IP_WINDOW_SEC = 15 * 60;

const SESSION_COOKIE = () => loadEnv().SESSION_COOKIE_NAME;
const CSRF_COOKIE = () => loadEnv().CSRF_COOKIE_NAME;

@Injectable()
export class AuthSecurityService {
  private readonly logger = new Logger(AuthSecurityService.name);

  constructor(private readonly redis: RedisService) {}

  clientIp(req: Request): string {
    const env = loadEnv();
    const peer = req.socket.remoteAddress ?? '';
    const trusted = env.TRUSTED_PROXIES.split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    if (trusted.length === 0) {
      return peer || 'unknown';
    }
    const peerNorm = peer.replace(/^::ffff:/, '');
    const peerTrusted = trusted.some(
      (t) => t === peer || t === peerNorm || t === '*',
    );
    if (!peerTrusted) {
      return peer || 'unknown';
    }
    const xff = req.headers['x-forwarded-for'];
    if (typeof xff === 'string' && xff.length > 0) {
      return xff.split(',')[0]?.trim() || peer || 'unknown';
    }
    return peer || 'unknown';
  }

  async recordFailedLogin(email: string, ip: string): Promise<{
    limited: boolean;
    retryAfterSec: number;
  }> {
    const client = await this.redis.getClient();
    const emailKey = `login:fail:email:${email.toLowerCase()}`;
    const ipKey = `login:fail:ip:${ip}`;
    const emailCount = await client.incr(emailKey);
    if (emailCount === 1) await client.expire(emailKey, LOGIN_EMAIL_WINDOW_SEC);
    const ipCount = await client.incr(ipKey);
    if (ipCount === 1) await client.expire(ipKey, LOGIN_IP_WINDOW_SEC);

    if (emailCount > LOGIN_EMAIL_MAX) {
      const ttl = await client.ttl(emailKey);
      return { limited: true, retryAfterSec: Math.max(ttl, 1) };
    }
    if (ipCount > LOGIN_IP_MAX) {
      const ttl = await client.ttl(ipKey);
      return { limited: true, retryAfterSec: Math.max(ttl, 1) };
    }
    return { limited: false, retryAfterSec: 0 };
  }

  async isLoginLimited(
    email: string,
    ip: string,
  ): Promise<{ limited: boolean; retryAfterSec: number }> {
    const client = await this.redis.getClient();
    const emailKey = `login:fail:email:${email.toLowerCase()}`;
    const ipKey = `login:fail:ip:${ip}`;
    const [emailCount, emailTtl, ipCount, ipTtl] = await Promise.all([
      client.get(emailKey),
      client.ttl(emailKey),
      client.get(ipKey),
      client.ttl(ipKey),
    ]);
    const ec = Number(emailCount ?? 0);
    const ic = Number(ipCount ?? 0);
    if (ec >= LOGIN_EMAIL_MAX) {
      return { limited: true, retryAfterSec: Math.max(emailTtl, 1) };
    }
    if (ic >= LOGIN_IP_MAX) {
      return { limited: true, retryAfterSec: Math.max(ipTtl, 1) };
    }
    return { limited: false, retryAfterSec: 0 };
  }

  async clearFailedLogin(email: string, ip: string): Promise<void> {
    const client = await this.redis.getClient();
    await client.del(`login:fail:email:${email.toLowerCase()}`);
    await client.del(`login:fail:ip:${ip}`);
  }

  setSessionCookies(res: Response, jwt: string): string {
    const env = loadEnv();
    const csrf = randomBytes(32).toString('base64url');
    const secure = env.APP_ENV === 'production';
    const domain = env.COOKIE_DOMAIN || undefined;
    const common = {
      domain,
      sameSite: 'lax' as const,
      path: '/',
      maxAge: 24 * 60 * 60 * 1000,
    };
    res.cookie(SESSION_COOKIE(), jwt, {
      ...common,
      httpOnly: true,
      secure,
    });
    res.cookie(CSRF_COOKIE(), csrf, {
      ...common,
      httpOnly: false,
      secure,
    });
    return csrf;
  }

  clearSessionCookies(res: Response): void {
    const env = loadEnv();
    const domain = env.COOKIE_DOMAIN || undefined;
    const secure = env.APP_ENV === 'production';
    for (const name of [SESSION_COOKIE(), CSRF_COOKIE()]) {
      res.clearCookie(name, {
        domain,
        path: '/',
        sameSite: 'lax',
        secure,
        httpOnly: name === SESSION_COOKIE(),
      });
    }
  }

  /** Double-submit CSRF + Origin allowlist. Bearer-only requests skip CSRF. */
  assertCsrf(req: Request): { ok: true } | { ok: false; status: number; message: string } {
    const auth = req.headers.authorization;
    if (typeof auth === 'string' && auth.toLowerCase().startsWith('bearer ')) {
      return { ok: true };
    }
    const method = req.method.toUpperCase();
    if (method === 'GET' || method === 'HEAD' || method === 'OPTIONS') {
      return { ok: true };
    }
    const path = (req.path || req.url || '').split('?')[0];
    // Login establishes the session — no prior CSRF cookie exists yet.
    if (path === '/auth/login') {
      return { ok: true };
    }

    const origins = parseWebOrigins();
    const origin = req.headers.origin;
    if (origin) {
      if (!origins.includes(origin)) {
        return { ok: false, status: 403, message: 'origin not allowed' };
      }
    } else {
      const hasSession = Boolean(req.cookies?.[SESSION_COOKIE()]);
      if (hasSession) {
        return { ok: false, status: 403, message: 'origin required' };
      }
      return { ok: true };
    }

    const cookieTok = req.cookies?.[CSRF_COOKIE()] as string | undefined;
    const headerTok =
      (req.headers['x-csrf-token'] as string | undefined) ||
      (req.headers['x-xsrf-token'] as string | undefined);
    if (!cookieTok || !headerTok) {
      return { ok: false, status: 403, message: 'csrf required' };
    }
    const a = Buffer.from(cookieTok);
    const b = Buffer.from(headerTok);
    if (a.length !== b.length || !timingSafeEqual(a, b)) {
      return { ok: false, status: 403, message: 'csrf invalid' };
    }
    return { ok: true };
  }

  hashTokenPreview(token: string): string {
    return createHash('sha256').update(token).digest('hex').slice(0, 12);
  }
}
