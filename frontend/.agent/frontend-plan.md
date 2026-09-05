# Frontend AgroPulse — спецификация для реализации и промпты для агента

Всё ниже — только твоя зона: `frontend/`, дизайн-система, макеты, `.agent/ui-spec.md`. Документ построен так, чтобы его можно было целиком положить в `frontend/.agent/` и скармливать агенту по разделам. Источники истины: **design-brief.pdf** (поведение, состояния, что нельзя), **2.pdf** (что обязан показывать интерфейс: полигон, исходный и восстановленный ряд `primary_ndvi`, негативные периоды, интерпретация).

---

## 0. Что фиксируем сразу (решения, чтобы не спорить потом)

| Вопрос | Решение | Почему |
|---|---|---|
| Тема | **Светлая** основная; тёмная — не делаем в MVP | Бриф: «диаграммы и состояния важнее декора», светлая подложка карты читается лучше с векторными контурами; тёмная тема удваивает работу над цветами статусов |
| Композиция | Вариант A (по брифу): список слева → карта в центре → карточка справа → график под картой. Вариант B рисуется только как эскиз для выбора | A однозначно ложится на мобильные вкладки «Участки / Карта / Анализ» |
| Карта | Подложка + векторные полигоны. Никаких растров NDVI, до/после, тепловых карт | Бриф §4 «Карта и легенда» |
| Прогресс | Только стадия из бэкенда, без процентов и ETA | Бриф §3B |
| Математика | Frontend ничего не считает: ни z, ни статусов, ни отклонений | Бриф §4 |
| Расширения (после P0) | текстовый поиск места, экспорт, сравнение участков | Бриф §7 |
| Не делаем вообще | аккаунты, уведомления, AI-чат, загрузка CSV, обучение модели | Бриф §7 |

---

## 1. Стек frontend (зафиксировать в `package.json`, версии не ниже)

```
node 22 LTS, npm
vite ^5.4            react ^18.3           typescript ^5.6
@tanstack/react-router ^1.7x   @tanstack/react-query ^5.59   zustand ^5
tailwindcss ^3.4     shadcn/ui (CLI, компоненты: button, dialog, sheet, drawer, tabs, tooltip, popover,
                      badge, skeleton, input, label, select, toast/sonner, alert, collapsible, scroll-area)
lucide-react ^0.45x  motion ^11 (только для появления панелей; уважать prefers-reduced-motion)
maplibre-gl ^4.7     react-map-gl ^7.1 (import из 'react-map-gl/maplibre')
terra-draw ^1.0      terra-draw-maplibre-gl-adapter ^1.0
@turf/area @turf/bbox @turf/kinks @turf/boolean-valid ^7.1
echarts ^5.5         echarts-for-react ^3.0.2
dayjs ^1.11 (+ plugins utc, customParseFormat, локаль ru)
openapi-typescript ^7  openapi-fetch ^0.13   msw ^2.6
vitest ^2.1  @testing-library/react ^16  @playwright/test ^1.48  @biomejs/biome ^1.9
```

Подложка карты: **CARTO Positron** `https://basemaps.cartocdn.com/gl/positron-gl-style/style.json` (бесплатно, обязательна атрибуция «© OpenStreetMap contributors © CARTO»). Запасной — OpenFreeMap `https://tiles.openfreemap.org/styles/positron`.

---

## 2. Дизайн-система

### 2.1. Идея

«Спокойный рабочий инструмент агронома-аналитика»: много белого, светло-серая подложка карты, один цвет действия, а весь «цвет» отдан данным — происхождению значений и статусам. Ничего не пульсирует, не светится. Референсы по духу: Linear (иерархия), Felt (карта + панели), Observable (графики).

### 2.2. Токены (`src/styles/tokens.css`)

```css
:root {
  /* Поверхности */
  --bg-app: #F5F7FA;        /* фон приложения вокруг панелей */
  --bg-surface: #FFFFFF;    /* панели, карточки */
  --bg-muted: #EEF1F5;      /* вторичные области, заголовки таблиц */
  --bg-hover: #F1F4F8;
  --border: #E2E7EE;
  --border-strong: #C6CFDA;

  /* Текст */
  --fg: #151A21;
  --fg-secondary: #4A5563;
  --fg-tertiary: #7B8794;
  --fg-on-accent: #FFFFFF;

  /* Действие (кнопки, ссылки, фокус, выбранный элемент списка) */
  --action: #2D5BE3;
  --action-hover: #244BC0;
  --action-soft: #E8EEFC;
  --focus-ring: #2D5BE3;

  /* Происхождение значения (график, легенда, подсказка) */
  --observed: #0E7C6B;      /* «Наблюдения» — тёмно-бирюзовый, закрашенный круг */
  --imputed: #7A3FE0;       /* «Восстановлено» — фиолетовый, полый ромб, пунктир */
  --missing: #9AA5B1;       /* «Нет данных» — только в подписи/подсказке */
  --background-band: rgba(100,116,139,.16);  /* сезонный фон p10–p90 */
  --background-mean: #64748B;                /* линия среднего фона, штрих */

  /* Вывод анализа (карточка, список, полигон на карте) */
  --verdict-normal: #1B7F3B;        /* «Негативных отклонений не выявлено» */
  --verdict-candidate: #C77700;     /* «Возможное изменение» */
  --verdict-confirmed: #C8102E;     /* «Подтверждённое изменение» */
  --verdict-insufficient: #6B7785;  /* «Недостаточно данных» — нейтральный, НЕ зелёный */
  --verdict-none: #A8B3C0;          /* участок не анализировался — пунктирная обводка */

  /* Состояние задачи */
  --job-queued: #6B7785;  --job-running: #2D5BE3;  --job-failed: #C8102E;  --job-cancelled: #6B7785;

  /* Погода */
  --temp: #D9531E;          /* линия температуры, °C */
  --precip: #1477B8;        /* столбцы осадков, мм */

  /* Карта */
  --contour-found: #2D5BE3;       /* найденные контуры OSM — тонкий штрих */
  --contour-found-fill: rgba(45,91,227,.08);
  --area-fill-alpha: .22;
  --area-selected-outline: #151A21;  /* выбранный — тёмная обводка 3px с белым гало, независимо от статуса */

  /* Геометрия */
  --radius-sm: 6px; --radius: 10px; --radius-lg: 14px;
  --shadow-1: 0 1px 2px rgba(21,26,33,.06), 0 1px 3px rgba(21,26,33,.08);
  --shadow-2: 0 4px 16px rgba(21,26,33,.10);
  --tap-min: 44px;   /* минимальная область нажатия — правило проекта */
}
```

