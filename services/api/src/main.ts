import { NestFactory } from '@nestjs/core';
import pino from 'pino';
import pinoHttp from 'pino-http';
import { AppModule } from './app.module';
import { loadEnv } from './config/env';

async function bootstrap(): Promise<void> {
  const env = loadEnv();
  const logger = pino({ level: 'info' });

  const app = await NestFactory.create(AppModule, { logger: false });
  app.use(
    pinoHttp({
      logger,
      customProps: () => ({ service: 'api' }),
    }),
  );
  app.enableCors({ origin: true, credentials: true });

  await app.listen(env.API_PORT);
  logger.info({ port: env.API_PORT }, 'api server started');
}

bootstrap().catch((error: unknown) => {
  process.stderr.write(`api bootstrap failed: ${String(error)}\n`);
  process.exit(1);
});
