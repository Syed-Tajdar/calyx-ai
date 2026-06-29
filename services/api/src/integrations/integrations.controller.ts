import { Controller, Delete, Get, HttpCode, Param, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import { IntegrationRecord, IntegrationsService } from './integrations.service';

@Controller('integrations')
@UseGuards(AuthGuard('jwt'))
export class IntegrationsController {
  constructor(private readonly integrationsService: IntegrationsService) {}

  @Get()
  async list(@CurrentTenant() tenantId: string): Promise<IntegrationRecord[]> {
    return this.integrationsService.list(tenantId);
  }

  @Delete(':id')
  @HttpCode(204)
  async remove(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<void> {
    await this.integrationsService.softDelete(tenantId, id);
  }
}
