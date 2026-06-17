import { Controller, Get, Query, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { CurrentTenant } from '../common/tenant.decorator';
import {
  ComplianceService,
  ExposureSummary,
  RiskScoreRecord,
  ViolationRecord,
} from './compliance.service';

@Controller('compliance')
@UseGuards(AuthGuard('jwt'))
export class ComplianceController {
  constructor(private readonly complianceService: ComplianceService) {}

  @Get('risk-scores')
  async riskScores(
    @CurrentTenant() tenantId: string,
    @Query('limit') limit?: string,
  ): Promise<{ data: RiskScoreRecord[] }> {
    return this.complianceService.listRiskScores(
      tenantId,
      limit ? Number(limit) : 50,
    );
  }

  @Get('violations')
  async violations(
    @CurrentTenant() tenantId: string,
    @Query('limit') limit?: string,
  ): Promise<{ data: ViolationRecord[] }> {
    return this.complianceService.listViolations(
      tenantId,
      limit ? Number(limit) : 50,
    );
  }

  @Get('exposure')
  async exposure(@CurrentTenant() tenantId: string): Promise<ExposureSummary> {
    return this.complianceService.getExposure(tenantId);
  }
}
