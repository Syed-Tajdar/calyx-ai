import { Module } from '@nestjs/common';
import { AuthModule } from './auth/auth.module';
import { DatabaseModule } from './database/database.module';
import { HealthModule } from './health/health.module';
import { IntegrationsModule } from './integrations/integrations.module';
import { PingsModule } from './pings/pings.module';

@Module({
  imports: [
    DatabaseModule,
    HealthModule,
    AuthModule,
    PingsModule,
    IntegrationsModule,
  ],
})
export class AppModule {}
