#!/usr/bin/env node
/**
 * WebSocket smoke test: login → connect /live → receive ≥3 ping_received
 * events whose call_id matches seeds sent this run.
 * With MOCK_EMITTER_ENABLED=false, seeds Ringba webhooks so Kafka → live consumer
 * produces ping_received.
 *
 * Signing is in-process HMAC-SHA256 (hex), matching services/ingestion/cmd/platformsig.
 */
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, execSync } from 'node:child_process';
import { createHmac, randomUUID } from 'node:crypto';

const require = createRequire(
  join(dirname(fileURLToPath(import.meta.url)), '..', 'services', 'api', 'package.json'),
);
const { io } = require('socket.io-client');

const API_BASE = process.env.API_BASE ?? 'http://localhost:3100';
const INGESTION_BASE = process.env.INGESTION_BASE ?? 'http://localhost:8080';
const WS_BASE = process.env.WS_BASE ?? API_BASE;
const TIMEOUT_MS = Number(process.env.WS_SMOKE_TIMEOUT_MS ?? 45000);
const MIN_PING_RECEIVED = Number(process.env.WS_SMOKE_MIN_EVENTS ?? 3);
const GROUP_ID = 'calyx-api-live-feed';
const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const ingestionDir = join(repoRoot, 'services', 'ingestion');

/** Defaults match platformsig envOrDefault. */
const PLATFORM_SECRETS = {
  ringba: process.env.RINGBA_WEBHOOK_SECRET || 'dev_ringba_hmac_secret_change_me',
  retreaver: process.env.RETREAVER_WEBHOOK_SECRET || 'dev_retreaver_hmac_secret_change_me',
  callgrid: process.env.CALLGRID_WEBHOOK_SECRET || 'dev_callgrid_hmac_secret_change_me',
};

function ts() {
  return new Date().toISOString();
}

function log(msg) {
  process.stdout.write(`[${ts()}] ${msg}\n`);
}

/**
 * HMAC-SHA256 hex of rawQuery (or body), same as security.Compute*HMAC with nil body.
 * @param {'ringba'|'retreaver'|'callgrid'} platform
 * @param {string} rawQuery
 */
function signPlatform(platform, rawQuery) {
  const secret = PLATFORM_SECRETS[platform];
  if (!secret) {
    throw new Error(`unsupported platform for signing: ${platform}`);
  }
  return createHmac('sha256', secret).update(rawQuery, 'utf8').digest('hex');
}

function signRingba(query) {
  return signPlatform('ringba', query);
}

/** One-shot: Node HMAC must equal `go run ./cmd/platformsig` for a fixed query. */
function selfCheckVsPlatformsig() {
  const fixedQuery =
    'CallID=smoke-sigcheck-fixed&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50';
  const nodeSig = signRingba(fixedQuery);
  const goSig = execFileSync('go', ['run', './cmd/platformsig', 'ringba', fixedQuery], {
    cwd: ingestionDir,
    encoding: 'utf8',
  }).trim();
  if (nodeSig !== goSig) {
    throw new Error(
      `HMAC self-check FAIL: node=${nodeSig} platformsig=${goSig} query=${fixedQuery}`,
    );
  }
  log(`HMAC self-check PASS: node === platformsig (${nodeSig.slice(0, 12)}…)`);
}

function describeLiveFeedGroup() {
  try {
    const out = execSync(
      `docker exec deploy-redpanda-1 rpk group describe ${GROUP_ID}`,
      { encoding: 'utf8', timeout: 15000 },
    );
    const state = (out.match(/^STATE\s+(\S+)/m) || [])[1] ?? '?';
    const members = (out.match(/^MEMBERS\s+(\S+)/m) || [])[1] ?? '?';
    const lag = (out.match(/^TOTAL-LAG\s+(\S+)/m) || [])[1] ?? '?';
    return { raw: out, state, members, lag, ok: true };
  } catch (err) {
    return {
      raw: String(err?.stderr ?? err?.message ?? err),
      state: 'error',
      members: 'error',
      lag: 'error',
      ok: false,
    };
  }
}

function printGroup(label, g) {
  log(`group ${label}: STATE=${g.state} MEMBERS=${g.members} TOTAL-LAG=${g.lag}`);
}

function formatFetchError(err) {
  const msg = err?.message ?? String(err);
  const cause = err?.cause;
  if (cause == null) {
    return msg;
  }
  const causeMsg =
    cause instanceof Error
      ? `${cause.name}: ${cause.message}`
      : typeof cause === 'object'
        ? JSON.stringify(cause)
        : String(cause);
  return `${msg} cause=${causeMsg}`;
}

async function login() {
  let res;
  try {
    res = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Calyx-Client': 'script',
      },
      body: JSON.stringify({
        email: 'admin@calyx.ai',
        password: 'calyx_dev',
      }),
    });
  } catch (err) {
    throw new Error(`login fetch failed: ${formatFetchError(err)}`);
  }
  if (!res.ok) {
    throw new Error(`login failed: ${res.status}`);
  }
  const body = await res.json();
  if (!body.access_token) {
    throw new Error('login missing access_token (need X-Calyx-Client: script)');
  }
  return body.access_token;
}

