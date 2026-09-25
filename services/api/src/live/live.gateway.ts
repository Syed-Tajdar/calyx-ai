import {
  OnGatewayConnection,
  OnGatewayDisconnect,
  WebSocketGateway,
  WebSocketServer,
} from '@nestjs/websockets';
import { Injectable, Logger } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { Server, Socket } from 'socket.io';
import { loadEnv, parseWebOrigins } from '../config/env';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { LiveEventEnvelope, tenantRoom } from './live-event.types';

function parseCookieHeader(header: string | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  if (!header) return out;
  for (const part of header.split(';')) {
    const idx = part.indexOf('=');
    if (idx < 0) continue;
    const k = part.slice(0, idx).trim();
    const v = part.slice(idx + 1).trim();
    if (k) out[k] = decodeURIComponent(v);
  }
  return out;
}

@Injectable()
@WebSocketGateway({
  namespace: '/live',
  cors: {
    origin: parseWebOrigins(),
    credentials: true,
  },
})
export class LiveGateway implements OnGatewayConnection, OnGatewayDisconnect {
  private readonly logger = new Logger(LiveGateway.name);

  @WebSocketServer()
  server!: Server;

  constructor(private readonly jwtService: JwtService) {}

  async handleConnection(client: Socket): Promise<void> {
    const env = loadEnv();
    const cookies = parseCookieHeader(client.handshake.headers.cookie);
    const fromCookie = cookies[env.SESSION_COOKIE_NAME];
    const fromAuth = client.handshake.auth?.token as string | undefined;
    const token = fromCookie || fromAuth;
    if (!token) {
      client.disconnect(true);
      return;
    }
    try {
      const payload = await this.jwtService.verifyAsync<JwtPayload>(token);
      await client.join(tenantRoom(payload.tenant_id));
      client.data.tenantId = payload.tenant_id;
      this.logger.log(
        { tenant_id: payload.tenant_id, socket_id: client.id },
        'live client connected',
      );
    } catch {
      client.disconnect(true);
    }
  }

  handleDisconnect(client: Socket): void {
    this.logger.log(
      { tenant_id: client.data.tenantId as string | undefined, socket_id: client.id },
      'live client disconnected',
    );
  }

  emitToTenant(tenantId: string, event: LiveEventEnvelope): void {
    this.server.to(tenantRoom(tenantId)).emit('live_event', event);
  }
}
