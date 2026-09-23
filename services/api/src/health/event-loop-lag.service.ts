import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';

/**
 * Samples Node event-loop lag via setInterval drift.
 * Used to prove whether Kafka reconnect storms starve /readyz.
 */
@Injectable()
export class EventLoopLagService implements OnModuleInit, OnModuleDestroy {
  private timer: NodeJS.Timeout | null = null;
  private expectedAt = 0;
  private readonly intervalMs = 100;
  private lagMs = 0;
  private maxLagMs = 0;
  private samples = 0;

  onModuleInit(): void {
    this.expectedAt = Date.now() + this.intervalMs;
    this.timer = setInterval(() => {
      const now = Date.now();
      const lag = Math.max(0, now - this.expectedAt);
      this.lagMs = lag;
      if (lag > this.maxLagMs) {
        this.maxLagMs = lag;
      }
      this.samples += 1;
      this.expectedAt = now + this.intervalMs;
    }, this.intervalMs);
    this.timer.unref?.();
  }

  onModuleDestroy(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  snapshot(): { lag_ms: number; max_lag_ms: number; samples: number } {
    return {
      lag_ms: this.lagMs,
      max_lag_ms: this.maxLagMs,
      samples: this.samples,
    };
  }

  /** Reset max so an outage window can be measured cleanly. */
  resetMax(): void {
    this.maxLagMs = this.lagMs;
  }
}
