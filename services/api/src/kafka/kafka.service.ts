import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { Producer } from 'kafkajs';
import { loadEnv } from '../config/env';
import { createKafkaClient } from './kafka-client';

@Injectable()
export class KafkaService implements OnModuleInit, OnModuleDestroy {
  private readonly producer: Producer;

  constructor() {
    const env = loadEnv();
    const kafka = createKafkaClient([env.KAFKA_BROKER]);
    this.producer = kafka.producer();
  }

  async onModuleInit(): Promise<void> {
    await this.producer.connect();
  }

  async publish(topic: string, key: string, value: Record<string, unknown>): Promise<void> {
    await this.producer.send({
      topic,
      messages: [{ key, value: JSON.stringify(value) }],
    });
  }

  async onModuleDestroy(): Promise<void> {
    await this.producer.disconnect();
  }
}
