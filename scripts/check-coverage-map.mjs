import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const require = createRequire(import.meta.url);
const { chromium } = require(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'web', 'node_modules', 'playwright'));

const baseUrl = process.env.CALYX_WEB_URL ?? 'http://localhost:3101';

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
    const r = await fetch('http://localhost:3100/coverage/heatmap', {
      headers: { Authorization: `Bearer ${t}` },
    });
    return r.json();
  });

  const codes = (apiRes.data ?? []).map((row) => row.state);
  console.log('API states include WA:', codes.includes('WA'));
  console.log('API states include CO:', codes.includes('CO'));
  console.log('API states include WY:', codes.includes('WY'));
  console.log('API state count:', codes.length);

  await page.goto(`${baseUrl}/coverage`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForSelector('text=Coverage Intelligence', { timeout: 15000 });

  const summary = await page.locator('p.text-sm.text-calyx-muted').first().textContent();
  console.log('Summary:', summary?.trim());

  await page.waitForSelector('.calyx-coverage-map', { timeout: 20000 });
  const geoCount = await page.locator('.calyx-coverage-map path').count();
  console.log('Map SVG paths:', geoCount);

  const waRow = (apiRes.data ?? []).find((r) => r.state === 'WA');
  const coRow = (apiRes.data ?? []).find((r) => r.state === 'CO');
  const wyRow = (apiRes.data ?? []).find((r) => r.state === 'WY');
  console.log('WA pings:', waRow?.ping_count ?? 0);
  console.log('CO pings:', coRow?.ping_count ?? 0);
  console.log('WY pings:', wyRow?.ping_count ?? 0);

  const waGeo = page.locator('[aria-label*="Washington"]').first();
  if (await waGeo.count()) {
    await waGeo.hover({ force: true });
    await page.waitForTimeout(500);
    const tip = await page.locator('[role="tooltip"]').textContent().catch(() => '');
    console.log('WA tooltip:', tip?.replace(/\s+/g, ' ').trim());
  }

  const mapOk =
    codes.includes('WA') &&
    codes.includes('CO') &&
    codes.includes('WY') &&
    geoCount > 40;
  console.log('MAP_CHECK:', mapOk ? 'PASS' : 'FAIL');
  process.exit(mapOk ? 0 : 1);
} catch (err) {
  console.error(err);
  process.exit(1);
} finally {
  await browser.close();
}
