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
import { RedisModule } from './redis/redis.module';
import { ProvidersModule } from './providers/providers.module';
import { ComplianceModule } from './compliance/compliance.module';
import { AskAiModule } from './ask-ai/ask-ai.module';
import { AttributionModule } from './attribution/attribution.module';
import { NativeIntegrationModule } from './native-integration/native-integration.module';

@Module({
  imports: [
    DatabaseModule,
    RedisModule,
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
    ProvidersModule,
    ComplianceModule,
    AskAiModule,
    AttributionModule,
    NativeIntegrationModule,
  ],
})
export class AppModule {}
