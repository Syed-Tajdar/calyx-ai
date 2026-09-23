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

  @Get('readyz')
  async readyz(): Promise<{ status: string }> {
    try {
      await this.postgres.ping();
      await this.clickhouse.ping();
    } catch {
      throw new ServiceUnavailableException('dependencies unavailable');
    }

    const env = loadEnv();
    const kafkaLiveRequired =
      !(env.MOCK_MODE ?? true) || !(env.MOCK_EMITTER_ENABLED ?? true);
    if (kafkaLiveRequired) {
      const membership = getConsumerMembership(LIVE_FEED_GROUP_ID);
      if (!membership?.isGroupMember) {
        throw new ServiceUnavailableException('live kafka consumer not a group member');
      }
    }

    return { status: 'ok' };
  }
}
