#!/usr/bin/env node
/**
 * WebSocket smoke test: login → connect /live → receive ≥3 events in 15s (MOCK_MODE).
 */
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(
  join(dirname(fileURLToPath(import.meta.url)), '..', 'services', 'api', 'package.json'),
);
const { io } = require('socket.io-client');

const API_BASE = process.env.API_BASE ?? 'http://localhost:3000';
const WS_BASE = process.env.WS_BASE ?? API_BASE;
const TIMEOUT_MS = Number(process.env.WS_SMOKE_TIMEOUT_MS ?? 15000);
const MIN_EVENTS = Number(process.env.WS_SMOKE_MIN_EVENTS ?? 3);

async function login() {
  const res = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: 'admin@calyx.ai',
      password: 'calyx_dev',
    }),
  });
  if (!res.ok) {
    throw new Error(`login failed: ${res.status}`);
  }
  const body = await res.json();
  return body.access_token;
}

async function main() {
  const token = await login();

  let count = 0;
  const types = new Set();

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      socket.disconnect();
      if (count >= MIN_EVENTS) {
        resolve(undefined);
      } else {
        reject(new Error(`only ${count} events in ${TIMEOUT_MS}ms (need ${MIN_EVENTS})`));
      }
    }, TIMEOUT_MS);

    const socket = io(`${WS_BASE}/live`, {
      auth: { token },
      transports: ['websocket', 'polling'],
    });

    socket.on('connect_error', (err) => {
      clearTimeout(timer);
      reject(err);
    });

    socket.on('live_event', (event) => {
      count += 1;
      types.add(event.type);
      process.stdout.write(`  event #${count}: ${event.type}\n`);
      if (count >= MIN_EVENTS) {
        clearTimeout(timer);
        socket.disconnect();
        resolve(undefined);
      }
    });
  });

  console.log(`WS SMOKE PASS: ${count} events, types: ${[...types].join(', ')}`);
}

main().catch((err) => {
  console.error('WS SMOKE FAIL:', err.message ?? err);
  process.exit(1);
});
