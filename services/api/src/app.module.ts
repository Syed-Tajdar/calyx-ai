import { Module } from '@nestjs/common';
import { AuthModule } from './auth/auth.module';
import { DatabaseModule } from './database/database.module';
import { HealthModule } from './health/health.module';
import { IntegrationsModule } from './integrations/integrations.module';
import { PingsModule } from './pings/pings.module';
import { KafkaModule } from './kafka/kafka.module';
import { ScoringModule } from './scoring/scoring.module';
import { EnrichmentModule } from './enrichment/enrichment.module';
import { CoverageModule } from './coverage/coverage.module';
import { AnalyticsModule } from './analytics/analytics.module';
import { AlertsModule } from './alerts/alerts.module';

@Module({
  imports: [
    DatabaseModule,
    KafkaModule,
    HealthModule,
    AuthModule,
    PingsModule,
    IntegrationsModule,
    ScoringModule,
    EnrichmentModule,
    CoverageModule,
    AnalyticsModule,
    AlertsModule,
  ],
})
export class AppModule {}
