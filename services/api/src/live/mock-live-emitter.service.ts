import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { randomUUID } from 'crypto';
import { DEMO_TENANT_ID } from '../config/constants';
import { loadEnv } from '../config/env';
import { LiveGateway } from './live.gateway';
import type { LiveEventEnvelope } from './live-event.types';

@Injectable()
export class MockLiveEmitterService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(MockLiveEmitterService.name);
  private static readonly PING_PLATFORMS = [
    'ringba',
    'retreaver',
    'trackdrive',
    'callgrid',
    'callloom-native',
  ] as const;
  private timer: ReturnType<typeof setInterval> | null = null;
  private tick = 0;

  constructor(private readonly gateway: LiveGateway) {}

  onModuleInit(): void {
    const env = loadEnv();
    if (!(env.MOCK_MODE ?? true)) {
      return;
    }
    this.scheduleNext();
    this.logger.log('mock live emitter started');
  }

  onModuleDestroy(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  private scheduleNext(): void {
    const delayMs = 3000 + Math.floor(Math.random() * 3000);
    this.timer = setTimeout(() => {
      this.emitMockEvents();
      this.scheduleNext();
    }, delayMs);
  }

  private emitMockEvents(): void {
    this.tick += 1;
    const now = Date.now();
    const callId = randomUUID();

    const pingEvent: LiveEventEnvelope = {
      type: 'ping_received',
      payload: {
        schema_version: '1',
        event_id: randomUUID(),
        tenant_id: DEMO_TENANT_ID,
        call_id: callId,
        platform:
          MockLiveEmitterService.PING_PLATFORMS[
            this.tick % MockLiveEmitterService.PING_PLATFORMS.length
          ],
        event_type: 'incoming',
        is_duplicate: false,
        received_at_unix_ms: now,
      },
      emitted_at: new Date(now).toISOString(),
    };
    this.gateway.emitToTenant(DEMO_TENANT_ID, pingEvent);

    if (this.tick % 5 === 0) {
      const scored: LiveEventEnvelope = {
        type: 'call_scored',
        payload: {
          schema_version: '1',
          event_id: randomUUID(),
          tenant_id: DEMO_TENANT_ID,
          call_id: callId,
          overall_score: 72 + (this.tick % 20),
          sentiment: 'positive',
          scored_at_unix_ms: now,
        },
        emitted_at: new Date(now).toISOString(),
      };
      this.gateway.emitToTenant(DEMO_TENANT_ID, scored);
    }

    if (this.tick % 7 === 0) {
      const violation: LiveEventEnvelope = {
        type: 'compliance_violation',
        payload: {
          schema_version: '1',
          event_id: randomUUID(),
          tenant_id: DEMO_TENANT_ID,
          call_id: callId,
          violation_type: 'calling_hours',
          risk_score: 42,
          exposure_usd: 12.5,
          detected_at_unix_ms: now,
        },
        emitted_at: new Date(now).toISOString(),
      };
      this.gateway.emitToTenant(DEMO_TENANT_ID, violation);
    }

    if (this.tick % 6 === 0) {
      const attributed: LiveEventEnvelope = {
        type: 'conversion_attributed',
        payload: {
          tenant_id: DEMO_TENANT_ID,
          event_id: randomUUID(),
          call_id: callId,
          platform: 'meta',
          status: 'fired',
          revenue: 18.5,
          attributed_at: new Date(now).toISOString(),
        },
        emitted_at: new Date(now).toISOString(),
      };
      this.gateway.emitToTenant(DEMO_TENANT_ID, attributed);
    }
  }
}