/**
 * @param {(callId: string) => void} [onCallIdReady] Invoked before HTTP so live_event
 *   handlers can match ping_received that arrive while the request is in flight.
 * @returns {{ callId: string, httpStatus: number, signMs: number, httpMs: number, totalMs: number }}
 */
async function seedPing(onCallIdReady) {
  const callId = `smoke-${randomUUID()}`;
  onCallIdReady?.(callId);
  const query = `CallID=${callId}&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50`;
  const signStart = Date.now();
  const signature = signRingba(query);
  const signMs = Date.now() - signStart;
  const url = `${INGESTION_BASE}/webhooks/ringba/dev_ringba_token_change_me?${query}`;
  const httpStart = Date.now();
  let res;
  try {
    res = await fetch(url, { headers: { 'X-Ringba-Signature': signature } });
  } catch (err) {
    throw new Error(`seed fetch failed: ${formatFetchError(err)} call_id=${callId}`);
  }
  const httpMs = Date.now() - httpStart;
  log(
    `seed call_id=${callId} http=${res.status} sign_ms=${signMs} http_ms=${httpMs} total_ms=${signMs + httpMs}`,
  );
  if (!res.ok) {
    throw new Error(`seed ping failed: ${res.status} call_id=${callId}`);
  }
  return { callId, httpStatus: res.status, signMs, httpMs, totalMs: signMs + httpMs };
}

function summarize(seededIds, receivedIds, pendingSeed) {
  const sent = [...seededIds];
  const notSent = pendingSeed ? [pendingSeed] : [];
  const received = [...receivedIds];
  const notReceived = sent.filter((id) => !receivedIds.has(id));
  log(`exit summary: seeds_sent=${sent.length} [${sent.join(', ') || '-'}]`);
  log(`exit summary: seeds_not_sent=${notSent.length} [${notSent.join(', ') || '-'}]`);
  log(`exit summary: ping_received_matched=${received.length} [${received.join(', ') || '-'}]`);
  log(`exit summary: seeds_not_received=${notReceived.length} [${notReceived.join(', ') || '-'}]`);
}

async function main() {
  log('ws-smoke start');
  selfCheckVsPlatformsig();
  const groupStart = describeLiveFeedGroup();
  printGroup('start', groupStart);

  const token = await login();
  log('login ok');

  /** @type {string[]} */
  const seededIds = [];
  /** @type {Set<string>} */
  const receivedIds = new Set();
  /** @type {string | null} */
  let pendingSeed = null;
  /** @type {Set<string>} */
  const otherTypes = new Set();
  let matchedCount = 0;

  try {
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        socket.disconnect();
        reject(
          new Error(
            `only ${matchedCount} matching ping_received in ${TIMEOUT_MS}ms (need ${MIN_PING_RECEIVED}); ` +
              `seeds_sent=${seededIds.length} received=[${[...receivedIds].join(',')}]`,
          ),
        );
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
        log(`socket connect id=${socket.id}`);
        void (async () => {
          try {
            for (let i = 0; i < MIN_PING_RECEIVED; i += 1) {
              pendingSeed = `(seed ${i + 1}/${MIN_PING_RECEIVED} in progress)`;
              // Register call_id before fetch: with in-process HMAC, ping_received can
              // arrive before the HTTP promise resolves (race exposed when sign_ms≈0).
              await seedPing((callId) => {
                seededIds.push(callId);
                pendingSeed = null;
              });
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
        const callId =
          typeof event?.payload?.call_id === 'string' ? event.payload.call_id : '';
        log(`event type=${event?.type ?? '?'} call_id=${callId || '-'}`);
        if (event?.type !== 'ping_received') {
          otherTypes.add(String(event?.type ?? '?'));
          return;
        }
        if (!callId || !seededIds.includes(callId)) {
          log(`event ignored (ping_received but call_id not in this run's seeds)`);
          return;
        }
        if (receivedIds.has(callId)) {
          return;
        }
        receivedIds.add(callId);
        matchedCount = receivedIds.size;
        if (matchedCount >= MIN_PING_RECEIVED) {
          clearTimeout(timer);
          socket.disconnect();
          resolve(undefined);
        }
      });
    });
  } catch (err) {
    const groupFail = describeLiveFeedGroup();
    printGroup('failure', groupFail);
    summarize(seededIds, receivedIds, pendingSeed);
    throw err;
  }

  summarize(seededIds, receivedIds, pendingSeed);
  const extra =
    otherTypes.size > 0 ? `; other_types_logged=${[...otherTypes].join(',')}` : '';
  console.log(
    `WS SMOKE PASS: ${matchedCount} matching ping_received (seeds=${seededIds.length})${extra}`,
  );
}

main().catch((err) => {
  console.error('WS SMOKE FAIL:', formatFetchError(err));
  process.exit(1);
});
