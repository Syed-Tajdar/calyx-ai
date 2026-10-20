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
import { parseZod } from '../common/zod-parse';
import {
  createScorecardSchema,
  updateScorecardSchema,
} from './scorecards.schemas';
import { ScorecardRecord, ScorecardsService } from './scorecards.service';

@Controller('scorecards')
@FeatureGate('qa')
@UseGuards(AuthGuard('jwt'))
export class ScorecardsController {
  constructor(private readonly scorecardsService: ScorecardsService) {}

  @Get()
  async list(
    @CurrentTenant() tenantId: string,
    @Query('limit') limitRaw?: string,
    @Query('offset') offsetRaw?: string,
  ): Promise<{ data: ScorecardRecord[]; total: number }> {
    const limit = Math.min(Math.max(Number(limitRaw ?? 25) || 25, 1), 100);
    const offset = Math.max(Number(offsetRaw ?? 0) || 0, 0);
    return this.scorecardsService.list(tenantId, limit, offset);
  }

  @Get(':id')
  async getOne(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<ScorecardRecord> {
    return this.scorecardsService.getById(tenantId, id);
  }

  @Post()
  async create(
    @CurrentTenant() tenantId: string,
    @Body() body: unknown,
  ): Promise<ScorecardRecord> {
    const parsed = parseZod(createScorecardSchema, body);
    return this.scorecardsService.create(tenantId, parsed);
  }

  @Patch(':id')
  async update(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
    @Body() body: unknown,
  ): Promise<ScorecardRecord> {
    const parsed = parseZod(updateScorecardSchema, body);
    return this.scorecardsService.update(tenantId, id, parsed);
  }

  @Delete(':id')
  @HttpCode(204)
  async remove(
    @CurrentTenant() tenantId: string,
    @Param('id') id: string,
  ): Promise<void> {
    await this.scorecardsService.softDelete(tenantId, id);
  }
}
