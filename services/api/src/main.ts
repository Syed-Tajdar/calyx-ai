import { NestFactory } from '@nestjs/core';
import { IoAdapter } from '@nestjs/platform-socket.io';
import cookieParser from 'cookie-parser';
import pino from 'pino';
import pinoHttp from 'pino-http';
import { AppModule } from './app.module';
import { GlobalExceptionFilter } from './common/global-exception.filter';
import { loadEnv, parseWebOrigins } from './config/env';

async function bootstrap(): Promise<void> {
  const env = loadEnv();
  const logger = pino({ level: 'info' });
  const origins = parseWebOrigins(env);

  const app = await NestFactory.create(AppModule, { logger: false });
  app.useWebSocketAdapter(new IoAdapter(app));
  app.use(cookieParser());
  app.use(
    pinoHttp({
      logger,
      customProps: () => ({ service: 'api' }),
    }),
  );
  app.enableCors({
    origin: (origin, callback) => {
      // Non-browser / same-origin tools may omit Origin.
      if (!origin || origins.includes(origin)) {
        callback(null, true);
        return;
      }
      callback(null, false);
    },
    credentials: true,
  });
  app.useGlobalFilters(new GlobalExceptionFilter());
  // Drain HTTP + OnModuleDestroy (Kafka disconnect) within K8s grace (S1B B6).
  app.enableShutdownHooks();

  await app.listen(env.API_PORT);
  logger.info({ port: env.API_PORT, app_env: env.APP_ENV }, 'api server started');
}

bootstrap().catch((error: unknown) => {
  process.stderr.write(`api bootstrap failed: ${String(error)}\n`);
  process.exit(1);
});
