/**
 * Сценарии DoD FE-3 (one-off): контур → добавление и анализ → перекраска;
 * рисование тапами на mobile; drag не добавляет вершин; спутник; empty/failed.
 * Запуск: node /tmp/fe3-check.mjs  (dev-сервер должен быть поднят на 5173)
 */
import { chromium } from 'playwright';

const BASE = 'http://localhost:5173';
const SHOTS = '/home/prost/hack/cosmohack/frontend/docs/screens';
const EXE = '/usr/bin/chromium';

const browser = await chromium.launch({ headless: true, executablePath: EXE });
const results = [];
const check = (name, ok, extra = '') => {
  results.push(`${ok ? 'PASS' : 'FAIL'}: ${name}${extra ? ` (${extra})` : ''}`);
};

// ---------- Десктоп 1440: контуры → участок → анализ → перекраска ----------
{
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await page.goto(`${BASE}/?mock=1`, { waitUntil: 'load' });
  await page.waitForTimeout(2500); // карта + useAreas

  // 1. найти контуры
  await page.click('button:has-text("Найти контуры в этой области")');
  await page.waitForTimeout(1200);
  const okText = await page.locator('text=3 контура').count();
  check('контуры найдены (3 контура)', okText > 0);
  await page.screenshot({ path: `${SHOTS}/fe3-map-1440.png` });

  // 2. клик по контуру справа от центра
  const map = page.locator('.maplibregl-canvas');
  const box = await map.boundingBox();
  await page.mouse.click(box.x + box.width / 2 + 120, box.y + box.height / 2);
  await page.waitForTimeout(800);
  const popover = await page.locator('aside:has-text("Добавить участок")').count();
  check('поповер контура открыт', popover > 0);

  // 3. добавить и проанализировать
  await page.click('button:has-text("Добавить участок")');
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${SHOTS}/fe3-add-dialog.png` });
  await page.click('button:has-text("Добавить и проанализировать")');
  await page.waitForTimeout(1500);
  const running = await page.locator('text=Выполняется').count();
  check('участок в списке со статусом «Выполняется»', running > 0);

  // 4. дождались завершения → перекрасился (в списке появился verdict)
  await page.waitForTimeout(6500);
  const done = await page.locator('text=Возможное изменение').count();
  check('после завершения участок перекрасился (verdict в списке)', done > 0);
  await page.screenshot({ path: `${SHOTS}/fe3-map-after.png` });
  await page.close();
}

// ---------- Десктоп: спутник ----------
{
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await page.goto(`${BASE}/?mock=1`, { waitUntil: 'load' });
  await page.waitForTimeout(2500);
  await page.click('button:has-text("Спутник")');
  await page.waitForTimeout(3500); // загрузка тайлов Esri
  const tiles = await page.locator('.maplibregl-canvas').count();
  const satLayer = await page.evaluate(() => {
    const maps = document.querySelectorAll('.maplibregl-canvas');
    return maps.length > 0;
  });
  check('спутник: канвас на месте', tiles > 0 && satLayer);
  await page.screenshot({ path: `${SHOTS}/fe3-satellite.png` });
  await page.close();
}

// ---------- Десктоп: empty/failed контуры ----------
{
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await page.goto(`${BASE}/?mock=1&mock_case=empty`, { waitUntil: 'load' });
  await page.waitForTimeout(2500);
  await page.click('button:has-text("Найти контуры в этой области")');
  await page.waitForTimeout(1000);
  const empty = await page.locator('text=Контуры не найдены в этой области').count();
  const drawBtn = await page.locator('button:has-text("Нарисовать участок")').count();
  check('empty: «Контуры не найдены» + «Нарисовать участок»', empty > 0 && drawBtn > 0);
  await page.screenshot({ path: `${SHOTS}/fe3-contours-empty.png` });
  await page.close();

  const page2 = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await page2.goto(`${BASE}/?mock=1&mock_case=failed`, { waitUntil: 'load' });
  await page2.waitForTimeout(2500);
  await page2.click('button:has-text("Найти контуры в этой области")');
  await page2.waitForTimeout(1000);
  const failed = await page2.locator('text=Не удалось получить контуры').count();
  check('failed: «Не удалось получить контуры»', failed > 0);
  await page2.screenshot({ path: `${SHOTS}/fe3-contours-failed.png` });
  await page2.close();
}

// ---------- Мобиль 390: рисование тапами → сохранить; drag не добавляет ----------
{
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });
  const page = await context.newPage();
  await page.goto(`${BASE}/?mock=1`, { waitUntil: 'load' });
  await page.waitForTimeout(2500);
  await page.click('nav[aria-label="Основная навигация"] button:has-text("Карта")');
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${SHOTS}/fe3-map-390.png` });

  // режим рисования
  await page.click('[aria-label="Нарисовать участок"]');
  await page.waitForTimeout(800);

  const canvas = page.locator('.maplibregl-canvas');
  const box = await canvas.boundingBox();
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;

  // drag карты в режиме рисования не должен добавить вершин
  await page.mouse.move(cx - 100, cy);
  await page.mouse.down();
  await page.mouse.move(cx + 60, cy - 40, { steps: 12 });
  await page.mouse.up();
  await page.waitForTimeout(600);
  let counter = await page.locator('text=Вершин: 0').count();
  check('drag карты в режиме рисования не добавляет вершин', counter > 0);

  // три тапа → 3 вершины
  for (const [dx, dy] of [
    [-60, -30],
    [70, -10],
    [0, 60],
  ]) {
    await page.mouse.click(cx + dx, cy + dy);
    await page.waitForTimeout(400);
  }
  counter = await page.locator('text=Вершин: 3').count();
  check('три тапа поставили 3 вершины', counter > 0);

  // «Отменить точку» → 2, вернуть обратно → 3
  await page.click('button:has-text("Отменить точку")');
  await page.waitForTimeout(400);
  const afterUndo = await page.locator('text=Вершин: 2').count();
  check('«Отменить точку» снимает вершину', afterUndo > 0);
  await page.mouse.click(cx + 5, cy + 55);
  await page.waitForTimeout(400);

  // завершить → «Только сохранить»
  await page.click('button:has-text("Завершить")');
  await page.waitForTimeout(800);
  const dialog = await page.locator('text=Добавить и проанализировать').count();
  check('диалог добавления открыт после завершения', dialog > 0);
  await page.screenshot({ path: `${SHOTS}/fe3-draw-dialog-390.png` });
  await page.click('button:has-text("Только сохранить")');
  await page.waitForTimeout(1200);
  await page.click('nav[aria-label="Основная навигация"] button:has-text("Участки")');
  await page.waitForTimeout(800);
  const inList = await page.locator('text=Участок 5').count();
  check('нарисованный участок появился в списке', inList > 0);
  await page.screenshot({ path: `${SHOTS}/fe3-list-390.png` });
  await context.close();
}

await browser.close();
console.log(results.join('\n'));
