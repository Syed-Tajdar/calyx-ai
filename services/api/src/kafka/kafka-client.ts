import { CompressionCodecs, CompressionTypes, Kafka } from 'kafkajs';

let registered = false;

/**
 * Register Snappy so consumers survive producers that default to Snappy
 * (e.g. `rpk topic produce`). KafkaJS core only ships gzip.
 */
export function registerKafkaCompressionCodecs(): void {
  if (registered) return;
  // kafkajs-snappy has no shipped types; runtime export matches CompressionCodecs slot.
  // eslint-disable-next-line @typescript-eslint/no-require-imports, @typescript-eslint/no-unsafe-assignment
  const SnappyCodec = require('kafkajs-snappy');
  // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
  CompressionCodecs[CompressionTypes.Snappy] = SnappyCodec;
  registered = true;
}

export function createKafkaClient(brokers: string[]): Kafka {
  registerKafkaCompressionCodecs();
  return new Kafka({
    brokers,
    // Keep reconnect attempts short so DNS failures do not monopolize the
    // libuv threadpool that Postgres/ClickHouse readiness lookups also use.
    connectionTimeout: 3_000,
    requestTimeout: 10_000,
    retry: {
      retries: 3,
      initialRetryTime: 300,
      maxRetryTime: 5_000,
    },
  });
}
