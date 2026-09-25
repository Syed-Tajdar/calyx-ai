import {
  CanActivate,
  ExecutionContext,
  Injectable,
  ForbiddenException,
} from '@nestjs/common';
import { Request } from 'express';
import { AuthSecurityService } from './auth-security.service';

@Injectable()
export class CsrfGuard implements CanActivate {
  constructor(private readonly security: AuthSecurityService) {}

  canActivate(context: ExecutionContext): boolean {
    const req = context.switchToHttp().getRequest<Request>();
    const result = this.security.assertCsrf(req);
    if (!result.ok) {
      throw new ForbiddenException(result.message);
    }
    return true;
  }
}
