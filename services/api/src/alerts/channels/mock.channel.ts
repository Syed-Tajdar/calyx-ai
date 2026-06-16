import { Injectable } from '@nestjs/common';
import { loadEnv } from '../../config/env';

export interface NotificationPayload {
  tenantId: string;
  ruleName: string;
  message: string;
}

export interface NotificationChannel {
  send(payload: NotificationPayload): Promise<void>;
}

@Injectable()
export class MockNotificationChannel implements NotificationChannel {
  async send(payload: NotificationPayload): Promise<void> {
    const env = loadEnv();
    if (env.NOTIFICATION_CHANNEL !== 'mock') {
      return;
    }
    // Structured log only — no external send in mock mode
    process.stdout.write(
      JSON.stringify({
        level: 'info',
        service: 'api',
        component: 'alerts',
        channel: 'mock',
        tenant_id: payload.tenantId,
        rule: payload.ruleName,
        message: payload.message,
      }) + '\n',
    );
  }
}
