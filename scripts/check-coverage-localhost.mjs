import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'path';

const require = createRequire(import.meta.url);
const { chromium } = require(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'web', 'node_modules', 'playwright'));

const baseUrl = process.env.CALYX_WEB_URL ?? 'http://localhost:3001';

const browser = await chromium.launch({ channel: 'msedge' });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
const pageErrors = [];
page.on('pageerror', (err) => pageErrors.push(err.message));
page.on('console', (msg) => {
  if (msg.type() === 'error') pageErrors.push(msg.text());
});

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
    const r = await fetch('http://localhost:3000/coverage/heatmap', {
      headers: { Authorization: `Bearer ${t}` },
    });
    return r.json();
  });

  const rows = apiRes.data ?? [];
  const codes = rows.map((row) => row.state);
  console.log('API state count:', codes.length);
  console.log('API total pings:', apiRes.total_pings ?? 0);

  await page.goto(`${baseUrl}/coverage`, { waitUntil: 'domcontentloaded', timeout: 30000 });
  await page.waitForTimeout(3000);

  const runtimeError = await page.locator('nextjs-portal, [data-nextjs-dialog]').first().textContent().catch(() => '');
  if (runtimeError) {
    console.log('Runtime error:', runtimeError.replace(/\s+/g, ' ').trim().slice(0, 500));
  }
  if (pageErrors.length) {
    console.log('Page errors:', pageErrors.join(' | ').slice(0, 800));
  }

  const title = await page.locator('h1').first().textContent().catch(() => '');
  console.log('Page title:', title?.trim());

  const emptyState = await page.locator('text=No geographic data yet').count();
  if (emptyState > 0) {
    console.log('MAP_VISIBLE: false (no geographic data)');
    console.log('LOCALHOST_CHECK: FAIL — map hidden without API data');
    process.exit(1);
  }

  await page.waitForSelector('.calyx-coverage-map', { timeout: 30000 });
  await page.waitForSelector('.calyx-coverage-map-label', { timeout: 30000 });
  await page.waitForTimeout(1500);

  const map = await page.evaluate(() => {
    const paths = document.querySelectorAll('.calyx-coverage-map path').length;
    const labels = [...document.querySelectorAll('.calyx-coverage-map-label')].map((el) =>
      el.textContent?.trim() ?? '',
    );
    const unique = [...new Set(labels.filter(Boolean))].sort();
    let keyframeUsesTeal = false;
    for (const sheet of document.styleSheets) {
      try {
        for (const rule of sheet.cssRules) {
          if (rule.type === CSSRule.KEYFRAMES_RULE && rule.name === 'calyx-geo-stroke-pulse') {
            keyframeUsesTeal = rule.cssText.includes('201, 183');
          }
        }
      } catch {
        // ignore cross-origin sheets
      }
    }
    const pulsePaths = [...document.querySelectorAll('.calyx-coverage-map path')].filter((p) =>
      (p.getAttribute('style') || '').includes('calyx-geo-stroke-pulse'),
    ).length;
    return { paths, labelCount: labels.length, unique, pulsePaths, keyframeUsesTeal };
  });

  console.log('Map SVG paths:', map.paths);
  console.log('State labels:', map.labelCount);
  console.log('Unique codes:', map.unique.length);
  console.log('Sample labels:', map.unique.slice(0, 15).join(', '));
  console.log('Has TX:', map.unique.includes('TX'));
  console.log('Has CA:', map.unique.includes('CA'));
  console.log('Has NY:', map.unique.includes('NY'));
  console.log('Geo pulse paths:', map.pulsePaths);
  console.log('Pulse uses teal rgba(0,201,183):', map.keyframeUsesTeal);

  const pass =
    map.paths > 40 &&
    map.labelCount >= 50 &&
    map.unique.includes('TX') &&
    map.unique.includes('CA') &&
    map.pulsePaths > 40 &&
    map.keyframeUsesTeal;

  console.log('LOCALHOST_CHECK:', pass ? 'PASS' : 'FAIL');
  process.exit(pass ? 0 : 1);
} catch (err) {
  if (pageErrors.length) {
    console.log('Page errors:', pageErrors.join(' | ').slice(0, 800));
  }
  console.error(err);
  process.exit(1);
} finally {
  await browser.close();
}
