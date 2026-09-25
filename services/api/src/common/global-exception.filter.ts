import {
  ArgumentsHost,
  Catch,
  ExceptionFilter,
  HttpException,
  HttpStatus,
  Logger,
} from '@nestjs/common';
import { randomUUID } from 'crypto';
import { Request, Response } from 'express';
import { ZodError } from 'zod';

@Catch()
export class GlobalExceptionFilter implements ExceptionFilter {
  private readonly logger = new Logger(GlobalExceptionFilter.name);

  catch(exception: unknown, host: ArgumentsHost): void {
    const ctx = host.switchToHttp();
    const res = ctx.getResponse<Response>();
    const req = ctx.getRequest<Request & { id?: string }>();
    const requestId =
      (typeof req.id === 'string' && req.id) ||
      (req.headers['x-request-id'] as string | undefined) ||
      randomUUID();

    if (exception instanceof ZodError) {
      res.status(HttpStatus.BAD_REQUEST).json({
        statusCode: HttpStatus.BAD_REQUEST,
        message: 'Validation failed',
        request_id: requestId,
        errors: exception.issues.map((i) => ({
          path: i.path.join('.'),
          message: i.message,
        })),
      });
      return;
    }

    if (exception instanceof HttpException) {
      const status = exception.getStatus();
      if (status === HttpStatus.TOO_MANY_REQUESTS && !res.getHeader('Retry-After')) {
        res.setHeader('Retry-After', '900');
      }
      const body = exception.getResponse();
      const payload =
        typeof body === 'string'
          ? { statusCode: status, message: body, request_id: requestId }
          : { ...(body as object), request_id: requestId };
      res.status(status).json(payload);
      return;
    }

    this.logger.error(
      { err: exception, request_id: requestId, path: req.url },
      'unhandled error',
    );
    res.status(HttpStatus.INTERNAL_SERVER_ERROR).json({
      statusCode: HttpStatus.INTERNAL_SERVER_ERROR,
      message: 'Internal server error',
      request_id: requestId,
    });
  }
}
