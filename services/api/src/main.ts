import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';

async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule);
  await app.listen(process.env.API_PORT ?? 3000);
}

bootstrap().catch((error: unknown) => {
  process.stderr.write(`api bootstrap failed: ${String(error)}\n`);
  process.exit(1);
});
