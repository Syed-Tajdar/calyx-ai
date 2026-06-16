import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { Kafka, Consumer } from 'kafkajs';
import { loadEnv } from '../config/env';
import { MockNotificationChannel } from './channels/mock.channel';
import { AlertsService } from './alerts.service';

interface CallScoredEvent {
  tenant_id: string;
  call_id: string;
  overall_score: number;
}

@Injectable()
export class AlertEngineService implements OnModuleInit, OnModuleDestroy {
  private consumer: Consumer | null = null;

  constructor(
    private readonly alertsService: AlertsService,
    private readonly mockChannel: MockNotificationChannel,
  ) {}

  async onModuleInit(): Promise<void> {
    const env = loadEnv();
    const kafka = new Kafka({ brokers: [env.KAFKA_BROKER] });
    this.consumer = kafka.consumer({ groupId: 'callloom-alert-engine' });

    await this.consumer.connect();
    await this.consumer.subscribe({
      topic: 'callloom.calls.call_scored.v1',
      fromBeginning: false,
    });

    await this.consumer.run({
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
    if (this.consumer) {
      await this.consumer.disconnect();
    }
  }
}
