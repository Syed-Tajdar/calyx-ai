import {
  Body,
  Controller,
  Get,
  Headers,
  HttpCode,
  HttpException,
  HttpStatus,
  Post,
  Req,
  Res,
  UseGuards,
} from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { Request, Response } from 'express';
import { z } from 'zod';
import { AuthService } from './auth.service';
import { AuthSecurityService } from './auth-security.service';
import { CsrfGuard } from './csrf.guard';
import { JwtPayload } from './jwt-payload.interface';

const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

@Controller('auth')
export class AuthController {
  constructor(
    private readonly authService: AuthService,
    private readonly security: AuthSecurityService,
  ) {}

  @Post('login')
  async login(
    @Body() body: unknown,
    @Req() req: Request,
    @Res({ passthrough: true }) res: Response,
    @Headers('x-calyx-client') clientHdr?: string,
  ): Promise<{ access_token?: string; csrf_token: string; ok: true }> {
    const parsed = loginSchema.parse(body);
    const ip = this.security.clientIp(req);

    const limited = await this.security.isLoginLimited(parsed.email, ip);
    if (limited.limited) {
      throw new HttpException(
        { statusCode: 429, message: 'Too many attempts' },
        HttpStatus.TOO_MANY_REQUESTS,
        { description: String(limited.retryAfterSec) },
      );
    }

    try {
      const { access_token } = await this.authService.login(
        parsed.email,
        parsed.password,
      );
      await this.security.clearFailedLogin(parsed.email, ip);
      const csrf = this.security.setSessionCookies(res, access_token);
      const scriptClient = (clientHdr ?? '').toLowerCase() === 'script';
      if (scriptClient) {
        return { ok: true, csrf_token: csrf, access_token };
      }
      // Browser never receives the JWT in a readable form.
      return { ok: true, csrf_token: csrf };
    } catch (err) {
      if (err instanceof HttpException && err.getStatus() === 429) {
        throw err;
      }
      const after = await this.security.recordFailedLogin(parsed.email, ip);
      if (after.limited) {
        throw new HttpException(
          { statusCode: 429, message: 'Too many attempts' },
          HttpStatus.TOO_MANY_REQUESTS,
        );
      }
      throw err;
    }
  }

  @Post('logout')
  @HttpCode(204)
  @UseGuards(CsrfGuard)
  logout(@Res({ passthrough: true }) res: Response): void {
    this.security.clearSessionCookies(res);
  }

  @Get('me')
  @UseGuards(AuthGuard('jwt'))
  async me(@Req() req: { user: JwtPayload }): Promise<{
    email: string;
    tenant_id: string;
    tenant_name: string;
    role: string;
    has_live_integration: boolean;
    demo_mode_enabled: boolean;
  }> {
    return this.authService.getMe(req.user);
  }
}
