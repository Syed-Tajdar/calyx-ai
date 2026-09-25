import { Injectable, Logger, OnModuleDestroy, OnModuleInit, ServiceUnavailableException } from '@nestjs/common';
import { Producer } from 'kafkajs';
import { loadEnv } from '../config/env';
import { createKafkaClient } from './kafka-client';

/**
 * Kafka producer connects in the background with backoff so API bootstrap
 * never waits on the broker (S0 V6 / S1B B2).
 */
@Injectable()
export class KafkaService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(KafkaService.name);
  private readonly producer: Producer;
  private connected = false;
  private stopped = false;
  private connectLoop: Promise<void> | null = null;

  constructor() {
    const env = loadEnv();
    const kafka = createKafkaClient([env.KAFKA_BROKER]);
    this.producer = kafka.producer();
  }

  async onModuleInit(): Promise<void> {
    this.connectLoop = this.connectWithBackoff();
  }

  isConnected(): boolean {
    return this.connected;
  }

  private async connectWithBackoff(): Promise<void> {
    let backoffMs = 1_000;
    while (!this.stopped && !this.connected) {
      try {
        await this.producer.connect();
        this.connected = true;
        this.logger.log('kafka producer connected');
        return;
      } catch (error: unknown) {
        this.logger.warn(
          `kafka producer connect failed; retrying in ${backoffMs}ms error=${String(error)}`,
        );
        await new Promise((r) => setTimeout(r, backoffMs));
        backoffMs = Math.min(backoffMs * 2, 30_000);
      }
    }
  }

  async publish(topic: string, key: string, value: Record<string, unknown>): Promise<void> {
    if (!this.connected) {
      throw new ServiceUnavailableException('kafka producer unavailable');
    }
    try {
      await this.producer.send({
        topic,
        messages: [{ key, value: JSON.stringify(value) }],
      });
    } catch (error: unknown) {
      this.connected = false;
      this.logger.error(`kafka publish failed error=${String(error)}`);
      if (!this.stopped) {
        this.connectLoop = this.connectWithBackoff();
      }
      throw new ServiceUnavailableException('kafka producer unavailable');
    }
  }

  async onModuleDestroy(): Promise<void> {
    this.stopped = true;
    try {
      await this.producer.disconnect();
    } catch (error: unknown) {
      this.logger.warn(`kafka producer disconnect error=${String(error)}`);
    }
    this.connected = false;
  }
}
