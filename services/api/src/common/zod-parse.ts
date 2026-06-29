import { BadRequestException } from '@nestjs/common';
import { ZodError, ZodTypeAny, z } from 'zod';

export function parseZod<S extends ZodTypeAny>(schema: S, body: unknown): z.output<S> {
  try {
    return schema.parse(body);
  } catch (error: unknown) {
    if (error instanceof ZodError) {
      throw new BadRequestException({
        statusCode: 422,
        message: 'Validation failed',
        errors: error.flatten(),
      });
    }
    throw error;
  }
}
