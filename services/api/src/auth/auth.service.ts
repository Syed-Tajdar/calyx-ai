import { Injectable, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { isDemoModeEnabled } from '../common/demo-rows';
import { DEMO_TENANT_NAME } from '../config/constants';
import { loadEnv } from '../config/env';
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
    has_live_integration: boolean;
    demo_mode_enabled: boolean;
  }> {
    const result = await this.postgres.withTenant(user.tenant_id, async (client) =>
      client.query<{ name: string; has_live: boolean }>(
        `SELECT
           (SELECT name FROM tenants WHERE id = $1::uuid) AS name,
           EXISTS (
             SELECT 1 FROM integrations
             WHERE tenant_id = $1::uuid
               AND deleted_at IS NULL
               AND is_active = TRUE
               AND sync_mode = 'real'
           ) AS has_live`,
        [user.tenant_id],
      ),
    );
    const mockMode = loadEnv().MOCK_MODE ?? true;
    const tenantName =
      result.rows[0]?.name ?? (mockMode ? DEMO_TENANT_NAME : 'Unknown tenant');

    return {
      email: user.email,
      tenant_id: user.tenant_id,
      tenant_name: tenantName,
      role: user.role,
      has_live_integration: Boolean(result.rows[0]?.has_live),
      demo_mode_enabled: isDemoModeEnabled(),
    };
  }
}
