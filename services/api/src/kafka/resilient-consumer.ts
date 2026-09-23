import { Logger } from '@nestjs/common';
import { Consumer, EachMessageHandler, Kafka } from 'kafkajs';

export type ConsumerMembershipState = {
  groupId: string;
  /** False until GROUP_JOIN; cleared on CRASH/STOP/DISCONNECT. */
  isGroupMember: boolean;
  lastEventAt: string;
  lastJoinAt?: string;
  lastError?: string;
  recreateCount: number;
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
    recreateCount: 0,
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
   * If not a group member for this long, tear down and recreate the consumer
   * instance (not the process). Default 60s.
   */
  deadForMs?: number;
};

type ManagedConsumer = {
  stop: () => Promise<void>;
};

/**
 * KafkaJS consumer with membership tracking. On sustained non-membership, recreate
 * the consumer with backoff — never process.exit for consumer-only failures (API must
 * keep serving Postgres/ClickHouse traffic).
 */
export async function startResilientConsumer(opts: ResilientConsumerOptions): Promise<ManagedConsumer> {
  const deadForMs = opts.deadForMs ?? 60_000;
  let stopped = false;
  let current: Consumer | null = null;
  let recreating = false;
  let backoffMs = 2_000;
  let notMemberSince: number | null = Date.now();

  setMembership(opts.groupId, { isGroupMember: false, recreateCount: 0 });

  const wireEvents = (consumer: Consumer) => {
    consumer.on(consumer.events.GROUP_JOIN, () => {
      const now = new Date().toISOString();
      setMembership(opts.groupId, {
        isGroupMember: true,
        lastError: undefined,
        lastJoinAt: now,
      });
      notMemberSince = null;
      backoffMs = 2_000;
      opts.logger.log(`kafka consumer group join groupId=${opts.groupId}`);
    });

    consumer.on(consumer.events.CRASH, (event) => {
      const error = String(event?.payload?.error ?? 'crash');
      const willRestart = Boolean(event?.payload?.restart);
      setMembership(opts.groupId, { isGroupMember: false, lastError: error });
      if (notMemberSince === null) {
        notMemberSince = Date.now();
      }
      opts.logger.error(
        `kafka consumer crash groupId=${opts.groupId} restart=${willRestart} error=${error}`,
      );
      // Even if KafkaJS will not restart, our watchdog recreates the consumer.
      // Do not process.exit — that would take the whole API out of service.
    });

    consumer.on(consumer.events.STOP, () => {
      setMembership(opts.groupId, { isGroupMember: false });
      if (notMemberSince === null) {
        notMemberSince = Date.now();
      }
      opts.logger.warn(`kafka consumer stopped groupId=${opts.groupId}`);
    });

    consumer.on(consumer.events.DISCONNECT, () => {
      setMembership(opts.groupId, { isGroupMember: false });
      if (notMemberSince === null) {
        notMemberSince = Date.now();
      }
    });
  };

  const startOne = async (): Promise<Consumer> => {
    const consumer = opts.kafka.consumer({
      groupId: opts.groupId,
      retry: {
        restartOnFailure: async () => true,
      },
    });
    wireEvents(consumer);
    await consumer.connect();
    for (const topic of opts.topics) {
      await consumer.subscribe({ topic, fromBeginning: false });
    }
    void consumer.run({ eachMessage: opts.eachMessage }).catch((error: unknown) => {
      opts.logger.error(
        `kafka consumer run rejected groupId=${opts.groupId} error=${String(error)}`,
      );
      setMembership(opts.groupId, { isGroupMember: false, lastError: String(error) });
      if (notMemberSince === null) {
        notMemberSince = Date.now();
      }
    });
    return consumer;
  };

  const recreate = async (reason: string): Promise<void> => {
    if (stopped || recreating) {
      return;
    }
    recreating = true;
    const prev = getConsumerMembership(opts.groupId);
    const nextCount = (prev?.recreateCount ?? 0) + 1;
    setMembership(opts.groupId, {
      isGroupMember: false,
      lastError: reason,
      recreateCount: nextCount,
    });
    opts.logger.warn(
      `kafka consumer recreate groupId=${opts.groupId} reason=${reason} attempt=${nextCount} backoffMs=${backoffMs}`,
    );
    try {
      if (current) {
        try {
          // Bound disconnect so a dead broker cannot stall the Nest event loop
          // long enough for /readyz probes to time out.
          await Promise.race([
            current.disconnect(),
            new Promise<void>((_, reject) => {
              setTimeout(() => reject(new Error('disconnect timed out after 3000ms')), 3_000);
            }),
          ]);
        } catch (err: unknown) {
          opts.logger.warn(
            `kafka consumer disconnect during recreate groupId=${opts.groupId} error=${String(err)}`,
          );
        }
        current = null;
      }
      await new Promise((r) => setTimeout(r, backoffMs));
      backoffMs = Math.min(backoffMs * 2, 30_000);
      if (stopped) {
        return;
      }
      current = await startOne();
    } catch (err: unknown) {
      opts.logger.error(
        `kafka consumer recreate failed groupId=${opts.groupId} error=${String(err)}`,
      );
      setMembership(opts.groupId, { isGroupMember: false, lastError: String(err) });
      if (notMemberSince === null) {
        notMemberSince = Date.now();
      }
    } finally {
      recreating = false;
    }
  };

  current = await startOne();

  const watchdog = setInterval(() => {
    if (stopped) {
      return;
    }
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
      // Reset the timer so we don't recreate every 5s; recreate() itself backs off.
      notMemberSince = Date.now();
      void recreate(`not a group member for ${deadMs}ms`);
    }
  }, 5_000);
  watchdog.unref?.();

  return {
    stop: async () => {
      stopped = true;
      clearInterval(watchdog);
      if (current) {
        await current.disconnect();
        current = null;
      }
    },
  };
}
