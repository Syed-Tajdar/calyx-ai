import { Body, Controller, Get, Post, Req, UseGuards } from '@nestjs/common';
import { AuthGuard } from '@nestjs/passport';
import { z } from 'zod';
import { AuthService } from './auth.service';
import { JwtPayload } from './jwt-payload.interface';

const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

@Controller('auth')
export class AuthController {
  constructor(private readonly authService: AuthService) {}

  @Post('login')
  async login(@Body() body: unknown): Promise<{ access_token: string }> {
    const parsed = loginSchema.parse(body);
    return this.authService.login(parsed.email, parsed.password);
  }

  @Get('me')
  @UseGuards(AuthGuard('jwt'))
  async me(@Req() req: { user: JwtPayload }): Promise<{
    email: string;
    tenant_id: string;
    tenant_name: string;
    role: string;
  }> {
    return this.authService.getMe(req.user);
  }
}
