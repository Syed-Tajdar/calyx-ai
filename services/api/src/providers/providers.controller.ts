import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  Patch,
  Post,
  Query,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant } from '../common/tenant.decorator';
import { ProvidersService } from './providers.service';

@Controller('providers')
@FeatureGate('providers')
@UseGuards(AuthGuard('jwt'))
export class ProvidersController {
  constructor(private readonly providersService: ProvidersService) {}

  @Get('catalog')
  getCatalog(): { version: string; providers: unknown[] } {
    return this.providersService.getCatalog();
  }

  @Get('bindings')
  async listBindings(
    @CurrentTenant() tenantId: string,
  ): Promise<{ data: unknown[] }> {
    return this.providersService.listBindings(tenantId);
  }

  @Post('bindings')
  async createBinding(
    @CurrentTenant() tenantId: string,
    @Body() body: unknown,
  ): Promise<unknown> {
    return this.providersService.createBinding(tenantId, body);
  }

  @Patch('bindings/:id')
  async updateBinding(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
    @Body() body: unknown,
  ): Promise<unknown> {
    return this.providersService.updateBinding(tenantId, id, body);
  }

  @Delete('bindings/:id')
  @HttpCode(204)
  async deleteBinding(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<void> {
    await this.providersService.softDeleteBinding(tenantId, id);
  }

  @Post('bindings/:id/test')
  async testBinding(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
    @Query('domain') domain?: string,
  ): Promise<Record<string, unknown>> {
    const resolvedDomain = domain ?? 'attribution';
    return this.providersService.resolve(tenantId, resolvedDomain, `test-${id}`);
  }

  @Post('resolve')
  async resolve(
    @CurrentTenant() tenantId: string,
    @Body() body: { domain: string; trace_id?: string },
  ): Promise<Record<string, unknown>> {
    return this.providersService.resolve(tenantId, body.domain, body.trace_id);
  }
}
