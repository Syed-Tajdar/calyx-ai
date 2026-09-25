import { Module } from '@nestjs/common';
import { APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { JwtModule } from '@nestjs/jwt';
import { PassportModule } from '@nestjs/passport';
import { loadEnv } from '../config/env';
import { RedisModule } from '../redis/redis.module';
import { AuthController } from './auth.controller';
import { AuthService } from './auth.service';
import { AuthSecurityService } from './auth-security.service';
import { CsrfGuard } from './csrf.guard';
import { JwtStrategy } from './jwt.strategy';
import { RetryAfterInterceptor } from './retry-after.interceptor';

@Module({
  imports: [
    PassportModule,
    RedisModule,
    JwtModule.register({
      secret: loadEnv().JWT_SECRET,
      // 24h meets the ≤24h bar; shorter sessions can be configured later.
      signOptions: { expiresIn: '24h' },
    }),
  ],
  controllers: [AuthController],
  providers: [
    AuthService,
    AuthSecurityService,
    JwtStrategy,
    CsrfGuard,
    { provide: APP_GUARD, useClass: CsrfGuard },
    { provide: APP_INTERCEPTOR, useClass: RetryAfterInterceptor },
  ],
  exports: [AuthService, JwtModule, AuthSecurityService, CsrfGuard],
})
export class AuthModule {}
