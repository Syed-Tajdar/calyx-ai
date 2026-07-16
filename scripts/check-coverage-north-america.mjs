import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import fs from 'node:fs';

const require = createRequire(import.meta.url);
const { chromium } = require(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'web', 'node_modules', 'playwright'));

const baseUrl = process.env.CALYX_WEB_URL ?? 'http://localhost:3001';
const outDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'mockups');
fs.mkdirSync(outDir, { recursive: true });
const screenshotPath = path.join(outDir, 'coverage-north-america.png');

const browser = await chromium.launch({ channel: 'msedge' });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

try {
  await page.goto(`${baseUrl}/login`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.fill('input[type="email"]', 'admin@calyx.ai');
  await page.fill('input[type="password"]', 'calyx_dev');
  await page.click('button[type="submit"]');
  await page.waitForURL('**/dashboard', { timeout: 30000 });

  const apiRes = await page.evaluate(async () => {
    const keys = ['calyx_token', 'token', 'access_token'];
    let t = '';
    for (const k of keys) {
      const v = localStorage.getItem(k);
      if (v) {
        t = v;
        break;
      }
    }
    const [coverage, pings] = await Promise.all([
      fetch('http://localhost:3000/coverage/heatmap', {
        headers: { Authorization: `Bearer ${t}` },
      }).then((r) => r.json()),
      fetch('http://localhost:3000/pings?limit=1', {
        headers: { Authorization: `Bearer ${t}` },
      }).then((r) => r.json()),
    ]);
    return { coverage, pings };
  });

  const codes = (apiRes.coverage.data ?? []).map((row) => row.state);
  const caRow = (apiRes.coverage.data ?? []).find((r) => r.state === 'ON' || r.country === 'CA');
  const caCalifornia = (apiRes.coverage.data ?? []).find((r) => r.state === 'CA');
  console.log('API region count:', codes.length);
  console.log('us_regions:', apiRes.coverage.us_regions, 'ca_regions:', apiRes.coverage.ca_regions);
  console.log('total_pings:', apiRes.coverage.total_pings);
  console.log('pings.distinct_total:', apiRes.pings.distinct_total, 'pings.total(raw):', apiRes.pings.total);
  console.log('CA (California) country:', caCalifornia?.country, 'pings:', caCalifornia?.ping_count);
  console.log('Sample CA-province:', caRow?.state, caRow?.country, caRow?.ping_count);

  await page.goto(`${baseUrl}/coverage`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForSelector('text=Coverage Intelligence', { timeout: 15000 });
  await page.waitForSelector('.calyx-coverage-map', { timeout: 20000 });

  const summary = await page.locator('p.text-sm.text-calyx-muted').first().textContent();
  const split = await page.locator('text=/\\d+ US · \\d+ CA/').first().textContent();
  console.log('Summary:', summary?.trim());
  console.log('Split:', split?.trim());

  const geoCount = await page.locator('.calyx-coverage-map path').count();
  console.log('Map SVG paths:', geoCount);

  // Screenshot first (capture US + Canada even if a province is partially off-frame)
  await page.screenshot({ path: screenshotPath, fullPage: false });
  console.log('Screenshot:', screenshotPath);

  const onGeo = page.locator('[aria-label*="Ontario"]').first();
  if (await onGeo.count()) {
    await onGeo.click({ force: true }).catch(() => {});
    await page.waitForTimeout(400);
    const panel = await page.locator('text=Region detail').first().isVisible().catch(() => false);
    const canadaInPanel = await page.getByText('Canada', { exact: false }).first().isVisible().catch(() => false);
    console.log('Ontario panel:', panel, 'Canada label:', canadaInPanel);
  }

  const caliGeo = page.locator('[aria-label*="California"]').first();
  if (await caliGeo.count()) {
    await caliGeo.hover({ force: true });
    await page.waitForTimeout(400);
    const tip = await page.locator('[role="tooltip"]').textContent().catch(() => '');
    console.log('California tooltip:', tip?.replace(/\s+/g, ' ').trim());
  }

  const headerOk =
    (summary ?? '').includes('US + Canada') &&
    !(summary ?? '').toLowerCase().includes('56 states');
  const mapOk = geoCount > 50; // US states + CA provinces
  const distinctOk =
    typeof apiRes.pings.distinct_total === 'number' &&
    apiRes.pings.distinct_total > 0 &&
    apiRes.pings.distinct_total <= apiRes.pings.total;
  const caCollisionOk = !caCalifornia || caCalifornia.country === 'US';

  const pass = headerOk && mapOk && distinctOk && caCollisionOk;
  console.log('MAP_CHECK:', pass ? 'PASS' : 'FAIL', { headerOk, mapOk, distinctOk, caCollisionOk });
  process.exit(pass ? 0 : 1);
} catch (err) {
  console.error(err);
  await page.screenshot({ path: path.join(outDir, 'coverage-north-america-error.png') }).catch(() => {});
  process.exit(1);
} finally {
  await browser.close();
}
