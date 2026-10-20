import { Module } from '@nestjs/common';
import { EMAIL_SENDER, EmailSender } from './email.sender';
import { HttpPublisherActionClient, PUBLISHER_ACTION_CLIENT } from './platform-pause.client';
import { WorkflowActionsService } from './workflow-actions.service';
import { WorkflowEvaluatorService } from './workflow-evaluator.service';
import { WorkflowMetricsService } from './workflow-metrics.service';
import { WorkflowsController } from './workflows.controller';
import { WorkflowsService } from './workflows.service';

/** v2 Phase 2D — Workflows. Routes 404 unless FEATURE_WORKFLOWS=true; evaluator idle likewise. */
@Module({
  controllers: [WorkflowsController],
  providers: [
    WorkflowMetricsService,
    WorkflowActionsService,
    WorkflowsService,
    WorkflowEvaluatorService,
    { provide: PUBLISHER_ACTION_CLIENT, useFactory: () => new HttpPublisherActionClient() },
    { provide: EMAIL_SENDER, useFactory: () => new EmailSender() },
  ],
})
export class WorkflowsModule {}