Контраст всех цветов текста на белом ≥ 4.5:1; статусные цвета всегда сопровождаются текстом или иконкой (бриф §4, §7).

### 2.3. Типографика

- Шрифт: **Inter** (variable, `font-feature-settings: "tnum" 1, "cv11" 1`) — единственный. Числа везде табличные.
- Шкала: 12 (подписи осей, метаданные), 13 (вторичный текст, список), 14 (основной), 16 (заголовки карточек), 20 (заголовок панели), 24 (название участка в шапке на десктопе).
- Начертания: 400 текст, 500 подписи/лейблы, 600 заголовки и числа-значения. Не использовать 700+.
- Межстрочный: 1.45 текст, 1.25 заголовки.

### 2.4. Сетка и отступы

- Базовый шаг 4px; внутренние отступы панелей 16px, карточек 12–16px, между блоками 12px.
- Ширины: список участков 300px, карточка 380px, шапка 56px, нижняя зона (график + погода) 440px на десктопе (график 300 + погода 2×60 + подписи).
- Брейкпоинты: `≥1280` десктоп (3 колонки + низ), `1024–1279` планшет (список сворачивается в рейку 56px, карточка — выезжающая панель поверх карты), `<1024` → мобильная компоновка с вкладками. Проверять 1440, 1024, 390, 360, 430.

### 2.5. Иконки и состояния

- `lucide-react`, 16px в тексте, 20px в кнопках, stroke 1.75.
- Иконки статусов вывода: `normal` — `CircleCheck`, `candidate` — `TriangleAlert`, `confirmed` — `OctagonAlert`, `insufficient_data` — `CircleHelp`; задача: `queued` — `Clock`, `running` — `LoaderCircle` (анимация вращения, отключается при reduced-motion), `failed` — `CircleX`.
- Происхождение на графике: наблюдение — закрашенный круг 6px; восстановление — полый ромб 7px + пунктир 4/4; пропуск — разрыв линии.
- Фокус: `outline: 2px solid var(--focus-ring); outline-offset: 2px` на всём интерактивном. Все действия доступны с клавиатуры, ни одна подсказка не зависит только от hover.

---

## 3. Словарь строк интерфейса (`src/lib/labels.ts`)

Строки берутся из брифа дословно — это то, что проверят.

```ts
export const JOB_LABEL = { queued:'В очереди', running:'Выполняется', completed:'Завершён', failed:'Ошибка', cancelled:'Отменён' };
export const VERDICT_LABEL = {
  normal:'Негативных отклонений не выявлено', candidate:'Возможное изменение',
  confirmed:'Подтверждённое изменение', insufficient_data:'Недостаточно данных' };
export const PROVENANCE_LABEL = { observed:'Наблюдение', imputed:'Восстановлено', missing:'Нет данных' };
export const STAGE_LABEL = { satellite:'Получение спутниковых данных', weather:'Получение погоды',
  prepare:'Подготовка данных', analysis:'Анализ' };          // ключи стадий согласовать с B4; неизвестная → 'Анализ выполняется'
export const EMPTY = {
  noAreas: 'Участков пока нет. Найдите сельхозконтуры в видимой области карты или нарисуйте участок.',
  contoursNotFound: 'Контуры не найдены в этой области',
  contoursFailed: 'Не удалось получить контуры',
  noSatellite: 'Нет пригодных спутниковых данных за выбранный период',
  littleHistory: 'Недостаточно истории для сравнения с сезонным фоном',
  connectionLost: 'Нет связи, состояние задачи неизвестно',
  weatherPartial: 'Погода доступна не на весь период',
  causeUnknown: 'Причина по доступным данным не установлена',
  deleteActive: 'Участок будет удалён. Результат текущего анализа не будет сохранён.',
  demo: 'Демонстрационные данные',
};
```

Терминология: «участок» (не «поле», не «полигон» в UI), «контур» для найденных OSM-геометрий, «сезонный фон» (не «норма»), «восстановлено» (не «прогноз»), «возможная причина» (не «диагноз»).

---

## 4. Модель данных на фронте и слой адаптеров

Бэкенд-контракт ещё не заморожен (бриф §6). Поэтому компоненты работают **только с view-моделями** из `src/api/types.ts`, а `src/api/adapters/*.ts` переводят сырые ответы в них. Когда B3 меняет поле — правится один адаптер.

