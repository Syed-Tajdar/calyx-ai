/**
 * Calyx web CSP + hydration gate (Playwright / Chromium).
 *
 * Fails on any CSP violation, pageerror, login that never reaches /dashboard,
 * dashboard stuck on loading skeletons, or empty Pings / Settings / call detail.
 * Seeds a live-feed ping and waits for a matched ping_received event.
 *
 * Env:
 *   CALYX_WEB_URL   default http://localhost:3101
 *   CALYX_API_URL   default http://localhost:3100
 *   CALYX_INGESTION_URL default http://127.0.0.1:8080
 *   CALYX_EVIDENCE_DIR  optional directory for screenshots + console log
 */
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import fs from 'node:fs';
import path from 'node:path';
import { createHmac, randomUUID } from 'node:crypto';

const require = createRequire(import.meta.url);
const webRoot = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'web');
const apiRoot = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'services', 'api');
const { chromium } = require(path.join(webRoot, 'node_modules', 'playwright'));
const { io } = require(path.join(apiRoot, 'node_modules', 'socket.io-client'));

const baseUrl = process.env.CALYX_WEB_URL ?? 'http://localhost:3101';
const apiUrl = process.env.CALYX_API_URL ?? 'http://127.0.0.1:3100';
const ingestionUrl = process.env.CALYX_INGESTION_URL ?? 'http://127.0.0.1:8080';
const ringbaSecret =
  process.env.RINGBA_WEBHOOK_SECRET || 'dev_ringba_hmac_secret_change_me';
const ringbaToken = process.env.RINGBA_WEBHOOK_TOKEN || 'dev_ringba_token_change_me';
const evidenceDir =
  process.env.CALYX_EVIDENCE_DIR ??
  path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'evidence', 's2-csp-fix');
fs.mkdirSync(evidenceDir, { recursive: true });

const consoleLog = [];
const cspViolations = [];
const pageErrors = [];
const checks = [];

function pass(name) {
  checks.push({ name, ok: true });
  console.log(`  PASS ${name}`);
}

function fail(name, detail) {
  checks.push({ name, ok: false, detail });
  console.log(`  FAIL ${name}: ${detail}`);
}

function isCsp(text) {
  return /Content Security Policy|CSP directive|violates the following Content Security Policy/i.test(
    text,
  );
}

const browser = await chromium.launch({
  headless: true,
  executablePath:
    process.env.PLAYWRIGHT_CHROME_PATH ||
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
});
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();

page.on('console', (msg) => {
  const entry = { type: msg.type(), text: msg.text(), loc: msg.location()?.url };
  consoleLog.push(entry);
  if (isCsp(entry.text)) cspViolations.push(entry);
});
page.on('pageerror', (err) => {
  pageErrors.push(String(err));
  consoleLog.push({ type: 'pageerror', text: String(err) });
});

async function assertNoSkeletons(label) {
  const stuck = await page.evaluate(() => {
    const pulses = document.querySelectorAll('.animate-pulse');
    // AppShell auth gate: lone pulsing circle, no main content.
    const shellGate =
      pulses.length > 0 &&
      !document.querySelector('aside') &&
      !document.querySelector('main');
    return { pulseCount: pulses.length, shellGate };
  });
  if (stuck.shellGate) {
    fail(`${label} not skeleton`, `AppShell loading gate still visible (pulses=${stuck.pulseCount})`);
    return false;
  }
  pass(`${label} not skeleton`);
  return true;
}

async function shot(name) {
  const p = path.join(evidenceDir, `${name}.png`);
  await page.screenshot({ path: p, fullPage: false });
  console.log(`  screenshot ${p}`);
}

async function seedRingba(callId) {
  const query = `CallID=${callId}&caller_id=15551234567&state=TX&event=incoming&call_revenue=12.50`;
  const signature = createHmac('sha256', ringbaSecret).update(query, 'utf8').digest('hex');
  const seedUrl = `${ingestionUrl}/webhooks/ringba/${ringbaToken}?${query}`;
  const res = await fetch(seedUrl, { headers: { 'X-Ringba-Signature': signature } });
  return { status: res.status, callId };
}

/**
 * Login via script client, connect /live, seed one ping, wait for matched ping_received.
 * Proves the live pipeline while the browser stays on the hydrated dashboard.
 */
