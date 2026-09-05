# Frontend — фактический статус handoff

> На момент ориентации в `HEAD e8f9da8` этого файла не было. Файл создан 05.09.2026
> по фактическому коду ветки `frontend/fe-0`; прежние записи статуса не удалялись и не
> переписывались, потому что их в checkout нет.

## FE-3 — карта, контуры, рисование, добавление

**Фактическое состояние после FE-3 hardening:** основной код этапа присутствует; mock job
lifecycle замкнут, точка входа рисования доступна из списка и карты. Typecheck, lint,
61 unit/integration-тест Vitest и production build проходят. Формальных Playwright-тестов
в `e2e/` нет, поэтому DoD подтверждён частично.

### Файлы и реализованное поведение

- `src/features/map/MapView.tsx` — MapLibre-карта; Positron, fallback OpenFreeMap и
  переключаемая Esri World Imagery; найденные контуры, сохранённые участки, отдельная
  нейтральная обводка участка без результата, выбранный участок с белым гало и тёмной
  линией; сохранение позиции карты; поиск по текущему bbox; выбор контура/участка;
  подключение рисования и диалога добавления.
- `src/features/map/map-style.ts`, `BasemapSwitcher.tsx` — стили подложек, стартовый вид
  (центр РФ либо Краснодарский край в mock), сохранение выбранной подложки и позиции в
  `localStorage`.
- `src/features/map/ContoursButton.tsx` — состояния `idle`, `loading`, `empty`, `failed`,
  `ok`, `stale`; число найденных контуров и `coverageNote`; переход к рисованию.
- `src/features/map/ContourPopover.tsx` — карточка у точки клика на desktop и Drawer на
  mobile; имя/ID, источник, площадь, действие добавления.
- `src/features/map/useTerraDraw.ts`, `DrawToolbar.tsx` — freehand-обведение непрерывным
  pointer/touch-жестом через `TerraDrawFreehandMode` с smoothing; контур принимается только
  если пользователь сам вернулся к началу, незамкнутый stroke показывает предупреждение.
  Polygon-by-points больше не используется.
- `src/lib/geo.ts` — замыкание кольца, площадь в гектарах, самопересечение, минимум три
  вершины, ограничения площади/вершин/периода только из `useLimits`; проверка порядка дат.
- `src/features/map/AddAreaDialog.tsx` — название, период, read-only источник и площадь;
  клиентские ошибки названия/периода, общая серверная ошибка, pending-состояние;
  «Только сохранить» и «Добавить и проанализировать»; после ошибки запуска сохранённый
  участок помечается в `ui.pendingStart` для ручного запуска из списка.
- `src/features/map/MapEmptyHint.tsx`, `MapLegend.tsx` — пустое состояние с двумя
  действиями и сворачиваемая на mobile легенда статусов/контуров/выбора.
- `src/features/areas/AreaList.tsx` — временная интеграция FE-3: загрузка, имя, verdict,
  active job, выделение и ручной повтор запуска после частично успешного добавления.
  Это не полноценные `AreaListItem`/`AreaCard` этапа FE-2.
- `src/store/draft.ts`, `src/store/selection.ts`, `src/store/ui.ts` — черновик рисования,
  единое выделение, подложка и `pendingStart`.
- `src/api/mocks/handlers.ts` и `fixtures/contours-{ok,empty,failed}.json` — три сценария
  контуров; POST участка; 202/429 запуска; синтетическая задача со стадиями; мутабельный
  список участков.

### Что покрыто проверками

- `tests/geo.test.ts`: 14 тестов — замыкание кольца, самопересечение, площадь, минимум и
  лимиты вершин/площади, порядок и длительность периода.
- `tests/adapters.test.ts`: адаптер контуров различает `empty`/`failed`, маппит геометрию
  и `coverage_note`; дополнительно проверены остальные адаптеры и ошибки API.
- `tests/api-mocks.test.tsx`: useAreas, 202/429 запуска, отсутствие `/api/config`, сборка
  result bundle. Компоненты карты, AddAreaDialog и жизненный цикл новой задачи здесь не
  тестируются.
- `scripts/fe3-check.mjs`: одноразовая браузерная проверка desktop/mobile и генерация
  скриншотов. Она не подключена к `npm run test`/Playwright runner, использует жёсткие
  пути `/usr/bin/chromium` и `/home/prost/...`, а mobile-действия выполняет через
  `page.mouse`. Проверки статуса ищут любой текст «Выполняется»/«Возможное изменение» и
  не связывают его с ID только что созданного участка.
- `docs/screens/fe3-*.png`: сохранены карта 1440/390, добавление, рисование, список после
  сохранения, satellite, empty и failed.

### Расхождения `ui-spec.md` / `frontend-plan.md` / фактического кода

- В `ui-spec.md` для AddAreaDialog указан также несуществующий путь
  `src/features/areas/AddAreaDialog.tsx`; фактический файл —
  `src/features/map/AddAreaDialog.tsx`.
