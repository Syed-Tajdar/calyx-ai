import { Logger } from '@nestjs/common';
import { Consumer, EachMessageHandler, Kafka } from 'kafkajs';

export type ConsumerMembershipState = {
  groupId: string;
  /** False until GROUP_JOIN; cleared on CRASH/STOP/DISCONNECT. */
  isGroupMember: boolean;
  lastEventAt: string;
  lastError?: string;
};

const membershipByGroup = new Map<string, ConsumerMembershipState>();

export function getConsumerMembership(groupId: string): ConsumerMembershipState | undefined {
  return membershipByGroup.get(groupId);
}

function setMembership(groupId: string, patch: Partial<ConsumerMembershipState>): void {
  const prev = membershipByGroup.get(groupId) ?? {
    groupId,
    isGroupMember: false,
    lastEventAt: new Date().toISOString(),
  };
  membershipByGroup.set(groupId, {
    ...prev,
    ...patch,
    groupId,
    lastEventAt: new Date().toISOString(),
  });
}

export type ResilientConsumerOptions = {
  kafka: Kafka;
  groupId: string;
  topics: string[];
  eachMessage: EachMessageHandler;
  logger: Logger;
  /**
   * If not a group member for this long, exit the process so Docker restarts the
   * container. Covers the KafkaJS "Restarting in Nms" hang (31m gap observed).
   * Default 60s.
   */
  deadForMs?: number;
};

/**
 * KafkaJS consumer with membership tracking + process exit if recovery stalls.
 *
 * KafkaJS already schedules restartOnFailure after Crash; the 2026-09-23 incident
 * logged Restarting then stayed Stopped for ~31m. We keep restartOnFailure=true and
 * exit the process if we are not a group member for deadForMs so compose restart
 * brings a fresh consumer.
 */
export async function startResilientConsumer(opts: ResilientConsumerOptions): Promise<Consumer> {
  const deadForMs = opts.deadForMs ?? 60_000;
  const consumer = opts.kafka.consumer({
    groupId: opts.groupId,
    retry: {
      restartOnFailure: async () => true,
    },
  });

  setMembership(opts.groupId, { isGroupMember: false });

  consumer.on(consumer.events.GROUP_JOIN, () => {
    setMembership(opts.groupId, { isGroupMember: true, lastError: undefined });
    opts.logger.log(`kafka consumer group join groupId=${opts.groupId}`);
  });

  consumer.on(consumer.events.CRASH, (event) => {
    const error = String(event?.payload?.error ?? 'crash');
    const willRestart = Boolean(event?.payload?.restart);
    setMembership(opts.groupId, { isGroupMember: false, lastError: error });
    opts.logger.error(
      `kafka consumer crash groupId=${opts.groupId} restart=${willRestart} error=${error}`,
    );
    if (!willRestart) {
      opts.logger.error(
        `kafka consumer crash non-restartable groupId=${opts.groupId} — exiting process`,
      );
      process.exit(1);
    }
  });

  consumer.on(consumer.events.STOP, () => {
    setMembership(opts.groupId, { isGroupMember: false });
    opts.logger.warn(`kafka consumer stopped groupId=${opts.groupId}`);
  });

  consumer.on(consumer.events.DISCONNECT, () => {
    setMembership(opts.groupId, { isGroupMember: false });
  });

  let notMemberSince: number | null = Date.now();
  const watchdog = setInterval(() => {
    const state = getConsumerMembership(opts.groupId);
    if (state?.isGroupMember) {
      notMemberSince = null;
      return;
    }
    if (notMemberSince === null) {
      notMemberSince = Date.now();
    }
    const deadMs = Date.now() - notMemberSince;
    if (deadMs >= deadForMs) {
      opts.logger.error(
        `kafka consumer not a group member for ${deadMs}ms groupId=${opts.groupId} — exiting process for container restart`,
      );
      clearInterval(watchdog);
      process.exit(1);
    }
  }, 5_000);
  watchdog.unref?.();

  await consumer.connect();
  for (const topic of opts.topics) {
    await consumer.subscribe({ topic, fromBeginning: false });
  }
  // Do not await run() — it only settles when the consumer fully stops.
  void consumer
    .run({ eachMessage: opts.eachMessage })
    .catch((error: unknown) => {
      opts.logger.error(
        `kafka consumer run rejected groupId=${opts.groupId} error=${String(error)}`,
      );
      setMembership(opts.groupId, { isGroupMember: false, lastError: String(error) });
    });

  return consumer;
}