```ts
// src/api/types.ts — модели интерфейса (не OpenAPI!)
export type JobStatus = 'queued'|'running'|'completed'|'failed'|'cancelled';
export type Verdict = 'normal'|'candidate'|'confirmed'|'insufficient_data';
export type Provenance = 'observed'|'imputed'|'missing';
export type SourceStatus = 'ok'|'partial'|'unavailable';

export interface Period { from: string; to: string }              // YYYY-MM-DD, UTC

export interface Area {
  id: string; name: string; geometry: GeoJSON.Polygon|GeoJSON.MultiPolygon;
  source: { kind: 'contour'|'drawn'; label: string; externalId?: string };   // label: «Контур OpenStreetMap», «Нарисован вручную»
  createdAt: string;
  lastResult?: ResultMeta;       // сохранённый результат (может быть от старого периода)
  activeJob?: JobMeta;           // текущая задача, если есть
}
export interface ResultMeta {
  resultVersion: string; period: Period; computedAt: string;
  verdict: Verdict; severity?: string|null;                        // severity — строка от ML, показываем как есть
  sources: Record<string, { status: SourceStatus; note?: string }>; // {'sentinel2': {...}, 'modis': ..., 'era5': ...}
  limitations: string[];
}
export interface JobMeta {
  id: string; areaId: string; requestedPeriod: Period; status: JobStatus;
  stage?: string|null; message?: string|null; errorCode?: string|null; errorMessage?: string|null;
  resultVersion?: string|null; updatedAt: string;
}
export interface SeriesPoint {
  date: string; ndvi: number|null; provenance: Provenance;
  quality?: string|null;                      // текстовое качество от B1, если есть
  background?: { mean: number; low?: number; high?: number }|null;
  deviation?: { value: number; unit: 'ndvi'|'percent'; base: string }|null;   // «−0.18 NDVI к сезонному фону 2017–2024»
  z?: number|null; source?: string|null;
}
export interface Series {
  areaId: string; resultVersion: string; period: Period; points: SeriesPoint[];
  background?: { label: string; yearsFrom: number; yearsTo: number; source: string; bandMeaning?: string }|null;
  weather?: {
    temperature: { date: string; value: number|null }[]; precipitation: { date: string; value: number|null }[];
    units: { temperature: '°C'; precipitation: 'мм' };
    aggregation: { temperature: string; precipitation: string };  // «средняя за интервал», «сумма за интервал»
    coverage: Period; source: string; spatialNote: string;        // «ячейка реанализа ERA5-Land ~9 км»
  }|null;
}
export interface AnalysisEvent {
  id: string; period: Period; verdict: 'candidate'|'confirmed'; severity?: string|null;
  detected: { magnitude: number; unit: 'ndvi'|'percent'; base: string; text: string };
  basis: { observedCount: number; imputedCount: number; backgroundComparable: boolean; gapsNote?: string; criteria?: string };
  weather?: { facts: { label: string; value: string }[]; hypothesis?: string|null }|null;
  limitations: string[];
}
export interface ResultBundle { meta: ResultMeta; series: Series; events: AnalysisEvent[] }   // одна версия целиком
```

Маршруты (по брифу §6; адаптеры под них, подтвердить с B3 в первые 2 часа):

| Назначение | Маршрут | Адаптер |
|---|---|---|
| Контуры в области | `GET /api/regions/contours?bbox=w,s,e,n` → `{features, status:'ok'|'empty'|'failed', source, coverageNote}` | `adaptContours` — различать `empty` и `failed` |
| Участки | `GET/POST /api/areas`, `DELETE /api/areas/{id}` | `adaptArea` |
| Лимиты формы | либо `GET /api/config` `{limits:{areaHaMax, verticesMax, periodDaysMax, minDate}}`, либо поле `limits` в ошибке 422 | `adaptLimits` — **числа только отсюда**, в коде не хардкодить |
| Запуск | `POST /api/areas/{id}/analyses {period}` → `{jobId}` (200 с существующим jobId, если уже идёт) | |
| Прогресс | `GET /api/jobs/{id}`; если B3 даст SSE — `useJobStream`, иначе `useJobPolling` (2 с) | `adaptJob` |
| Ряд + погода | `GET /api/areas/{id}/series?version=` | `adaptSeries` |
| События | `GET /api/areas/{id}/events?version=` | `adaptEvents` |

Правило согласованности (бриф §6): `ResultBundle` собирается в `useResultBundle(areaId, version)` — пока не пришли **и** series, **и** events одной версии, UI показывает предыдущий bundle целиком или скелетон. Никогда не смешивать версии.

---

## 5. Структура `frontend/`

