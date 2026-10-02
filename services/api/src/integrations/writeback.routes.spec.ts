import { ForbiddenException } from '@nestjs/common';
import { Request } from 'express';
import { AuthSecurityService } from '../auth/auth-security.service';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { IntegrationsController } from './integrations.controller';

function viewer(): JwtPayload {
  return {
    sub: 'user-viewer',
    email: 'viewer@example.com',
    tenant_id: '11111111-1111-1111-1111-111111111111',
    role: 'viewer',
  };
}

describe('writeback convert and cancel routes', () => {
  it('returns 403 for a non-admin', async () => {
    const ctrl = new IntegrationsController({
      convertWriteback: () => Promise.reject(new Error('should not run')),
      voidWriteback: () => Promise.reject(new Error('should not run')),
    } as never);
    const user = viewer();
    await expect(ctrl.convertWriteback(user.tenant_id, user, 'id', {})).rejects.toBeInstanceOf(
      ForbiddenException,
    );
    await expect(ctrl.voidWriteback(user.tenant_id, user, 'id', {})).rejects.toBeInstanceOf(
      ForbiddenException,
    );
  });

  it('pause and resume are 403 for a non-admin', async () => {
    const ctrl = new IntegrationsController({
      setActive: () => Promise.reject(new Error('should not run')),
    } as never);
    const user = viewer();
    await expect(ctrl.pause(user.tenant_id, user, 'id')).rejects.toBeInstanceOf(ForbiddenException);
    await expect(ctrl.resume(user.tenant_id, user, 'id')).rejects.toBeInstanceOf(ForbiddenException);
  });

  it('cookie session without a csrf token is 403', () => {
    process.env.APP_ENV = 'development';
    process.env.DATABASE_URL ??= 'postgres://localhost:5432/callloom';
    process.env.JWT_SECRET ??= '0123456789abcdef0123456789abcdef';
    const svc = new AuthSecurityService({} as never);
    const paths = [
      '/integrations/11111111-1111-1111-1111-111111111111/writeback/convert',
      '/integrations/11111111-1111-1111-1111-111111111111/writeback/void',
      '/integrations/11111111-1111-1111-1111-111111111111/pause',
      '/integrations/11111111-1111-1111-1111-111111111111/resume',
    ];
    for (const path of paths) {
      const req = {
        method: 'POST',
        path,
        url: path,
        headers: { origin: 'http://localhost:3101' },
        cookies: { calyx_session: 'present' },
      } as unknown as Request;
      expect(svc.assertCsrf(req)).toEqual({
        ok: false,
        status: 403,
        message: 'csrf required',
      });
    }
  });
});
