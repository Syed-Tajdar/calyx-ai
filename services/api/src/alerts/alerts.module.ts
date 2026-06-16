import { Module } from '@nestjs/common';
import { AlertEngineService } from './alert-engine.service';
import { AlertsController } from './alerts.controller';
import { AlertsService } from './alerts.service';
import { MockNotificationChannel } from './channels/mock.channel';

@Module({
  controllers: [AlertsController],
  providers: [AlertsService, AlertEngineService, MockNotificationChannel],
})
export class AlertsModule {}
