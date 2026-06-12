import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { CurrentTenant } from '../common/tenant.decorator';
import { PingListResult, PingsService } from './pings.service';

const listQuerySchema = z.object({
  limit: z.coerce.number().int().min(1).max(100).optional(),
  offset: z.coerce.number().int().min(0).optional(),
  platform: z.string().optional(),
});

@Controller('pings')
@UseGuards(AuthGuard('jwt'))
export class PingsController {
  constructor(private readonly pingsService: PingsService) {}

  @Get()
  async list(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<PingListResult> {
    const parsed = listQuerySchema.parse(query);
    return this.pingsService.list(
      tenantId,
      parsed.limit,
      parsed.offset,
      parsed.platform,
    );
  }
}