- Формулировка `ui-spec.md` «диалог добавления (202/429/422)» теперь поддержана mock
  lifecycle: POST анализа записывает `active_job`, `AreaList` подключает `JobWatcher`, а
  terminal polling инвалидирует area/list. Реальный backend-контракт всё ещё требует
  отдельной интеграционной правки (см. дополнение ниже).
- DoD FE-3 требует Playwright e2e для desktop и mobile. В `e2e/` только `.gitkeep`;
  `scripts/fe3-check.mjs` — непереносимая one-off проверка и не доказывает touch-жесты.
- Для `failed` видна отдельная кнопка повторного запроса с текстом «Повторить поиск
  контуров».
- Ошибка HTTP/422 самого запроса контуров не переводится в визуальное состояние failed:
  `ContoursButton` получает только `data`, `isFetching` и синтетический `status:'failed'`,
  но не `contoursQuery.error`.
- По §8 клик по карте не должен делать `fitBounds`. `selectionActions.selectArea(...,
  'map')` записывает источник, но последующий `SearchSync.setFromSearch()` выставляет
  `selectionSource:'list'`; это поведение не покрыто тестом и по чтению кода нарушает
  различие map/list.
- Атрибуция остаётся custom-control, но сокращена до одной строки без дублирования
  OSM/CARTO; легенда скрывается во время рисования, чтобы не перекрывать toolbar.
- Растровый P2-задел удалён из `MapView.tsx`; NDVI не рисуется без подтверждённого URL.
- `area-selected-outline` добавлен в Tailwind-токены; полупрозрачные поверхности по-прежнему
  требуют отдельного визуального QA на целевых браузерах.
- Комментарий `useTerraDraw.ts` обещает пересоздание draw-слоёв после смены подложки, но
  хук зависит от экземпляра map, а не от basemap/style. Сценарий смены подложки во время
  рисования не покрыт.
- `ui-spec.md` корректно помечает FE-2/FE-4/FE-5 как TBD, однако уже утверждает FE-3 как
  «реализован». Точнее считать реализацию присутствующей, а DoD — частично закрытым до
  исправления жизненного цикла job и воспроизводимых e2e.

### Нерешённые стыки, уже следующие из кода

Новых вопросов к backend не добавлено. Действуют только уже записанные в
`ui-spec.md` §8.2: форма ошибок и 202/429, источник лимитов, чтение согласованной версии,
ключи стадий, семантика дат/погоды/quality и источник meta результата.

### Дополнение: фактический backend в сохранённом `origin/main`

Frontend-ветка основана на `ac7eb25`; сохранённый ref `origin/main` (`ee8655b`) содержит
ещё 36 коммитов backend. Read-only сверка через `git show origin/main:...` показывает, что
несколько пунктов §8.2 уже имеют конкретную реализацию, но она не совпадает с FE-1:

- Ошибка backend — `{error:{code,message,retryable}}`. `normalizeError()` не разбирает
  вложенный объект и возвращает общий `code:'error'`/`title:'Ошибка запроса'`; в частности,
  фактический код 429 `queue_full` теряется.
- `POST /api/areas/{id}/analyses` действительно возвращает 202 `{job_id}`; при активной
  задаче backend также может вернуть 409 `conflict`, которого нет в рукописной frontend
  `paths`.
- Ключи стадий уже заданы backend: `collect_satellite`, `collect_weather`,
  `prepare_input`, `analyze`, `save_result`; frontend знает только
  `satellite/weather/prepare/analysis`, поэтому живой UI покажет fallback.
- Backend area использует `source:{kind,contour_id,provider}`, `shown_result` и краткий
  `active_job:{job_id,status,stage}`. Frontend ожидает `source.label/external_id`,
  `last_result` и полный `JobMeta`; `adaptArea()` на фактическом ответе завершится ошибкой.
- Отдельного `GET /api/areas/{id}` в mux нет, хотя `useArea()` и `useResultBundle()` его
  вызывают. `GET /api/config` также отсутствует; fallback `useLimits() -> null` остаётся
  актуален.
- Контуры приходят как `{contours:[{id,geometry,source}]}`; frontend ожидает
  `{status,features,source,coverage_note}` и классифицирует фактический ответ как failed.
- Job приходит с `period` и вложенным `error`; frontend ожидает `requested_period` и
  плоские `error_code/error_message`, поэтому `adaptJob()` не принимает живой ответ.
- Series — один envelope с массивами `series`/`weather` и полями
  `primary_ndvi/value/state/baseline/z_score`; frontend ожидает envelope `{series:{...}}`
  и точки `ndvi/provenance/background/deviation`. Events также имеют другую форму
  (`start_date/end_date/status/facts/...` вместо `period/verdict/detected/basis/weather`).
- Параметр `version` текущими backend handlers не читается: отдаётся `shown_result`.

Следствие: mock-режим FE-1/FE-3 самосогласован, но живое подключение сейчас не работает.
Это подтверждённое расхождение контрактов для FE-7/интеграции, а не новый вопрос к
backend. Перед живым API нужно выбрать каноническую сторону контракта и затем менять
адаптеры/моки либо backend согласованным diff.
