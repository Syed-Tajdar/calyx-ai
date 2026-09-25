'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { io, Socket } from 'socket.io-client';
import { hasSession } from '@/lib/auth';

export type LiveEventType =
  | 'ping_received'
  | 'call_scored'
  | 'conversion_attributed'
  | 'compliance_violation';

export interface LiveEventEnvelope {
  type: LiveEventType;
  payload: Record<string, unknown>;
  emitted_at: string;
}

const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? 'http://localhost:3000';

interface UseLiveFeedOptions {
  enabled?: boolean;
  onEvent?: (event: LiveEventEnvelope) => void;
}

interface UseLiveFeedResult {
  connected: boolean;
  events: LiveEventEnvelope[];
  lastEvent: LiveEventEnvelope | null;
}

export function useLiveFeed({
  enabled = true,
  onEvent,
}: UseLiveFeedOptions = {}): UseLiveFeedResult {
  const [connected, setConnected] = useState(false);
  const [events, setEvents] = useState<LiveEventEnvelope[]>([]);
  const [lastEvent, setLastEvent] = useState<LiveEventEnvelope | null>(null);
  const socketRef = useRef<Socket | null>(null);
  const onEventRef = useRef(onEvent);

  useEffect(() => {
    onEventRef.current = onEvent;
  }, [onEvent]);

  const handleEvent = useCallback((event: LiveEventEnvelope) => {
    setLastEvent(event);
    setEvents((prev) => [event, ...prev].slice(0, 100));
    onEventRef.current?.(event);
  }, []);

  useEffect(() => {
    if (!enabled) {
      return undefined;
    }
    if (!hasSession()) {
      return undefined;
    }

    const socket = io(`${WS_URL}/live`, {
      withCredentials: true,
      transports: ['websocket', 'polling'],
      reconnection: true,
      reconnectionDelay: 2000,
    });
    socketRef.current = socket;

    socket.on('connect', () => setConnected(true));
    socket.on('disconnect', () => setConnected(false));
    socket.on('live_event', (event: LiveEventEnvelope) => handleEvent(event));

    return () => {
      socket.disconnect();
      socketRef.current = null;
      setConnected(false);
    };
  }, [enabled, handleEvent]);

  return { connected, events, lastEvent };
}
