import { Controller, Get } from '@nestjs/common';

@Controller()
export class HealthController {
  @Get('healthz')
  healthz(): { status: string } {
    return { status: 'ok' };
  }

  @Get('readyz')
  readyz(): { status: string } {
    // TODO: verify PostgreSQL and Redis connectivity
    return { status: 'ok' };
  }
}
