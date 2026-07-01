import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const require = createRequire(import.meta.url);
const { chromium } = require(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'web', 'node_modules', 'playwright'));

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const inputName = process.argv[2] ?? 'sidebar-recommendations-comparison';
const htmlPath = path.join(root, 'docs', 'mockups', `${inputName}.html`);
const outPath = path.join(root, 'docs', 'mockups', `${inputName}.png`);

const browser = await chromium.launch({ channel: 'msedge' });
const page = await browser.newPage({ viewport: { width: 1800, height: 1100 } });
await page.goto(`file:///${htmlPath.replace(/\\/g, '/')}`, { waitUntil: 'networkidle' });
await page.waitForTimeout(500);
await page.screenshot({ path: outPath, fullPage: true });
await browser.close();

console.log(`Saved: ${outPath}`);