```
frontend/
├── .agent/
│   ├── ui-spec.md            # фиксируется после выбора макета (шаблон в §11)
│   └── frontend-plan.md      # этот документ
├── public/fonts/inter/
├── src/
│   ├── main.tsx  app/router.tsx  app/providers.tsx  app/AppShell.tsx
│   ├── api/
│   │   ├── client.ts         # openapi-fetch + baseUrl + обработка RFC7807
│   │   ├── types.ts          # view-модели (§4)
│   │   ├── adapters/         # contours.ts areas.ts jobs.ts series.ts events.ts limits.ts
│   │   ├── queries.ts        # useAreas, useArea, useContours, useJob, useResultBundle, useLimits
│   │   ├── mutations.ts      # useCreateArea, useDeleteArea, useStartAnalysis
│   │   └── mocks/            # fixtures/*.json (normal, candidate, confirmed, insufficient, job-*, error-*), msw/handlers.ts
│   ├── store/
│   │   ├── selection.ts      # selectedAreaId, selectedEventId, selectedDate — синхронизировано с URL (?area=&event=&date=)
│   │   ├── draft.ts          # режим рисования, черновик геометрии, ошибки валидации
│   │   └── ui.ts             # mobileTab, listCollapsed, cardOpen, demoMode
│   ├── features/
│   │   ├── shell/            # Header, PeriodPicker, MobileTabBar, DemoBadge
│   │   ├── areas/            # AreaList, AreaListItem, AreaCard, AreaSummary, AddAreaDialog, DeleteAreaDialog, SourcesStatus
│   │   ├── map/              # MapView, useMapLayers, ContoursButton, ContourPopover, DrawToolbar, DrawHints, MapLegend, MapEmptyHint
│   │   ├── analysis/         # NdviChart, WeatherCharts, ChartLegend, PointDetails, EventList, EventCard, JobStatusBar, ResultProvenance
│   │   └── states/           # EmptyState, ErrorState, PartialDataNotice, InsufficientHistoryNotice
│   ├── components/ui/        # shadcn
│   ├── lib/                  # labels.ts format.ts (числа/даты/единицы) dates.ts geo.ts (площадь, bbox, kinks) series.ts (разбиение по происхождению) chart-theme.ts
│   └── styles/tokens.css globals.css
├── tests/ (vitest unit) e2e/ (playwright: desktop.spec.ts mobile.spec.ts)
├── index.html vite.config.ts tailwind.config.ts biome.json tsconfig.json package.json
```

---

## 6. Экраны и компоненты

