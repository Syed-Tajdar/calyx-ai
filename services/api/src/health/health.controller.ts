import { Controller, Get, ServiceUnavailableException } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { PostgresService } from '../database/postgres.service';
import { loadEnv } from '../config/env';
import { getConsumerMembership } from '../kafka/resilient-consumer';
import { LIVE_FEED_GROUP_ID } from '../live/kafka-live-consumer.service';
import { EventLoopLagService } from './event-loop-lag.service';

const READY_PING_ATTEMPTS = 3;
const READY_PING_RETRY_MS = 150;

async function withRetries(fn: () => Promise<void>, attempts: number, gapMs: number): Promise<void> {
  let last: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      await fn();
      return;
    } catch (err: unknown) {
      last = err;
      if (i + 1 < attempts) {
        await new Promise((r) => setTimeout(r, gapMs));
      }
    }
  }
  throw last instanceof Error ? last : new Error(String(last));
}

@Controller()
export class HealthController {
  constructor(
    private readonly postgres: PostgresService,
    private readonly clickhouse: ClickhouseService,
    private readonly eventLoopLag: EventLoopLagService,
  ) {}

  @Get('healthz')
  healthz(): { status: string } {
    return { status: 'ok' };
  }

  /**
   * Readiness for serving API requests (Postgres + ClickHouse only).
   * Live Kafka consumer health is intentionally NOT here — a broker outage must not
   * remove the pod from service (login/dashboards/reports still work).
   */
  @Get('readyz')
  async readyz(): Promise<{ status: string }> {
    try {
      // Pings themselves enforce query/request timeouts and release pool clients.
      // Brief Docker network flaps while a broker container is stopped are retried.
      await withRetries(() => this.postgres.ping(), READY_PING_ATTEMPTS, READY_PING_RETRY_MS);
      await withRetries(() => this.clickhouse.ping(), READY_PING_ATTEMPTS, READY_PING_RETRY_MS);
    } catch {
      throw new ServiceUnavailableException('dependencies unavailable');
    }
    return { status: 'ok' };
  }

  /**
   * Live-feed Kafka consumer membership. Used by verify-system; not a readiness probe.
   */
  @Get('live-status')
  liveStatus(): {
    status: string;
    required: boolean;
    group_id: string;
    is_group_member: boolean;
    last_event_at: string | null;
    last_join_at: string | null;
    last_error: string | null;
    recreate_count: number;
    lag: number | null;
    event_loop_lag_ms: number;
    event_loop_max_lag_ms: number;
  } {
    const env = loadEnv();
    const required = !(env.MOCK_MODE ?? true) || !(env.MOCK_EMITTER_ENABLED ?? true);
    const membership = getConsumerMembership(LIVE_FEED_GROUP_ID);
    const isMember = Boolean(membership?.isGroupMember);
    const loop = this.eventLoopLag.snapshot();
    return {
      status: !required || isMember ? 'ok' : 'down',
      required,
      group_id: LIVE_FEED_GROUP_ID,
      is_group_member: isMember,
      last_event_at: membership?.lastEventAt ?? null,
      last_join_at: membership?.lastJoinAt ?? null,
      last_error: membership?.lastError ?? null,
      recreate_count: membership?.recreateCount ?? 0,
      // Lag requires broker admin APIs; not available in-process without extra deps.
      lag: null,
      event_loop_lag_ms: loop.lag_ms,
      event_loop_max_lag_ms: loop.max_lag_ms,
    };
  }

  @Get('event-loop-lag')
  eventLoopLagEndpoint(): {
    lag_ms: number;
    max_lag_ms: number;
    samples: number;
  } {
    return this.eventLoopLag.snapshot();
  }
}
