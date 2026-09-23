import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { loadEnv } from '../config/env';
import { createKafkaClient } from '../kafka/kafka-client';
import { startResilientConsumer } from '../kafka/resilient-consumer';
import { MockNotificationChannel } from './channels/mock.channel';
import { AlertsService } from './alerts.service';

interface CallScoredEvent {
  tenant_id: string;
  call_id: string;
  overall_score: number;
}

export const ALERT_ENGINE_GROUP_ID = 'callloom-alert-engine';

type Stoppable = { stop: () => Promise<void> };

@Injectable()
export class AlertEngineService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(AlertEngineService.name);
  private managed: Stoppable | null = null;

  constructor(
    private readonly alertsService: AlertsService,
    private readonly mockChannel: MockNotificationChannel,
  ) {}

  async onModuleInit(): Promise<void> {
    const env = loadEnv();
    const kafka = createKafkaClient([env.KAFKA_BROKER]);

    this.managed = await startResilientConsumer({
      kafka,
      groupId: ALERT_ENGINE_GROUP_ID,
      topics: ['callloom.calls.call_scored.v1'],
      logger: this.logger,
      eachMessage: async ({ message }) => {
        if (!message.value) {
          return;
        }
        const evt = JSON.parse(message.value.toString()) as CallScoredEvent;
        await this.evaluate(evt);
      },
    });
  }

  private async evaluate(evt: CallScoredEvent): Promise<void> {
    const rules = await this.alertsService.list(evt.tenant_id);
    for (const rule of rules) {
      const cond = rule.condition as { field?: string; op?: string };
      if (cond.field === 'overall_score' && evt.overall_score < rule.threshold) {
        await this.mockChannel.send({
          tenantId: evt.tenant_id,
          ruleName: rule.name,
          message: `Call ${evt.call_id} score ${evt.overall_score} below threshold ${rule.threshold}`,
        });
      }
    }
  }

  async onModuleDestroy(): Promise<void> {
    if (this.managed) {
      await this.managed.stop();
      this.managed = null;
    }
  }
}
