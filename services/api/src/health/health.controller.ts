import { Controller, Get, ServiceUnavailableException } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { PostgresService } from '../database/postgres.service';
import { loadEnv } from '../config/env';
import { getConsumerMembership } from '../kafka/resilient-consumer';
import { LIVE_FEED_GROUP_ID } from '../live/kafka-live-consumer.service';

@Controller()
export class HealthController {
  constructor(
    private readonly postgres: PostgresService,
    private readonly clickhouse: ClickhouseService,
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
      await this.postgres.ping();
      await this.clickhouse.ping();
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
  } {
    const env = loadEnv();
    const required = !(env.MOCK_MODE ?? true) || !(env.MOCK_EMITTER_ENABLED ?? true);
    const membership = getConsumerMembership(LIVE_FEED_GROUP_ID);
    const isMember = Boolean(membership?.isGroupMember);
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
    };
  }
}
