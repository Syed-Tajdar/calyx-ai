import {
  OnGatewayConnection,
  OnGatewayDisconnect,
  WebSocketGateway,
  WebSocketServer,
} from '@nestjs/websockets';
import { Injectable, Logger } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { Server, Socket } from 'socket.io';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { LiveEventEnvelope, tenantRoom } from './live-event.types';

@Injectable()
@WebSocketGateway({
  namespace: '/live',
  cors: { origin: true, credentials: true },
})
export class LiveGateway implements OnGatewayConnection, OnGatewayDisconnect {
  private readonly logger = new Logger(LiveGateway.name);

  @WebSocketServer()
  server!: Server;

  constructor(private readonly jwtService: JwtService) {}

  async handleConnection(client: Socket): Promise<void> {
    const token = client.handshake.auth?.token as string | undefined;
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
