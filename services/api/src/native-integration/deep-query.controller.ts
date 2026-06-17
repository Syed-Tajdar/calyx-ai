import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import { CallDeepView, DeepQueryService } from './deep-query.service';

@Controller('native')
@UseGuards(AuthGuard('jwt'))
export class DeepQueryController {
  constructor(private readonly deepQueryService: DeepQueryService) {}

  @Get('call-deep-view')
  async callDeepView(
    @CurrentTenant() tenantId: string,
    @Query('callId') callId?: string,
  ): Promise<CallDeepView> {
    return this.deepQueryService.getCallDeepView(tenantId, callId ?? '');
  }
}
