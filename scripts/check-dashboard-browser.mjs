import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const require = createRequire(import.meta.url);
const { chromium } = require(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'web', 'node_modules', 'playwright'));

const baseUrl = process.env.CALYX_WEB_URL ?? 'http://localhost:3001';
const outDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'mockups');
const screenshotPath = path.join(outDir, 'dashboard-browser-check.png');

const browser = await chromium.launch({ channel: 'msedge' });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

const checks = [];

function pass(name) {
  checks.push({ name, ok: true });
  console.log(`  PASS ${name}`);
}

function fail(name, detail) {
  checks.push({ name, ok: false, detail });
  console.log(`  FAIL ${name}: ${detail}`);
}

try {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.fill('input[type="email"]', 'admin@calyx.ai');
  await page.fill('input[type="password"]', 'calyx_dev');
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard', { timeout: 30000 });

  pass('login redirects to /dashboard');

  await page.waitForSelector('text=Operations Overview', { timeout: 15000 });
  pass('dashboard page title visible');

  // Wait for data load (loader gone + metrics rendered)
  await page.waitForSelector('text=Total Revenue', { timeout: 30000 });
  await page.waitForSelector('.calyx-metric-card', { timeout: 15000 });

  const sidebar = page.locator('aside').first();
  if (await sidebar.isVisible()) {
    pass('sidebar visible');
  } else {
    fail('sidebar visible', 'aside not found');
  }

  const operationsGroup = page.locator('text=Operations').first();
  if (await operationsGroup.isVisible()) {
    pass('nav group Operations');
  } else {
    fail('nav group Operations', 'not visible');
  }

  const pingsNav = page.locator('aside a[href="/pings"]').first();
  if (await pingsNav.isVisible()) {
    pass('Pings nav link');
  } else {
    fail('Pings nav link', 'not visible');
  }

  const metricCards = page.locator('.calyx-metric-card');
  const metricCount = await metricCards.count();
  if (metricCount >= 4) {
    pass(`metric cards (${metricCount})`);
  } else {
    fail('metric cards', `expected >=4, got ${metricCount}`);
  }

  const pulseIcons = page.locator('.calyx-metric-icon-pulse');
  const pulseCount = await pulseIcons.count();
  if (pulseCount >= 3) {
    pass(`metric icon pulse (${pulseCount})`);
  } else {
    fail('metric icon pulse', `expected >=3, got ${pulseCount}`);
  }

  const mockPill = page.locator('aside').getByText('Mock', { exact: true }).first();
  if (await mockPill.isVisible()) {
    pass('sidebar Mock footer pill');
  } else {
    fail('sidebar Mock footer pill', 'not visible');
  }

  const chart = page.locator('text=Inbound Ping Overview').first();
  if (await chart.isVisible()) {
    pass('ping volume chart');
  } else {
    fail('ping volume chart', 'not visible');
  }

  const errorBanner = page.locator('text=Failed to load dashboard data');
  if (await errorBanner.count() === 0) {
    pass('no dashboard error banner');
  } else {
    fail('no dashboard error banner', 'error banner shown');
  }

  await page.screenshot({ path: screenshotPath, fullPage: false });
  console.log(`\nScreenshot: ${screenshotPath}`);

  const failed = checks.filter((c) => !c.ok);
  console.log(`\n=== ${checks.length - failed.length}/${checks.length} checks passed ===`);
  if (failed.length > 0) process.exit(1);
} catch (err) {
  console.error('Browser check error:', err);
  await page.screenshot({ path: path.join(outDir, 'dashboard-browser-check-error.png') }).catch(() => {});
  process.exit(1);
} finally {
  await browser.close();
}
