#!/usr/bin/env node
/**
 * P1 — Prove /live rejects a handshake Origin not in WEB_ORIGINS.
 */
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(
  join(dirname(fileURLToPath(import.meta.url)), '..', 'services', 'api', 'package.json'),
);
const { io } = require('socket.io-client');

const API = process.env.API_BASE ?? 'http://127.0.0.1:3100';
const BAD_ORIGIN = process.env.BAD_ORIGIN ?? 'http://evil.example:3999';

const login = await fetch(`${API}/auth/login`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Calyx-Client': 'script',
  },
  body: JSON.stringify({ email: 'admin@calyx.ai', password: 'calyx_dev' }),
});
if (!login.ok) {
  console.error('login failed', login.status);
  process.exit(1);
}
const { access_token: token } = await login.json();

await new Promise((resolve, reject) => {
  const timer = setTimeout(() => {
    socket.disconnect();
    reject(new Error('timeout — bad origin was not rejected'));
  }, 10000);

  const socket = io(`${API}/live`, {
    auth: { token },
    transports: ['websocket'],
    reconnection: false,
    extraHeaders: { Origin: BAD_ORIGIN },
  });

  socket.on('connect', () => {
    clearTimeout(timer);
    socket.disconnect();
    reject(new Error(`UNEXPECTED connect with Origin=${BAD_ORIGIN}`));
  });

  socket.on('connect_error', (err) => {
    clearTimeout(timer);
    socket.disconnect();
    console.log(`PASS: rejected Origin=${BAD_ORIGIN} err=${err.message}`);
    resolve(undefined);
  });

  socket.on('disconnect', (reason) => {
    // server may accept then disconnect in handleConnection
    clearTimeout(timer);
    console.log(`PASS: disconnected Origin=${BAD_ORIGIN} reason=${reason}`);
    resolve(undefined);
  });
}).catch((e) => {
  console.error('FAIL:', e.message);
  process.exit(1);
});

// Control: allowed origin + token must still connect
await new Promise((resolve, reject) => {
  const timer = setTimeout(() => {
    socket.disconnect();
    reject(new Error('timeout — allowed origin failed to connect'));
  }, 10000);
  const socket = io(`${API}/live`, {
    auth: { token },
    transports: ['websocket'],
    reconnection: false,
    extraHeaders: { Origin: 'http://localhost:3101' },
  });
  socket.on('connect', () => {
    clearTimeout(timer);
    socket.disconnect();
    console.log('PASS: allowed Origin=http://localhost:3101 connected');
    resolve(undefined);
  });
  socket.on('connect_error', (err) => {
    clearTimeout(timer);
    reject(new Error(`allowed origin connect_error: ${err.message}`));
  });
}).catch((e) => {
  console.error('FAIL:', e.message);
  process.exit(1);
});

console.log('P1 WebSocket origin check PASS');