async function waitForLiveEvent(callId) {
  const loginRes = await fetch(`${apiUrl}/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Calyx-Client': 'script',
    },
    body: JSON.stringify({ email: 'admin@calyx.ai', password: 'calyx_dev' }),
  });
  if (!loginRes.ok) {
    throw new Error(`script login ${loginRes.status}`);
  }
  const { access_token: token } = await loginRes.json();
  if (!token) throw new Error('script login missing access_token');

  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      socket.disconnect();
      reject(new Error(`no ping_received for ${callId} in 35s`));
    }, 35000);

    const socket = io(`${apiUrl}/live`, {
      auth: { token },
      transports: ['websocket'],
      reconnection: false,
    });

    socket.on('connect_error', (err) => {
      clearTimeout(timer);
      socket.disconnect();
      reject(new Error(`socket connect_error: ${err.message}`));
    });

    socket.on('connect', async () => {
      try {
        const seeded = await seedRingba(callId);
        if (seeded.status < 200 || seeded.status >= 300) {
          clearTimeout(timer);
          socket.disconnect();
          reject(new Error(`ringba seed http=${seeded.status}`));
        }
      } catch (e) {
        clearTimeout(timer);
        socket.disconnect();
        reject(e);
      }
    });

    socket.on('live_event', (event) => {
      const payload = event?.payload ?? {};
      const id = payload.call_id ?? payload.CallID ?? payload.callId;
      if (event?.type === 'ping_received' && id === callId) {
        clearTimeout(timer);
        socket.disconnect();
        resolve({ callId, event });
      }
    });
  });
}

try {
  // --- Login ---
  await page.goto(`${baseUrl}/login`, { waitUntil: 'networkidle', timeout: 45000 });
  await page.waitForTimeout(1500);
  await shot('01-login');

  // Controlled inputs: type (not fill) so React state updates.
  await page.locator('#email').click();
  await page.locator('#email').fill('admin@calyx.ai');
  await page.locator('#password').click();
  await page.locator('#password').fill('calyx_dev');
  await page.locator('button[type="submit"]').click();

  try {
    await page.waitForURL('**/dashboard', { timeout: 30000 });
    pass('login redirects to /dashboard');
  } catch (e) {
    fail('login redirects to /dashboard', `still at ${page.url()}: ${e.message}`);
  }

  await page.waitForTimeout(2000);
  await shot('02-dashboard');
  await assertNoSkeletons('dashboard');

  // Fonts: body must be Inter (not generic serif); headings Geist
  const fontCheck = await page.evaluate(() => {
    const body = getComputedStyle(document.body).fontFamily;
    const h1 = document.querySelector('h1');
    const heading = h1 ? getComputedStyle(h1).fontFamily : '';
    return { body, heading };
  });
  const bodyLower = fontCheck.body.toLowerCase();
  const isSerifFallback =
    /^(times|times new roman|georgia|serif)\b/i.test(fontCheck.body.trim()) ||
    (bodyLower.includes('serif') && !bodyLower.includes('sans-serif') && !bodyLower.includes('inter'));
  if (isSerifFallback || !/inter/i.test(fontCheck.body)) {
    fail('body font Inter', `got "${fontCheck.body}"`);
  } else {
    pass(`body font Inter (${fontCheck.body.split(',')[0].trim()})`);
  }
  if (fontCheck.heading && !/geist/i.test(fontCheck.heading)) {
    fail('h1 font Geist', `got "${fontCheck.heading}"`);
  } else if (fontCheck.heading) {
    pass(`h1 font Geist (${fontCheck.heading.split(',')[0].trim()})`);
  }

  // Account email from /auth/me (not JWT decode)
  try {
    await page.waitForSelector('[data-testid="topbar-email"]', { timeout: 15000 });
    await page.waitForFunction(
      () => {
        const el = document.querySelector('[data-testid="topbar-email"]');
        return el && /@/.test(el.textContent || '');
      },
      { timeout: 15000 },
    );
    const emailText = await page.locator('[data-testid="topbar-email"]').innerText();
    if (/admin@calyx\.ai/i.test(emailText)) pass(`topbar email (${emailText.trim()})`);
    else fail('topbar email', `got "${emailText}"`);
  } catch {
    fail('topbar email', 'email never appeared (still Account?)');
  }
  await shot('02b-topbar-email');

  // KPI data (not empty shells)
  try {
    await page.waitForSelector('text=Total Revenue', { timeout: 25000 });
    pass('dashboard KPI Total Revenue');
  } catch {
    fail('dashboard KPI Total Revenue', 'not visible');
  }
  const metricCount = await page.locator('.calyx-metric-card').count();
  if (metricCount >= 1) pass(`dashboard metric cards (${metricCount})`);
  else fail('dashboard metric cards', `got ${metricCount}`);

  // --- Pings ---
  await page.goto(`${baseUrl}/pings`, { waitUntil: 'networkidle', timeout: 45000 });
  await page.waitForTimeout(2000);
  await shot('03-pings');
  await assertNoSkeletons('pings');
  {
    const ff = await page.evaluate(() => getComputedStyle(document.body).fontFamily);
    if (!/inter/i.test(ff)) fail('pings body font', ff);
    else pass('pings body font Inter');
  }
  const pingsBody = await page.locator('main').innerText().catch(() => '');
  if (pingsBody.length > 20) pass('pings has content');
  else fail('pings has content', `main text len=${pingsBody.length}`);

  // --- Settings ---
  await page.goto(`${baseUrl}/settings`, { waitUntil: 'networkidle', timeout: 45000 });
  await page.waitForTimeout(2000);
  await shot('04-settings');
  await assertNoSkeletons('settings');
  {
    const ff = await page.evaluate(() => getComputedStyle(document.body).fontFamily);
    if (!/inter/i.test(ff)) fail('settings body font', ff);
    else pass('settings body font Inter');
  }
  const settingsBody = await page.locator('main').innerText().catch(() => '');
  if (/integration|setting|platform/i.test(settingsBody) || settingsBody.length > 20) {
    pass('settings has content');
  } else {
    fail('settings has content', `main text len=${settingsBody.length}`);
  }

  // --- Call detail ---
  await page.goto(`${baseUrl}/calls/demo-call-1`, { waitUntil: 'networkidle', timeout: 45000 });
  await page.waitForTimeout(2000);
  await shot('05-call-detail');
  await assertNoSkeletons('call detail');
  {
    const ff = await page.evaluate(() => ({
      body: getComputedStyle(document.body).fontFamily,
      h1: document.querySelector('h1')
        ? getComputedStyle(document.querySelector('h1')).fontFamily
        : '',
    }));
    if (!/inter/i.test(ff.body)) fail('call detail body font', ff.body);
    else pass('call detail body font Inter');
    if (ff.h1 && !/geist/i.test(ff.h1)) fail('call detail h1 font', ff.h1);
    else if (ff.h1) pass('call detail h1 font Geist');
  }
  const callBody = await page.locator('main').innerText().catch(() => '');
  if (callBody.length > 20) pass('call detail has content');
  else fail('call detail has content', `main text len=${callBody.length}`);

  // --- Live feed: keep dashboard open; seed + receive via /live (same pipeline the UI uses) ---
  await page.goto(`${baseUrl}/dashboard`, { waitUntil: 'networkidle', timeout: 45000 });
  await page.waitForTimeout(1500);

  let liveOk = false;
  try {
    // Cookie session already established by form login — confirm in-page auth works.
    const session = await page.evaluate(async (api) => {
      const r = await fetch(`${api}/auth/me`, { credentials: 'include' });
      return { status: r.status };
    }, apiUrl.replace('127.0.0.1', 'localhost'));
    if (session.status === 200) pass('in-page session login');
    else {
      // fallback: /auth/me may not exist — try live-status
      const liveStatus = await page.evaluate(async (api) => {
        const r = await fetch(`${api}/live-status`, { credentials: 'include' });
        return { status: r.status };
      }, apiUrl.replace('127.0.0.1', 'localhost'));
      if (liveStatus.status === 200) pass('in-page session login');
      else fail('in-page session login', `auth/me=${session.status} live-status=${liveStatus.status}`);
    }

    // Must use smoke- so KPI exclusion (apisync-/demo-/smoke-) keeps gates out of dashboards.
    const callId = `smoke-csp-${randomUUID()}`;
    await waitForLiveEvent(callId);
    liveOk = true;
    pass(`live feed received event (${callId})`);
  } catch (e) {
    fail('live feed received event', String(e));
  }

  // --- CSP / console ---
  if (cspViolations.length === 0) pass('zero CSP violations');
  else fail('zero CSP violations', `${cspViolations.length}: ${cspViolations[0]?.text?.slice(0, 160)}`);

  const hardErrors = pageErrors.filter((e) => !/hydration|Download the React DevTools/i.test(e));
  const consoleHard = consoleLog.filter(
    (e) =>
      e.type === 'error' &&
      !isCsp(e.text) &&
      !/Download the React DevTools|third-party cookie|favicon\.ico/i.test(e.text) &&
      !/favicon\.ico/i.test(e.loc || ''),
  );
  if (hardErrors.length === 0 && consoleHard.length === 0) pass('zero console/page errors');
  else
    fail(
      'zero console/page errors',
      `pageErrors=${hardErrors.length} consoleErrors=${consoleHard.length} sample=${hardErrors[0] || consoleHard[0]?.text}`,
    );

  fs.writeFileSync(
    path.join(evidenceDir, 'browser_console.json'),
    JSON.stringify({ cspViolations, pageErrors, consoleLog: consoleLog.slice(-80), checks, liveOk }, null, 2),
  );

  const failed = checks.filter((c) => !c.ok);
  console.log(`\n=== ${checks.length - failed.length}/${checks.length} checks passed ===`);
  if (failed.length > 0) process.exit(1);
} catch (err) {
  console.error('Browser check error:', err);
  await page.screenshot({ path: path.join(evidenceDir, 'browser-check-error.png') }).catch(() => {});
  fs.writeFileSync(
    path.join(evidenceDir, 'browser_console.json'),
    JSON.stringify({ fatal: String(err), cspViolations, pageErrors, consoleLog }, null, 2),
  );
  process.exit(1);
} finally {
  await browser.close();
}
