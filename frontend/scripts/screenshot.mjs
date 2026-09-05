/**
 * Скрипт снимков каркаса: `npm run screens`.
 * Не входит в npm run test (уточнение к FE-0): сам поднимает vite dev через API,
 * снимает каркас в mock-режиме на 1440×900 и 390×844 (touch) и закрывает сервер.
 * Браузер: CHROMIUM_PATH → системный chromium → установленный playwright chromium.
 */
import { existsSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { chromium } from 'playwright';
import { createServer } from 'vite';

const SYSTEM_CHROMIUM = [
  '/usr/bin/chromium',
  '/usr/bin/chromium-browser',
  '/usr/bin/google-chrome-stable',
  '/usr/bin/google-chrome',
];

const executablePath = process.env.CHROMIUM_PATH ?? SYSTEM_CHROMIUM.find(existsSync);

const server = await createServer({
  root: process.cwd(),
  logLevel: 'error',
  server: { port: 5173, strictPort: true },
});
await server.listen();
console.log('dev-сервер поднят: http://localhost:5173');

const SHOTS = [
  { file: '1440.png', viewport: { width: 1440, height: 900 }, mobile: false },
  { file: '390.png', viewport: { width: 390, height: 844 }, mobile: true },
];

mkdirSync(path.resolve('docs/screens'), { recursive: true });

let browser;
try {
  browser = await chromium.launch({ headless: true, executablePath });
} catch (error) {
  console.error(
    'Не удалось запустить браузер. Установи системный chromium, задай CHROMIUM_PATH или выполни `npx playwright install chromium`.',
  );
  await server.close();
  throw error;
}

try {
  for (const shot of SHOTS) {
    const context = await browser.newContext({
      viewport: shot.viewport,
      deviceScaleFactor: 1,
      isMobile: shot.mobile,
      hasTouch: shot.mobile,
    });
    const page = await context.newPage();
    await page.goto('http://localhost:5173/panel.html?mock=1', { waitUntil: 'load' });
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(400);
    await page.screenshot({ path: path.resolve('docs/screens', shot.file) });
    await context.close();
    console.log(`скриншот: docs/screens/${shot.file}`);
  }
} finally {
  await browser.close();
  await server.close();
}
