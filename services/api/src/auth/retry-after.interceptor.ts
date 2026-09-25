import {
  CallHandler,
  ExecutionContext,
  Injectable,
  NestInterceptor,
} from '@nestjs/common';
import { Observable, tap } from 'rxjs';
import { Response } from 'express';

/** Attach Retry-After when the handler throws/returns 429. */
@Injectable()
export class RetryAfterInterceptor implements NestInterceptor {
  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const res = context.switchToHttp().getResponse<Response>();
    return next.handle().pipe(
      tap({
        error: (err: { status?: number; getStatus?: () => number; message?: string }) => {
          const status = err?.getStatus?.() ?? err?.status;
          if (status === 429 && !res.getHeader('Retry-After')) {
            res.setHeader('Retry-After', '900');
          }
        },
      }),
    );
  }
}
