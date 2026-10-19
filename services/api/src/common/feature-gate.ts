import {
  CanActivate,
  ExecutionContext,
  Injectable,
  NotFoundException,
  SetMetadata,
} from '@nestjs/common';
import { Reflector } from '@nestjs/core';

/**
 * Route gating for features that are out of v1 scope.
 *
 * v1 is Ringba / Retreaver call data plus Convert / Cancel. The other API surfaces still run on
 * mock data, so with `V1_ONLY` on (the default when unset) their routes answer **404**, as if
 * they did not exist. Each feature is keyed separately so a v2 flag re-opens only its own routes.
 *
 * - `V1_ONLY=false` re-opens everything (internal demo builds).
 * - Otherwise a feature is open only when its flag below is exactly `'true'`.
 * - A feature mapped to `null` has no flag: it stays closed unless `V1_ONLY=false`.
 */
export const FEATURES = [
  'ask_ai',
  'transcription',
  'qa',
  'workflows',
  'attribution',
  'compliance',
  'enrichment',
  'providers',
] as const;

export type Feature = (typeof FEATURES)[number];

export const FEATURE_FLAG_ENV: Readonly<Record<Feature, string | null>> = {
  ask_ai: 'FEATURE_ASK_AI',
  transcription: 'FEATURE_TRANSCRIPTION',
  qa: 'FEATURE_QA',
  workflows: 'FEATURE_WORKFLOWS',
  // Out of scope in v1 and v2 — no flag can re-open them.
  attribution: null,
  compliance: null,
  enrichment: null,
  // Closed in v1. v2 will re-open it for the Providers status tab: give it a flag here then.
  providers: null,
};

type Env = Record<string, string | undefined>;

export function isV1Only(env: Env = process.env): boolean {
  return env.V1_ONLY !== 'false';
}

export function isFeatureOpen(feature: Feature, env: Env = process.env): boolean {
  if (!isV1Only(env)) return true;
  const flag = FEATURE_FLAG_ENV[feature];
  return flag !== null && env[flag] === 'true';
}

export const FEATURE_GATE_KEY = 'calyx:feature-gate';

/** Mark a controller (or a single handler) as belonging to an out-of-v1 feature. */
export const FeatureGate = (feature: Feature): ClassDecorator & MethodDecorator =>
  SetMetadata(FEATURE_GATE_KEY, feature);

/**
 * Global guard. Registered as `APP_GUARD` in the root `AppModule`, which Nest scans first, so it
 * runs before the global `CsrfGuard` (AuthModule) and before every controller / handler
 * `AuthGuard('jwt')`. A closed route therefore answers 404 even to an anonymous request, and never
 * 401/403 — the response does not reveal that the route exists.
 */
@Injectable()
export class FeatureGateGuard implements CanActivate {
  constructor(private readonly reflector: Reflector) {}

  canActivate(context: ExecutionContext): boolean {
    if (context.getType() !== 'http') return true;
    const feature = this.reflector.getAllAndOverride<Feature | undefined>(FEATURE_GATE_KEY, [
      context.getHandler(),
      context.getClass(),
    ]);
    if (feature && !isFeatureOpen(feature)) {
      const req = context.switchToHttp().getRequest<{ method?: string; path?: string }>();
      // Same shape as Nest's own unknown-route 404.
      throw new NotFoundException(`Cannot ${req.method ?? 'GET'} ${req.path ?? ''}`);
    }
    return true;
  }
}
