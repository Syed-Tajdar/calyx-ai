import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { Consumer, Kafka, EachMessagePayload } from 'kafkajs';
import { loadEnv } from '../config/env';
import { LiveGateway } from './live.gateway';
import {
  callScoredPayloadSchema,
  complianceViolationPayloadSchema,
  conversionAttributedPayloadSchema,
  LiveEventEnvelope,
  liveEventEnvelopeSchema,
  pingEventPayloadSchema,
} from './live-event.types';

@Injectable()
export class KafkaLiveConsumerService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(KafkaLiveConsumerService.name);
  private consumer: Consumer | null = null;

  constructor(private readonly gateway: LiveGateway) {}

  async onModuleInit(): Promise<void> {
    const env = loadEnv();
    if (env.MOCK_MODE ?? true) {
      return;
    }

    const kafka = new Kafka({ brokers: [env.KAFKA_BROKER] });
    this.consumer = kafka.consumer({ groupId: 'calyx-api-live-feed' });

    const topics = [
      env.KAFKA_TOPIC_PINGS,
      env.KAFKA_TOPIC_CALL_SCORED,
      env.KAFKA_TOPIC_ATTRIBUTION_ATTRIBUTED,
      env.KAFKA_TOPIC_COMPLIANCE_VIOLATION,
    ];

    try {
      await this.consumer.connect();
      for (const topic of topics) {
        await this.consumer.subscribe({ topic, fromBeginning: false });
      }
      await this.consumer.run({
        eachMessage: async (payload: EachMessagePayload) => {
          await this.handleMessage(payload);
        },
      });
      this.logger.log({ topics }, 'kafka live consumer started');
    } catch (error: unknown) {
      this.logger.error({ error: String(error) }, 'kafka live consumer failed to start');
    }
  }

  async onModuleDestroy(): Promise<void> {
    if (this.consumer) {
      await this.consumer.disconnect();
      this.consumer = null;
    }
  }

  private async handleMessage({ topic, message }: EachMessagePayload): Promise<void> {
    if (!message.value) {
      return;
    }
    const env = loadEnv();
    let raw: Record<string, unknown>;
    try {
      raw = JSON.parse(message.value.toString()) as Record<string, unknown>;
    } catch {
      return;
    }

    const tenantId = typeof raw.tenant_id === 'string' ? raw.tenant_id : '';
    if (!tenantId) {
      return;
    }

    let envelope: LiveEventEnvelope | null = null;
    const emittedAt = new Date().toISOString();

    if (topic === env.KAFKA_TOPIC_PINGS) {
      const parsed = pingEventPayloadSchema.safeParse(raw);
      if (parsed.success) {
        envelope = { type: 'ping_received', payload: parsed.data, emitted_at: emittedAt };
      }
    } else if (topic === env.KAFKA_TOPIC_CALL_SCORED) {
      const parsed = callScoredPayloadSchema.safeParse(raw);
      if (parsed.success) {
        envelope = { type: 'call_scored', payload: parsed.data, emitted_at: emittedAt };
      }
    } else if (topic === env.KAFKA_TOPIC_ATTRIBUTION_ATTRIBUTED) {
      const parsed = conversionAttributedPayloadSchema.safeParse(raw);
      if (parsed.success) {
        envelope = {
          type: 'conversion_attributed',
          payload: parsed.data,
          emitted_at: emittedAt,
        };
      }
    } else if (topic === env.KAFKA_TOPIC_COMPLIANCE_VIOLATION) {
      const parsed = complianceViolationPayloadSchema.safeParse(raw);
      if (parsed.success) {
        envelope = {
          type: 'compliance_violation',
          payload: parsed.data,
          emitted_at: emittedAt,
        };
      }
    }

    if (!envelope) {
      return;
    }

    const validated = liveEventEnvelopeSchema.safeParse(envelope);
    if (!validated.success) {
      return;
    }

    this.gateway.emitToTenant(tenantId, validated.data);
  }
}
