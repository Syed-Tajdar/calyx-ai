import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant } from '../common/tenant.decorator';
import {
  ComplianceService,
  ExposureSummary,
  RiskScoreRecord,
  ViolationRecord,
} from './compliance.service';

const showDemoField = z
  .union([z.literal('true'), z.literal('1'), z.literal('false'), z.literal('0')])
  .optional()
  .transform((v) => v === 'true' || v === '1');

const listQuerySchema = z.object({
  limit: z.string().optional(),
  show_demo: showDemoField,
});

const exposureQuerySchema = z.object({
  show_demo: showDemoField,
});

@Controller('compliance')
@FeatureGate('compliance')
@UseGuards(AuthGuard('jwt'))
export class ComplianceController {
  constructor(private readonly complianceService: ComplianceService) {}

  @Get('risk-scores')
  async riskScores(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<{ data: RiskScoreRecord[] }> {
    const parsed = listQuerySchema.parse(query);
    return this.complianceService.listRiskScores(
      tenantId,
      parsed.limit ? Number(parsed.limit) : 50,
      parsed.show_demo,
    );
  }

  @Get('violations')
  async violations(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<{ data: ViolationRecord[] }> {
    const parsed = listQuerySchema.parse(query);
    return this.complianceService.listViolations(
      tenantId,
      parsed.limit ? Number(parsed.limit) : 50,
      parsed.show_demo,
    );
  }

  @Get('exposure')
  async exposure(
    @CurrentTenant() tenantId: string,
    @Query() query: Record<string, string>,
  ): Promise<ExposureSummary> {
    const parsed = exposureQuerySchema.parse(query);
    return this.complianceService.getExposure(tenantId, parsed.show_demo);
  }
}
