import { Body, Controller, Post } from '@nestjs/common';
import { z } from 'zod';
import { AuthService } from './auth.service';

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
}
