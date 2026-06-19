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

  async getMe(user: JwtPayload): Promise<{
    email: string;
    tenant_id: string;
    tenant_name: string;
    role: string;
  }> {
    const result = await this.postgres.getPool().query<{ name: string }>(
      `SELECT name FROM tenants WHERE id = $1::uuid`,
      [user.tenant_id],
    );
    const tenantName = result.rows[0]?.name ?? 'Unknown tenant';

    return {
      email: user.email,
      tenant_id: user.tenant_id,
      tenant_name: tenantName,
      role: user.role,
    };
  }
}
