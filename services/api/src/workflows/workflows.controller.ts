import {
  Body,
  Controller,
  Delete,
  ForbiddenException,
  Get,
  HttpCode,
  Param,
  ParseUUIDPipe,
  Patch,
  Post,
  Query,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { FeatureGate } from '../common/feature-gate';
import { CurrentTenant, CurrentUser } from '../common/tenant.decorator';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { WorkflowActionsService } from './workflow-actions.service';
import { WorkflowsService } from './workflows.service';
import { createRuleSchema, updateRuleSchema } from './workflow-types';

const platformSchema = z.enum(['ringba', 'retreaver']);
const publisherIdSchema = z.string().trim().min(1).max(200);
const protectedSchema = z
  .object({ platform: platformSchema, publisher_id: publisherIdSchema, name: z.string().trim().max(200).default('') })
  .strict();
const allowSchema = z.object({ allow: z.boolean() }).strict();
const alertStatusSchema = z.enum(['open', 'acknowledged', 'actioned', 'resolved']).optional();
const impactSchema = z.object({ platform: platformSchema, publisher_id: publisherIdSchema }).strict();
const uuid = new ParseUUIDPipe();

/**
 * v2 Phase 2D Workflows (alert → action, publisher pause). Behind FEATURE_WORKFLOWS: the global
 * FeatureGateGuard answers 404 while it is off, before auth. Every route is admin only (403);
 * POST/PATCH/DELETE are also covered by the global CSRF guard.
 */
@Controller('workflows')
@FeatureGate('workflows')
@UseGuards(AuthGuard('jwt'))
export class WorkflowsController {
  constructor(
    private readonly workflows: WorkflowsService,
    private readonly actions: WorkflowActionsService,
  ) {}

  private admin(user: JwtPayload): void {
    if (user?.role !== 'admin') throw new ForbiddenException('Workflows are for admins');
  }

  @Get('status')
  status(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return this.workflows.status(t);
  }

  // ---- rules ----

  @Get('rules')
  async listRules(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return { data: await this.workflows.listRules(t) };
  }

  @Post('rules')
  createRule(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Body() body: unknown) {
    this.admin(u);
    return this.workflows.createRule(t, u.sub ?? null, createRuleSchema.parse(body));
  }

  @Post('rules/dry-run')
  @HttpCode(200)
  dryRunUnsaved(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Body() body: unknown) {
    this.admin(u);
    return this.workflows.dryRunUnsaved(t, createRuleSchema.parse(body));
  }

  @Patch('rules/:id')
  updateRule(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Param('id', uuid) id: string,
    @Body() body: unknown,
  ) {
    this.admin(u);
    return this.workflows.updateRule(t, id, updateRuleSchema.parse(body));
  }

  @Delete('rules/:id')
  @HttpCode(204)
  async deleteRule(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string) {
    this.admin(u);
    await this.workflows.deleteRule(t, id);
  }

  @Post('rules/:id/dry-run')
  @HttpCode(200)
  dryRunSaved(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string) {
    this.admin(u);
    return this.workflows.dryRunSaved(t, id);
  }

  // ---- alert centre ----

  @Get('alerts')
  listAlerts(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Query('status') status?: string,
    @Query('limit') limit?: string,
  ) {
    this.admin(u);
    const s = alertStatusSchema.parse(status || undefined);
    const n = Number(limit ?? 100);
    return this.workflows.listAlerts(t, s, Number.isFinite(n) ? n : 100);
  }

  @Post('alerts/:id/ack')
  @HttpCode(200)
  ack(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string) {
    this.admin(u);
    return this.workflows.acknowledge(t, u.sub, id);
  }

  @Post('alerts/:id/pause')
  @HttpCode(200)
  pause(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string) {
    this.admin(u);
    return this.actions.manualPause(t, u.sub, id);
  }

  @Get('pauses')
  async pauses(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return { data: await this.workflows.listOpenPauses(t) };
  }

  @Post('pauses/:id/unpause')
  @HttpCode(200)
  unpause(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Param('id', uuid) id: string) {
    this.admin(u);
    return this.actions.unpause(t, id, 'manual', u.sub);
  }

  @Get('publisher-impact')
  impact(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Query('platform') platform?: string,
    @Query('publisher_id') publisherId?: string,
  ) {
    this.admin(u);
    const q = impactSchema.parse({ platform, publisher_id: publisherId });
    return this.workflows.publisherImpact(t, q.platform, q.publisher_id);
  }

  @Get('audit')
  async audit(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return { data: await this.workflows.listAudit(t) };
  }

  // ---- protected publishers ----

  @Get('protected-publishers')
  async listProtected(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return { data: await this.workflows.listProtected(t) };
  }

  @Post('protected-publishers')
  addProtected(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload, @Body() body: unknown) {
    this.admin(u);
    return this.workflows.addProtected(t, u.sub ?? null, protectedSchema.parse(body));
  }

  @Delete('protected-publishers/:platform/:publisherId')
  @HttpCode(204)
  async removeProtected(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Param('platform') platform: string,
    @Param('publisherId') publisherId: string,
  ) {
    this.admin(u);
    await this.workflows.removeProtected(t, platformSchema.parse(platform), publisherIdSchema.parse(publisherId));
  }

  // ---- per-integration toggle ----

  @Get('integrations')
  async integrations(@CurrentTenant() t: string, @CurrentUser() u: JwtPayload) {
    this.admin(u);
    return { data: await this.workflows.listIntegrations(t) };
  }

  @Patch('integrations/:id/allow-pause')
  allowPause(
    @CurrentTenant() t: string,
    @CurrentUser() u: JwtPayload,
    @Param('id', uuid) id: string,
    @Body() body: unknown,
  ) {
    this.admin(u);
    return this.workflows.setAllowPause(t, u.sub, id, allowSchema.parse(body).allow);
  }
}
