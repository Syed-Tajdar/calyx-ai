import { Controller, Get, Header, Query, Res, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { Response } from 'express';
import { z } from 'zod';
import { optionalIsoDate } from '../common/date-params';
import { CurrentTenant } from '../common/tenant.decorator';
import { parseZod } from '../common/zod-parse';
import { PING_CONVERSION_FILTERS } from './pings.filters';
import { PingListResult, PingsService } from './pings.service';

const listQuerySchema = z.object({
  limit: z.coerce.number().int().min(1).max(100).optional(),
  offset: z.coerce.number().int().min(0).optional(),
  platform: z.string().optional(),
  from: optionalIsoDate,
  to: optionalIsoDate,
  search: z.string().optional(),
  show_demo: z
    .union([z.literal('true'), z.literal('1'), z.literal('false'), z.literal('0')])
    .optional()
    .transform((v) => v === 'true' || v === '1'),
  conversion: z.enum(PING_CONVERSION_FILTERS).optional(),
  // Raw event stream (one row per sync) instead of one row per call.
  raw: z
    .union([z.literal('true'), z.literal('1'), z.literal('false'), z.literal('0')])
    .optional()
    .transform((v) => v === 'true' || v === '1'),
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
    const parsed = parseZod(listQuerySchema, query);
    return this.pingsService.list(tenantId, parsed.limit, parsed.offset, {
      platform: parsed.platform,
      from: parsed.from,
      to: parsed.to,
      search: parsed.search,
      show_demo: parsed.show_demo,
      conversion: parsed.conversion,
      raw: parsed.raw,
    });
  }

  @Get('export')
  @Header('Content-Type', 'text/csv')
  @Header('Content-Disposition', 'attachment; filename="pings-export.csv"')
  async export(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
    @Res() res: Response,
  ): Promise<void> {
    const parsed = parseZod(listQuerySchema, query);
    const csv = await this.pingsService.exportCsv(tenantId, {
      platform: parsed.platform,
      from: parsed.from,
      to: parsed.to,
      search: parsed.search,
      show_demo: parsed.show_demo,
      conversion: parsed.conversion,
      raw: parsed.raw,
    });
    res.send(csv);
  }
}
