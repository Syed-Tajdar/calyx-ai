import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { Consumer, EachMessagePayload } from 'kafkajs';
import { loadEnv } from '../config/env';
import { createKafkaClient } from '../kafka/kafka-client';
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
    // Prefer Kafka live whenever the synthetic emitter is off (real integration path).
    const useKafkaLive =
      !(env.MOCK_MODE ?? true) || !(env.MOCK_EMITTER_ENABLED ?? true);
    if (!useKafkaLive) {
      return;
    }

    const kafka = createKafkaClient([env.KAFKA_BROKER]);
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
      // Kafka ping envelope uses external_call_id; live WS schema uses call_id.
      const normalized = {
        schema_version: '1' as const,
        event_id:
          typeof raw.event_id === 'string' && raw.event_id.length > 0
            ? raw.event_id
            : typeof raw.external_call_id === 'string'
              ? `${raw.external_call_id}:${String(raw.received_at_unix_ms ?? '')}`
              : '',
        tenant_id: tenantId,
        call_id:
          typeof raw.call_id === 'string'
            ? raw.call_id
            : typeof raw.external_call_id === 'string'
              ? raw.external_call_id
              : '',
        platform: typeof raw.platform === 'string' ? raw.platform : '',
        ani_hash: typeof raw.ani_hash === 'string' ? raw.ani_hash : undefined,
        campaign_id: typeof raw.campaign_id === 'string' ? raw.campaign_id : undefined,
        publisher_id: typeof raw.publisher_id === 'string' ? raw.publisher_id : undefined,
        target_id: typeof raw.target_id === 'string' ? raw.target_id : undefined,
        state: typeof raw.state === 'string' ? raw.state : undefined,
        event_type: typeof raw.event_type === 'string' ? raw.event_type : undefined,
        revenue: typeof raw.revenue === 'number' ? raw.revenue : undefined,
        payout: typeof raw.payout === 'number' ? raw.payout : undefined,
        is_duplicate: typeof raw.is_duplicate === 'boolean' ? raw.is_duplicate : undefined,
        received_at_unix_ms:
          typeof raw.received_at_unix_ms === 'number' ? raw.received_at_unix_ms : Date.now(),
        occurred_at_unix_ms:
          typeof raw.occurred_at_unix_ms === 'number' ? raw.occurred_at_unix_ms : undefined,
      };
      const parsed = pingEventPayloadSchema.safeParse(normalized);
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
