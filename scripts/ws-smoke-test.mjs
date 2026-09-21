#!/usr/bin/env node
/**
 * WebSocket smoke test: login → connect /live → receive ≥3 events.
 * With MOCK_EMITTER_ENABLED=false, seeds Ringba webhooks so Kafka → live consumer
 * produces ping_received (and possibly call_scored) events.
 */
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';

const require = createRequire(
  join(dirname(fileURLToPath(import.meta.url)), '..', 'services', 'api', 'package.json'),
);
const { io } = require('socket.io-client');

const API_BASE = process.env.API_BASE ?? 'http://localhost:3000';
const INGESTION_BASE = process.env.INGESTION_BASE ?? 'http://localhost:8080';
const WS_BASE = process.env.WS_BASE ?? API_BASE;
const TIMEOUT_MS = Number(process.env.WS_SMOKE_TIMEOUT_MS ?? 45000);
const MIN_EVENTS = Number(process.env.WS_SMOKE_MIN_EVENTS ?? 3);
const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const ingestionDir = join(repoRoot, 'services', 'ingestion');

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

function signRingba(query) {
  return execFileSync('go', ['run', './cmd/platformsig', 'ringba', query], {
    cwd: ingestionDir,
    encoding: 'utf8',
  }).trim();
}

async function seedPing() {
  const callId = `smoke-${randomUUID()}`;
  const query = `CallID=${callId}&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50`;
  const signature = signRingba(query);
  const url = `${INGESTION_BASE}/webhooks/ringba/dev_ringba_token_change_me?${query}`;
  const res = await fetch(url, { headers: { 'X-Ringba-Signature': signature } });
  if (!res.ok) {
    throw new Error(`seed ping failed: ${res.status}`);
  }
  process.stdout.write(`  seeded webhook call_id=${callId}\n`);
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

    socket.on('connect', () => {
      // Seed three pings so Kafka live consumer has traffic without mock emitter.
      void (async () => {
        try {
          for (let i = 0; i < MIN_EVENTS; i += 1) {
            await seedPing();
            await new Promise((r) => setTimeout(r, 800));
          }
        } catch (err) {
          clearTimeout(timer);
          socket.disconnect();
          reject(err instanceof Error ? err : new Error(String(err)));
        }
      })();
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
