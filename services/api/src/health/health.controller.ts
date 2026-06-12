import { Controller, Get, ServiceUnavailableException } from '@nestjs/common';
import { ClickhouseService } from '../database/clickhouse.service';
import { PostgresService } from '../database/postgres.service';

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
    return { status: 'ok' };
  }
}
