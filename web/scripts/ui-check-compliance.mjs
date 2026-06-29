import { chromium } from 'playwright';

const BASE = process.env.UI_BASE_URL ?? 'http://localhost:3003';

async function main() {
  const browser = await chromium.launch({
    headless: true,
    channel: 'chrome',
  });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });

  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.getByLabel('Email', { exact: true }).fill('admin@calyx.ai');
  await page.getByLabel('Password', { exact: true }).fill('calyx_dev');
  await page.getByRole('button', { name: /sign in|log in|login/i }).click();
  await page.waitForURL(/\/dashboard/, { timeout: 20000 });

  await page.goto(`${BASE}/dashboard`, { waitUntil: 'networkidle' });
  const dashRing = page
    .locator('.relative')
    .filter({ has: page.locator('svg circle') })
    .first()
    .locator('span.tabular-nums')
    .first();
  await dashRing.waitFor({ state: 'visible', timeout: 15000 });
  const dashScore = (await dashRing.textContent())?.trim() ?? '';
  console.log('Dashboard ring score:', JSON.stringify(dashScore));

  await page.goto(`${BASE}/compliance`, { waitUntil: 'networkidle' });
  await page.waitForSelector('svg circle', { timeout: 20000 });

  const ringScore = page
    .locator('.relative')
    .filter({ has: page.locator('svg circle') })
    .first()
    .locator('span.tabular-nums')
    .first();
  await ringScore.waitFor({ state: 'visible', timeout: 15000 });
  const scoreText = (await ringScore.textContent())?.trim() ?? '';

  const threeDecimalPattern = /^\d+\.\d{3}$/;
  const integerPattern = /^\d+$/;
  const formatOk = threeDecimalPattern.test(scoreText) || integerPattern.test(scoreText);
  const noLongFloat = !scoreText.includes('424242');

  await page.screenshot({
    path: 'compliance-ring-check.png',
    fullPage: false,
    clip: { x: 200, y: 120, width: 400, height: 280 },
  });

  console.log('URL:', `${BASE}/compliance`);
  console.log('Ring score text:', JSON.stringify(scoreText));
  console.log('Format OK (max 3 decimals):', formatOk);
  console.log('No long float leak:', noLongFloat);
  console.log('Screenshot: web/compliance-ring-check.png');

  await browser.close();

  if (!formatOk || !noLongFloat) {
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
