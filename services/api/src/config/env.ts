import { z } from 'zod';

const envSchema = z.object({
  API_PORT: z.coerce.number().default(3000),
  DATABASE_URL: z.string().min(1),
  JWT_SECRET: z.string().min(32),
  CLICKHOUSE_URL: z.string().default('http://localhost:8123'),
  CLICKHOUSE_DB: z.string().default('default'),
});

export type EnvConfig = z.infer<typeof envSchema>;

export function loadEnv(): EnvConfig {
  const result = envSchema.safeParse(process.env);
  if (!result.success) {
    throw new Error(`Invalid environment: ${result.error.message}`);
  }
  return result.data;
}