### 6.1. Десктоп 1440 (вариант A)

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [логотип] AgroPulse   Участок: «Поле у Тимашевска»   Период: [01.03.2024 – 30.09.2024 ▾]  [Запустить анализ]  │ 56px
├──────────────┬────────────────────────────────────────────────┬──────────────────────┤
│ УЧАСТКИ (300)│ КАРТА                                          │ КАРТОЧКА УЧАСТКА(380)│
│ [+ Добавить] │  ┌ Найти контуры в этой области ┐ [Нарисовать] │ Поле у Тимашевска    │
│ ● Поле у Т.  │  │                              │              │ Контур OpenStreetMap │
│   Подтвержд. │  │      (полигоны + контуры)    │              │ Период: 03–09.2024   │
│   03–09.2024 │  │                              │              │ Рассчитан 12.09 14:02│
│ ○ Участок 2  │  └──────────────────────────────┘              │ ─────────────────── │
│   Выполняется│  Легенда: ● наблюд. ◇ восст. | статусы         │ ▣ Подтверждённое     │
│ ◌ Участок 3  │  © OpenStreetMap contributors © CARTO          │   изменение          │
│   Не анализ. │                                                │ −0.21 NDVI к фону    │
│              │                                                │ 2017–2023 (MODIS)    │
│              ├────────────────────────────────────────────────┤ Основание: 14 набл., │
│              │ ГРАФИК NDVI (300px) ● Наблюдения ◇ Восстановлено ░ Сезонный фон ▮ События │ 5 восст.            │
│              │ ...                                            │ Ограничения: ...     │
│              ├────────────────────────────────────────────────┤ Источники: S2 ● MODIS│
│              │ Температура, °C (средняя за интервал) ─────── │ ● ERA5 ◐             │
│              │ Осадки, мм (сумма за интервал)      ▁▃▂▁▅     │ [События ▾] 2        │
└──────────────┴────────────────────────────────────────────────┴──────────────────────┘
```

**Вариант B (только эскиз для выбора)**: список слева 300, карта во всю оставшуюся высоту, справа колонка 440 с прокруткой: карточка → график → погода → события. Плюс: карта выше, график шире не нужен. Минус: график узкий (440), хуже читается многолетний ряд. Рекомендация — A.

### 6.2. Планшет 1024

Список → рейка 56px с кнопкой «Участки» (открывает Sheet слева). Карточка → Sheet справа поверх карты, открывается при выборе участка, закрывается кнопкой. Низ (график+погода) остаётся под картой, высота 400.

### 6.3. Телефон 390 (P0)

- Нижняя панель вкладок 56px: **Участки · Карта · Анализ**. Шапка 48px: название участка (обрезка) + кнопка периода.
- **Участки**: список карточек (имя, статус вывода с иконкой, период результата, состояние задачи), кнопка «+ Добавить» → переводит на «Карта» в режим добавления.
- **Карта**: карта на весь экран; сверху две кнопки 44px «Найти контуры» / «Нарисовать»; при выборе участка снизу выезжает Drawer с краткой карточкой (статус, период, «Открыть анализ»). Режим рисования: нижняя панель «Отменить точку · Завершить · Отмена», вершины по касанию, перетаскивание карты вершины не добавляет.
- **Анализ**: график по ширине экрана (высота 240), под ним закреплённая подсказка выбранной точки (не всплывающая), затем события (карточки на всю ширину), затем погода (2 графика по 90px), затем источники/ограничения. Масштабирование — только внутри графика (pinch/drag по оси X), страница горизонтально не скроллится.
- При переключении вкладок сохраняются `selectedAreaId`, `selectedEventId`, `selectedDate`, период, состояние задачи (всё в zustand + URL).

### 6.4. Компоненты и их состояния

| Компонент | Обязательные состояния |
|---|---|
| `AreaListItem` | default / selected (левая полоса `--action` + фон `--action-soft`) / с активной задачей (спиннер + «Выполняется · Получение погоды») / без результата («Не анализировался», пунктирная точка) / результат старого периода (подпись периода серым) / hover / focus |
| `AreaCard` | 1-й уровень: статус вывода (иконка+текст), период результата и время расчёта, величина отклонения с базой («−0.21 NDVI к сезонному фону 2017–2023, MODIS»), краткое основание («14 наблюдений, 5 восстановлено»), 1–2 ограничения. Раскрытие: источники со статусами, все ограничения, критерии детектора, версия результата. Варианты: normal / candidate / confirmed / insufficient_data (при insufficient — нет зелёного, нет величины отклонения) / нет результата / результат + новая задача рядом |
| `JobStatusBar` | queued / running(+стадия или «Анализ выполняется») / failed (причина + «Запустить заново») / cancelled / connection-lost («Нет связи, состояние задачи неизвестно», продолжаем опрашивать ту же задачу) |
| `ContoursButton` | idle / loading («Ищем контуры…») / empty («Контуры не найдены в этой области» + «Нарисовать участок») / failed («Не удалось получить контуры» + «Повторить») / ok (N контуров + примечание о неполноте каталога по кнопке ⓘ) / stale (карту сдвинули: «Область изменилась — искать снова») |
| `DrawToolbar` | off / drawing (подсказка «Ставьте вершины касанием или кликом», кнопки Отменить точку / Завершить (≥3 вершин) / Отмена) / draft-invalid (текст ошибки: самопересечение, мало вершин, площадь вне лимита — лимиты из `useLimits`) / draft-ready |
| `AddAreaDialog` | поля: название (обязательное, автозаполнение «Участок N» или имя контура), период (from/to, ограничения из лимитов), источник (только чтение), площадь; кнопки «Добавить и проанализировать» (primary) и «Только сохранить»; состояние ошибки сохранения; состояние «сохранено, запуск не удался» → диалог закрывается, участок в списке с кнопкой «Запустить анализ» |
| `DeleteAreaDialog` | текст с именем; если задача активна — добавочная строка из `EMPTY.deleteActive`; ошибка удаления оставляет участок и показывает причину |
| `NdviChart` | loading / empty (нет пригодных данных — текст, без осей с «здоровым» рядом) / ready / ready-no-background (без фона, подпись «Сезонный фон недоступен: мало истории») / с выбранным событием / с выбранной точкой |
| `PointDetails` (закреплённая подсказка) | дата или интервал; NDVI и тип («Наблюдение»/«Восстановлено»/«Нет данных»); при наличии: сезонный фон, отклонение с базой, качество, источник; z-score — в раскрытии «Подробнее» с пояснением «стандартизованное отклонение от сезонного фона» |
| `EventCard` | четыре блока с заголовками: **Что обнаружено** / **На чём основано** / **Погодный контекст и гипотеза** (факты списком, гипотеза отдельным абзацем или `EMPTY.causeUnknown`) / **Ограничения** (только применимые). Бейджи: вывод (candidate/confirmed) отдельно от тяжести (если есть). Состояния: collapsed / expanded / selected (синхронно с графиком) |
| `WeatherCharts` | ready / partial (недоступный диапазон заштрихован + подпись) / unavailable («Погода недоступна; отсутствие данных не означает отсутствие осадков») |
| `MapLegend` | статусы вывода + «не анализировался» + «найденный контур» + «выбранный»; на мобиле — сворачиваемая |
| `ResultProvenance` | версия результата, период, время расчёта, источники и причины отсутствия — раскрывающийся блок внизу карточки |
| `DemoBadge` | виден всегда в mock-режиме: «Демонстрационные данные» |

---

## 7. График NDVI — точная спецификация (`features/analysis/NdviChart.tsx`)

**Подготовка данных** (`lib/series.ts`, покрыть vitest):

```ts
// Разбивает ряд на серии так, чтобы:
//  - наблюдения соединялись линией только между соседними наблюдениями;
//  - восстановленные участки рисовались отдельной пунктирной серией, включающей граничные наблюдения
//    (чтобы пунктир «висел» между двумя реальными точками);
//  - точки provenance='missing' давали разрыв в обеих сериях (null, connectNulls:false).
export function splitByProvenance(points: SeriesPoint[]): {
  observedLine: [string, number|null][]; observedDots: [string, number][];
  imputedLine: [string, number|null][]; imputedDots: [string, number][];
  bandLow: [string, number|null][]; bandDelta: [string, number|null][]; bgMean: [string, number|null][];
}
```

**ECharts option (ключевые части)**:

```ts
xAxis: { type:'time', axisLabel:{ hideOverlap:true, formatter: adaptiveDateFormatter }, boundaryGap:false },
yAxis: { type:'value', name:'NDVI', nameLocation:'end', min: v => Math.min(-0.1, Math.floor(v.min*10)/10), max: v => Math.max(1, Math.ceil(v.max*10)/10), splitLine:{ lineStyle:{ color:'#EEF1F5' } } },
series: [
  // сезонный фон — только если series.background != null
  { id:'bandLow',  type:'line', stack:'band', data:bandLow,   lineStyle:{opacity:0}, symbol:'none', silent:true, tooltip:{show:false} },
  { id:'bandHigh', type:'line', stack:'band', data:bandDelta, lineStyle:{opacity:0}, symbol:'none', areaStyle:{color:'var(--background-band)'}, silent:true, tooltip:{show:false}, name:'Сезонный фон (разброс)' },
  { id:'bgMean',   type:'line', data:bgMean, lineStyle:{type:'dashed', width:1.5, color:'#64748B'}, symbol:'none', name:'Сезонный фон (среднее)' },
  // события — markArea на пустой серии, чтобы не мешать легенде
  { id:'events', type:'line', data:[], markArea:{ silent:false, data: events.map(e => [{ xAxis:e.period.from, itemStyle:{ color: e.verdict==='confirmed' ? 'rgba(200,16,46,.14)' : 'rgba(199,119,0,.14)', borderColor: selected? verdictColor : 'transparent', borderWidth: selected?1.5:0 }, name:e.id }, { xAxis:e.period.to }]) } },
  { id:'imputedLine', type:'line', data:imputedLine, connectNulls:false, lineStyle:{ type:[4,4], width:1.75, color:'#7A3FE0' }, symbol:'none', name:'Восстановлено', z:3 },
  { id:'observedLine', type:'line', data:observedLine, connectNulls:false, lineStyle:{ width:2, color:'#0E7C6B' }, symbol:'none', name:'Наблюдения', z:4 },
  { id:'imputedDots', type:'scatter', data:imputedDots, symbol:'diamond', symbolSize:8, itemStyle:{ color:'#fff', borderColor:'#7A3FE0', borderWidth:1.75 }, z:5 },
  { id:'observedDots', type:'scatter', data:observedDots, symbol:'circle', symbolSize:6, itemStyle:{ color:'#0E7C6B' }, z:6 },
],
tooltip: { trigger:'axis', triggerOn: isTouch ? 'click' : 'mousemove|click', confine:true, formatter: pointTooltip },  // null → «Нет данных», никогда 0
dataZoom: [{ type:'inside', filterMode:'none', zoomOnMouseWheel:'shift', moveOnMouseMove:true, preventDefaultMouseMove:false }],   // на десктопе wheel только с Shift — чтобы не ломать скролл страницы
grid: { left:48, right:16, top:24, bottom:28 },
animationDuration: reducedMotion ? 0 : 300,
```

Правила:
- Клик/тап по точке → `selection.selectedDate`; `PointDetails` показывает закреплённую подсказку (на мобиле — единственный способ).
- Клик по `markArea` → `selection.selectedEventId`; `EventCard` раскрывается и подсвечивается; выбор события из списка — обратно подсвечивает область и делает `dispatchAction({type:'dataZoom', startValue, endValue})` с полями ±10 %.
- Легенда — своя (`ChartLegend`), не встроенная: подписи «Наблюдения», «Восстановлено», «Сезонный фон: среднее и разброс p10–p90 по годам 2017–2023 (MODIS)» — текст из `series.background.label`. Если фона нет — пункт легенды не показывается, вместо него строка «Сезонный фон недоступен: {причина}».
- `WeatherCharts` — два экземпляра ECharts, `echarts.connect([ndvi, temp, precip])` для общего курсора; температура — линия `--temp`, осадки — столбцы `--precip`; подписи осей с единицами и агрегацией из `series.weather.aggregation`; недоступный диапазон — `markArea` серым с подписью «нет данных».
- Отфильтрованные наблюдения (если B1 их отдаёт с `quality`) — отдельный стиль (полый серый круг) и причина в подсказке; по умолчанию не показываются.

---

## 8. Карта — спецификация (`features/map/`)

- `MapView`: `react-map-gl/maplibre`, стиль Positron, `attributionControl` включён и дополнен строкой «Контуры: © OpenStreetMap contributors». Начальный вид: последняя позиция из `localStorage`, иначе центр РФ, zoom 4.
- Слои и порядок: `contours-fill` → `contours-line` (штрих 1.5px `--contour-found`) → `areas-fill` (цвет по `verdict` через `match`, альфа .22; без результата — прозрачная) → `areas-line` (2px, цвет по verdict; без результата — штрих `--verdict-none`) → `areas-selected-halo` (6px белый) → `areas-selected-line` (3px `--area-selected-outline`) → terra-draw слои.
- Цвет полигона = итог **сохранённого** результата за подписанный период; при выборе даты на графике карта **не** перекрашивается (бриф §3C).
- `ContoursButton`: запрос по `map.getBounds()` только по кнопке; после `moveend` кнопка переходит в состояние `stale`; результаты `empty` и `failed` — разные компоненты; при 422 «область слишком большая» — текст из ошибки бэкенда.
- Клик по найденному контуру → `ContourPopover` (источник, площадь, «Добавить участок» → `AddAreaDialog` с предзаполнением). На мобиле — тот же контент в Drawer снизу.
- `DrawToolbar` + terra-draw `TerraDrawPolygonMode`: явный вход в режим; на тач-устройствах вершина ставится тапом (`terra-draw` по умолчанию различает drag/tap — проверить `pointerDistance: 40` для пальца); «Отменить точку» → `draw.removeLastVertex` (если недоступно в версии — пересоздать фичу без последней вершины); «Завершить» активна при ≥3 вершинах; Esc/«Отмена» очищает. Клиентская валидация в `lib/geo.ts`: `kinks` (самопересечение), площадь через `@turf/area` против `limits`, число вершин; ошибка выводится рядом с кнопкой «Завершить». Серверная ошибка 422 показывается в `AddAreaDialog` у поля и не закрывает диалог.
- Выбор участка на карте (клик по `areas-fill`) и в списке — один и тот же `selection.selectedAreaId`; карта делает `fitBounds` только при выборе из списка (не при клике на карте).
- Пустое состояние без участков: `MapEmptyHint` — компактная карточка поверх карты с текстом `EMPTY.noAreas` и двумя кнопками.

---

## 9. Состояние, задачи, сеть

- `selection` синхронизирован с URL (`/app?area=…&event=…&date=…`); при загрузке по ссылке — восстанавливается.
- `useJob(jobId)`: опрос `GET /api/jobs/{id}` каждые 2 с, пока `queued|running`; при `fetch` ошибке — состояние `connectionLost`, опрос продолжается с backoff до 10 с, **новая задача не создаётся**; при `completed` → инвалидировать `['area', id]` и запросить `useResultBundle(areaId, resultVersion)`; при `failed` — показать `errorMessage`, кнопка «Запустить заново» создаёт новую задачу только по явному клику.
- Кнопка «Запустить анализ» блокируется на время мутации; повторный клик до ответа невозможен; если бэкенд вернул существующий jobId — просто подписываемся.
- Переключение участка не отменяет задачи: `useJob` живёт в `AreaListItem`/глобальном `JobsWatcher`, а не в карточке.
- Старый результат при новом запуске: карточка показывает `lastResult` с подписью «Сохранённый результат · период … · рассчитан …», под ним `JobStatusBar` новой задачи; ошибка новой задачи не трогает `lastResult`.
- Mock-режим: `?mock=1` или `VITE_MOCK=1` включает MSW с фикстурами; в шапке `DemoBadge`. Фикстуры обязаны включать: normal, candidate, confirmed, insufficient_data, ряд с пропусками всех трёх типов происхождения, задача running с реальной стадией и без стадии, failed, контуры empty и failed. Каждая фикстура помечена полем `_synthetic: true`.

---

## 10. Промпты для агента (по порядку; каждому предпослать блок A из командного документа + ссылку на `.agent/frontend-plan.md`)

### FE-0. Каркас и дизайн-система
```
Создай frontend/ по структуре §5. Vite+React+TS, Tailwind с токенами из §2.2 (CSS-переменные + tailwind.config extend.colors через var()), Inter self-hosted,
shadcn/ui инициализирован (компоненты из §1), biome, vitest, playwright (проекты desktop 1440×900 и mobile 390×844 с touch).
AppShell: шапка 56px, три колонки по §6.1, нижняя зона 440px; брейкпоинты §2.4 (планшет — рейка+Sheet, мобильный — MobileTabBar с вкладками Участки/Карта/Анализ).
Роутер: '/' → редирект на '/app'; '/app' с search-параметрами area, event, date, mock. Store §9 (zustand) с синхронизацией selection ↔ URL.
DemoBadge при mock. Все интерактивные элементы: min 44×44 на touch, видимый фокус, aria-label.
DoD: `npm run dev` показывает пустой каркас на 1440 и 390; `npm run lint && npm run test` зелёные; скриншоты обоих размеров в /docs/screens/.
Не делать: карту, графики, API.
```

### FE-1. API-слой, типы, моки
```
Реализуй src/api по §4: types.ts дословно; adapters/* с unit-тестами на маппинг (включая различие contours status empty/failed и null → 'Нет данных');
client.ts (openapi-fetch, baseUrl из VITE_API_URL, RFC7807 → AppError{code,title,detail,extra}); queries.ts и mutations.ts (TanStack Query);
useJob с опросом и состоянием connectionLost по §9; useResultBundle с атомарной сборкой одной версии; useLimits.
Моки: fixtures/*.json по списку §9 (все _synthetic:true), msw/handlers.ts для всех маршрутов таблицы §4, включая задачу, которая проходит стадии
satellite→weather→prepare→analysis по 1.5 с и завершается completed с resultVersion; отдельная фикстура с failed.
DoD: тесты адаптеров и хуков (msw в vitest) зелёные; в mock-режиме useAreas возвращает 4 участка с разными verdict.
```

### FE-2. Список участков и карточка
```
Реализуй features/areas: AreaList, AreaListItem, AreaCard (первый уровень + раскрытие), SourcesStatus, ResultProvenance, DeleteAreaDialog по §6.4 и строкам §3.
Правила: verdict, статус задачи и severity — три независимых поля с отдельными подписями; insufficient_data — нейтральный цвет, без величины отклонения;
отсутствующий показатель → строка скрыта или «Нет данных», не примерное число; результат старого периода → подпись периода и времени расчёта;
удаление с именем участка и предупреждением при активной задаче; ошибка удаления оставляет участок.
Планшет/мобиль: список в Sheet/во вкладке «Участки», карточка в Sheet/в Drawer снизу на «Карте».
DoD: storybook-подобная страница /dev/states (только в dev) со всеми вариантами карточки и элемента списка; e2e: выбрать участок в списке → карточка обновилась → удалить → участок исчез.
```

### FE-3. Карта, контуры, рисование, добавление
```
Реализуй features/map по §8 и AddAreaDialog по §6.4. MapLibre + Positron + атрибуция; слои и порядок из §8; выбранный участок — тёмная обводка с гало независимо от статуса.
ContoursButton с состояниями idle/loading/empty/failed/ok/stale; ContourPopover (десктоп) / Drawer (мобиль). DrawToolbar с terra-draw: вход в режим, вершины тапом,
Отменить точку / Завершить / Отмена, подсказка, клиентская валидация из lib/geo.ts с лимитами из useLimits (никаких захардкоженных чисел).
AddAreaDialog: название, период (ограничения из лимитов), «Добавить и проанализировать» / «Только сохранить»; сценарий «сохранено, запуск не удался» → участок в списке с кнопкой «Запустить анализ».
MapEmptyHint при отсутствии участков. Легенда MapLegend.
DoD: e2e desktop и mobile: найти контуры (mock ok) → выбрать → добавить и проанализировать → участок в списке со статусом «Выполняется»; нарисовать полигон тапами на mobile → сохранить;
состояние contours failed показывает «Не удалось получить контуры» и кнопку «Повторить»; перетаскивание карты в режиме рисования не добавляет вершин.
```

### FE-4. График NDVI, погода, точка
```
Реализуй lib/series.ts (splitByProvenance, тесты на: одиночный пропуск, серия восстановленных между наблюдениями, missing даёт разрыв в обеих сериях, фон отсутствует),
NdviChart по §7 (option дословно, кастомная ChartLegend, выбор точки кликом/тапом, закреплённый PointDetails), WeatherCharts с echarts.connect, единицами и агрегацией из данных,
заштрихованным недоступным диапазоном. Пустые состояния: нет пригодных спутниковых данных (без осей и ряда), нет фона (подпись причины), погода недоступна (текст из §3).
Десктоп: колесо масштабирует только с Shift; мобиль: pinch/drag внутри графика, страница не скроллится по горизонтали (проверить overflow-x hidden на #root и touch-action на контейнере графика).
DoD: визуально различимы наблюдения (круг, бирюзовый) и восстановление (полый ромб, фиолетовый пунктир); null нигде не показан как 0; e2e mobile: тап по точке показывает закреплённую подсказку с типом значения.
```

### FE-5. События и связность
```
Реализуй EventList и EventCard (четыре блока §6.4, бейджи вывода и тяжести отдельно, гипотеза отдельно от фактов, «Причина по доступным данным не установлена» при null),
двустороннюю связь событие ↔ markArea ↔ карточка участка («Перейти к негативному периоду» → выбор события + зум графика). JobStatusBar по §9 с connectionLost и «Запустить заново».
Сценарий повторного анализа: PeriodPicker в шапке задаёт период следующего запуска; старый результат остаётся с подписью, ошибка новой задачи его не трогает.
DoD: e2e: выбрать событие в списке → область на графике подсвечена и карточка раскрыта; клик по области → карточка выбрана; изменить период → запустить → статус проходит стадии → новый результат заменяет старый одной версией.
```

### FE-6. Мобильная полировка и доступность
```
Пройди чек-лист брифа §2 «Телефон» и §7 «Макет готов к апруву» на 360/390/430: вкладки сохраняют выбор; формы видны при открытой клавиатуре (visualViewport, sticky-кнопки);
все цели ≥44px; фокус и клавиатура на десктопе; prefers-reduced-motion; контраст токенов проверен (axe в Playwright).
Состояния из таблицы брифа §5 «Ситуация → сообщение» реализованы каждое отдельным компонентом в features/states и подключены.
DoD: отчёт axe без критичных нарушений; видео e2e прохода на mobile: выбрать участок → событие → объяснение → период → запуск; добавление/удаление; ожидание; ошибка; недостаток данных.
```

### FE-7. Подключение к живому API
```
Переключи client на VITE_API_URL, сгенерируй types из /openapi.json (npm run gen), сверь адаптеры с реальными ответами, зафиксируй расхождения в .agent/ui-spec.md раздел «Нерешённые детали контракта»
и сообщи B3. Mock-режим остаётся рабочим через ?mock=1.
DoD: полный путь на живом бэке; при недоступном бэке приложение показывает ошибку загрузки участков с кнопкой «Повторить», а не пустой список.
```

---

## 11. Шаблон `.agent/ui-spec.md` (заполнить после выбора макета)

```
# UI-spec AgroPulse (утверждено <дата>)
1. Макет: ссылка/скриншоты 1440, 1024, 390. Выбран вариант A. Отличия от эскиза: …
2. Токены: см. src/styles/tokens.css (версия из §2.2). Изменения: …
3. Компоненты и состояния: таблица §6.4 + ссылки на файлы.
4. Взаимодействия: выбор участка (список↔карта↔карточка), событие↔график, точка→подсказка, период→запуск, удаление.
5. Адаптивность: правила перестройки 1280+/1024/<1024; мобильные вкладки; что открывается снизу.
6. Строки интерфейса: src/lib/labels.ts — источник истины.
7. Правила данных: null≠0; наблюдения/восстановление/пропуск; фон только при наличии; цвет полигона = период результата; frontend не считает.
8. Контракт: маршруты и поля, на которые опираются адаптеры; нерешённые детали и владелец каждой.
9. Вне P0: текстовый поиск места, экспорт, сравнение, растры.
```

---

## 12. Что нужно от коллег и когда (положи в общий чат)

| Час | От кого | Что |
|---|---|---|
| 1 | B3 | подтверждение маршрутов §4, формат ошибок RFC7807, где лежат `limits` (config или 422) |
| 2 | B3 + ML | четыре согласованных примера ответов (normal / candidate / confirmed / insufficient_data) + задача + ошибка, минимум один с пропусками; подпись `_synthetic` |
| 2 | B4 | точные ключи стадий задачи для `STAGE_LABEL` и способ доставки (polling или SSE) |
| 4 | B1 | семантика `date` (UTC, интервал), агрегация погоды, текст `spatialNote`, что такое `quality` |
| 4 | ML | список значений `severity` и их русские подписи; текст `criteria` детектора; форма `deviation` (единица и база) |
| 10 | B3 | живой `/openapi.json` |

Если что-то из этого не пришло вовремя — фронт продолжает на моках, а поле в UI показывается как «Нет данных», а не выдуманным значением. Это прямое требование брифа и одновременно то, что защитит вас на демо.