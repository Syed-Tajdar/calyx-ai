import { Injectable, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { PostgresService } from '../database/postgres.service';
import { JwtPayload } from './jwt-payload.interface';

interface AuthUserRow {
  id: string;
  tenant_id: string;
  email: string;
  role: string;
}

@Injectable()
export class AuthService {
  constructor(
    private readonly postgres: PostgresService,
    private readonly jwtService: JwtService,
  ) {}

  async login(email: string, password: string): Promise<{ access_token: string }> {
    const result = await this.postgres.getPool().query<AuthUserRow>(
      `SELECT id::text, tenant_id::text, email, role
       FROM authenticate_user($1, $2)`,
      [email, password],
    );

    const user = result.rows[0];
    if (!user) {
      throw new UnauthorizedException('Invalid credentials');
    }

    const payload: JwtPayload = {
      sub: user.id,
      tenant_id: user.tenant_id,
      email: user.email,
      role: user.role,
    };

    return {
      access_token: await this.jwtService.signAsync(payload),
    };
  }
}
