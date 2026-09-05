# Интеграция frontend с актуальным backend

**Статус:** planned
**Backend baseline:** `main` / `da9063c`
**Frontend baseline:** `origin/frontend` / `6daeadf`
**Ответственные:** frontend — `Prosteyshiyyy`; публичный API — `tsuckermandev`; источники — `semennejo`; composition root и поставка — `globalarray`; ML — `xsqclown`.

План рассчитан на интеграцию уже написанного frontend с текущим Go backend. Общий порядок этапа, два независимых review, апрув, Conventional Commit и push описаны в [общих инструкциях](../instructions.md). Канонической стороной публичного контракта считается фактический backend в `main`: его domain, handler и repository уже собраны и покрыты тестами. Frontend меняет transport types, adapters и mocks под этот контракт. Backend расширяется только там, где для пользовательского сценария действительно не хватает route или данных.

## Текущее состояние

- Ветка `origin/frontend` отстаёт от `origin/main`, но пробный трёхсторонний merge не показывает текстовых конфликтов.
- Frontend на Node `22.23.2` проходит typecheck, lint, 61 Vitest-тест и production build.
- Mock-режим frontend внутренне согласован, но живой API не подключается из-за различий JSON-контрактов.
- Production использует same-origin: корневой Dockerfile собирает `frontend/dist`, Go отдаёт его из `/app/public`, а браузер обращается только к `/api/*`.
- В локальном Vite-режиме proxy на Go отсутствует.
- В `internal/app` пока подключены `placeholderCollector` и `placeholderContours`; реализации B1 существуют, но не собраны в application composition root.
- Полного Python ML-сервиса в `backend/ml` пока нет, поэтому сквозной анализ до результата зависит от поставки `xsqclown`.
- Реализованная часть интерфейса в основном соответствует FE-3: карта, рисование, добавление участка и базовый polling. Полноценные карточки, графики и события ещё нужно завершить.

## Порядок и параллельность

```text
FI-00: merge frontend с актуальным main и baseline
  ├── FI-01: frontend adapters, mocks, dev proxy — Prosteyshiyyy
  ├── FI-02: недостающие публичные routes/version — tsuckermandev
  └── FI-03: B1 adapters и app wiring — semennejo + globalarray
          └── FI-04: реальный ML и полный analysis pipeline — xsqclown + globalarray

FI-05: продуктовые экраны стартуют после стабильных adapters FI-01,
       live-приёмка требует FI-02…FI-04
FI-06: общий E2E, CI и deploy после FI-01…FI-05
```

После FI-00 три направления FI-01, FI-02 и FI-03 выполняются параллельно в отдельных ветках/checkout. Общими JSON-полями владеет B3; frontend не меняет их самостоятельно, а B1/B4 не добавляют transport DTO в domain. FI-04 не блокирует синхронизацию контрактов и сборку экранов на точных backend fixtures.

## Решения по границам

1. React-компоненты получают только frontend view models. Форму backend JSON знают `src/api/types.ts` и `src/api/adapters/*`.
2. Frontend adapters выполняют только проверку и преобразование представления. Они не рассчитывают аномалии, фон, severity или причинность.
3. Смысл `observed`, `imputed` и `missing` сохраняется. Для графика используется backend `value`, а исходное измерение остаётся в `primary_ndvi`.
4. Отсутствующие backend-поля не заполняются вымышленными значениями. UI показывает доступные данные или явное состояние отсутствия.
5. Для MVP принимается только GeoJSON `Polygon`, потому что backend не принимает `MultiPolygon`.
6. Ошибки backend имеют envelope `{error:{code,message,retryable}}`; frontend нормализует его в `AppError`, сохраняя исходный `code`.
7. Результат series и events читается одной согласованной `result_version`. Backend начинает учитывать параметр `?version=`, frontend передаёт версию из завершённой job или `shown_result`.
8. Для локальной разработки Vite проксирует `/api` и `/readyz` на Go. CORS для этого не добавляется.
9. Временные placeholders удаляются после подключения реальных B1 adapters. Отдельный второй API или frontend backend-for-frontend не создаётся.

## FI-00 — интеграционная ветка и baseline

**Исполнитель:** `globalarray` совместно с `Prosteyshiyyy`
**Вход:** актуальные refs `main` и `origin/frontend`.

1. Создать рабочую ветку от актуального `main` и влить `origin/frontend` без переписывания истории frontend.
2. Не включать в интеграционный diff локальные `.idea/` и незавершённые изменения `.agent/plans/backend-4-cicd-integration.md`.
3. Зафиксировать Node из `frontend/.nvmrc`, выполнить `npm ci`, typecheck, lint, tests и build.
4. Из `backend/` выполнить `gofmt -l .`, `go vet ./...` и `go test -race ./...`.
5. Сохранить один воспроизводимый пример ответа каждого backend route как основу для frontend adapter tests.

**Готово, если:** frontend и backend находятся в одной ветке от актуального `main`, обе части проходят исходные проверки, а дальнейший diff не смешан с посторонними локальными файлами.

## FI-01 — синхронизация frontend transport contract

**Исполнитель:** `Prosteyshiyyy`
**Файлы:** `frontend/src/api/`, `frontend/src/lib/job-status.ts`, `frontend/src/lib/series.ts`, `frontend/tests/`, mock fixtures.

### Ошибки

- Научить `normalizeError` разбирать `{error:{code,message,retryable}}`.
- Сохранить backend-коды `queue_full`, `conflict`, `not_found`, `invalid_json`, `invalid_geometry`, `invalid_bbox`, `invalid_period`, `invalid_name`, `invalid_source`, `limit_exceeded`, `source_unavailable` и `internal_error`; UI-текст может локализоваться отдельно.
- Проверить 400, 404, 409, 429 и 500 отдельными adapter cases.

### Areas и create request

- Принимать `shown_result`, `source.kind`, `source.contour_id`, `source.provider`, `period` и краткий `active_job:{job_id,status,stage}`.
- Не переиспользовать полный job adapter для `active_job`; сделать отдельный узкий adapter.
- Для нарисованной области отправлять `source:{kind:"drawn"}`. Для выбранного контура отправлять `source:{kind:"contour",contour_id,provider}`.
- Удалить обязательность несуществующих backend-полей `source.label`, `source.external_id` и `last_result`.
- Не повторять автоматически POST создания участка после ошибки adapter: успешный ответ backend не должен приводить к дубликату.

### Contours

- Принимать `{contours:[{id,geometry,source:{provider,attribution}}]}`.
- Преобразовать пустой `contours` в состояние `empty`; HTTP/provider error — в `failed`.
- До расширения backend использовать `id` как отображаемый fallback. Имя контура добавлять отдельным согласованным полем, если B1 уже его возвращает на domain boundary.
- Передавать фактическую ошибку запроса в `ContoursButton`, а не только синтетический mock-status.

### Jobs

- Принимать `period`, вложенный `error`, `result_version` и backend `id`.
- Поддержать стадии `collect_satellite`, `collect_weather`, `prepare_input`, `analyze`, `save_result` и человекочитаемые подписи для неизвестной будущей стадии.
- После `completed` инвалидировать area/list и загружать series/events с одной `result_version`; после `failed/cancelled` останавливать polling.

### Series, weather и events

- Принимать top-level series envelope: `area_id`, `result_version`, `period`, `computed_at`, версии, `method`, `status`, `severity`, `series`, `weather`, `provenance`, `limitations`.
- Точка графика: `value` — отображаемое значение; `primary_ndvi` — исходное наблюдение; `state` — источник отображения; `baseline`, `z_score`, `interval`, `valid_fraction` остаются nullable.
- Погода приходит выровненным массивом с `temperature_mean_c`, `precipitation_sum_mm`, `source_id`; frontend не ожидает вложенные отдельные ряды.
- Событие принимает `start_date`, `end_date`, `status`, `severity`, `min_z_score`, `evidence_dates`, `facts`, `hypothesis`, `limitations`.
- Не трактовать `min_z_score` как величину NDVI и не генерировать фиктивный domain id. Для React key использовать стабильную комбинацию `start_date/end_date/index` только на уровне view.

### Геометрия и локальный запуск

- Сузить create/search types до `Polygon`.
- Добавить Vite proxy `/api` и `/readyz` на настраиваемый локальный адрес Go с безопасным default `http://127.0.0.1:8080`.
- Оставить production `VITE_API_URL` пустым для same-origin.

**Проверки:** adapter unit tests на реальные backend JSON; MSW handlers используют ту же форму; typecheck, lint, Vitest и build проходят.

**Готово, если:** список/создание/удаление участков, контуры, job polling, series и events читают фактические backend-ответы без исключений adapter и без вымышленных полей.

## FI-02 — минимальное дополнение публичного API

**Исполнитель:** `tsuckermandev`
**Файлы:** `backend/internal/handler/`, узкие service/repository ports только при необходимости.

1. Добавить `GET /api/areas/{id}` через существующий `area.Service.GetArea` и ту же `publicArea` projection, что использует список.
2. Прочитать `?version=` в series/events. При непустом параметре валидировать версию и вызвать существующий `GetResult(areaID, version)`; без параметра сохранить поведение `shown_result`.
3. Добавить `GET /api/config` с публичными лимитами площади, вершин и периода, которые реально применяет handler. Не выдавать секреты или внутренние адреса.
4. Если B1 domain уже предоставляет имя/метаданные поиска, сохранить их в публичной contour projection. Не вводить отдельный frontend-only каталог.
5. Для `version`, не принадлежащей area, вернуть согласованный 404; повреждённый сохранённый результат остаётся 500.
6. Привести создаваемые backend error messages к английскому и вынести повторяемые коды/сообщения в `errors.go`; русский текст формирует frontend localization layer.

**Проверки:** handler tests для GET area, config, default shown version, explicit version, чужой/несуществующей версии и прежних status/error envelopes; проверка английских error messages и сохранения machine-readable code.

**Готово, если:** frontend не использует 404-fallback для обязательных данных, а series/events гарантированно относятся к одной выбранной версии результата.

## FI-03 — подключение B1 в composition root

**Исполнители:** `semennejo` — source adapters; `globalarray` — `internal/app` wiring.

1. В `internal/app` собрать source factory из `config.Source` и общих limits.
2. Адаптировать domain contour finder B1 к consumer-owned `handler.ContourFinder`, сохранив геометрию, provider, attribution и доступное имя.
3. Адаптировать `service/source.Collector` и `AnalyzeRequestBuilder` к `analysis.Collector`: вход — `domain.Area`/`domain.Job`, выход — канонический запрос ML и provenance.
4. Подключить CDSE, Open-Meteo и Overpass через `internal/integration/*`; endpoints и credentials брать из config, не хардкодить в wiring.
5. Удалить `placeholderCollector` и `placeholderContours` после зелёных integration tests. Provider-specific DTO не передавать в handler или analysis domain.
6. Задокументировать обязательные и необязательные env для локального запуска и Compose.

**Проверки:** app wiring test с fake HTTP providers, B1 service/integration tests и ручной live smoke при доступных credentials.

**Готово, если:** поиск контуров и сбор входа анализа идут через реальные B1 реализации, а production app больше не использует placeholders.

## FI-04 — ML и сквозной анализ

**Исполнители:** `xsqclown` — Python-сервис; `globalarray` — Go wiring/Compose.

1. Поставить реальный `backend/ml` пакет, зависимости, `/readyz` и `/v1/analyze` по `.agent/contracts/go-ml-http.md`.
2. Подключить существующий Go ML client к analysis executor и общей очереди.
3. Обеспечить сохранение `AnalysisRecord` в PostgreSQL и публикацию `result_version` только после успешной проверки ответа ML.
4. Обновить Compose и release manifest так, чтобы Go запускался с совместимой версией ML и модели.
5. Проверить ошибку источника, ML unavailable/timeout/busy, insufficient data и успешный результат без автоматического повторения POST.

**Готово, если:** новый полигон проходит `frontend → Go → B1 → ML → PostgreSQL → Go → frontend`, а job и пользовательский интерфейс показывают фактические стадии и результат.

## FI-05 — завершение продуктовых экранов

**Исполнитель:** `Prosteyshiyyy`
**Зависимость:** стабильные adapters FI-01; для live acceptance — FI-02…04.

1. Завершить карточку участка и удаление с явным состоянием активной job.
2. Построить NDVI-график с разными стилями `observed`, `imputed`, `missing`, baseline и quality; null не соединять как измерение.
3. Добавить погодные ряды с единицами и общей временной осью.
4. Показать события: период, статус, severity, факты, гипотезу и ограничения без заявления причинности.
5. Реализовать loading, empty, partial, insufficient data, failed и stale/version states.
6. Проверить основные действия и просмотр результата на desktop, tablet и mobile; рисование проверить реальным touch/pointer input.

**Готово, если:** пользователь может создать или выбрать область, запустить анализ, дождаться результата и понять состояние растительности, данные, основания и ограничения на ноутбуке и телефоне.

## FI-06 — сквозная приёмка и поставка

**Исполнитель:** `globalarray`; участвуют все владельцы по своим модулям.

1. Добавить воспроизводимый browser smoke: загрузка карты → создание drawn/contour area → один POST analysis → polling → обновление area → series/events одной версии → удаление.
2. Проверить, что повторный render/retry frontend не создаёт второй area или analysis job.
3. Запустить frontend gates, Go gates, ML checks, PostgreSQL integration tests и обе Docker-сборки.
4. Проверить Compose same-origin, `/readyz`, сохранение PostgreSQL после restart и доступ с мобильного viewport.
5. После merge в `main` проверить GitHub Actions, публикацию обоих образов в GHCR и auto deploy на self-hosted runner.

**Готово, если:** сквозной сценарий воспроизводится из чистого checkout и после deploy, состояние сохраняется после restart, а UI не зависит от MSW.

## Минимальная демонстрация

- Карта открывается с корректным пустым состоянием.
- Пользователь находит контур или рисует Polygon и создаёт участок ровно один раз.
- Анализ запускается, frontend показывает реальные стадии и различает retryable/terminal errors.
- После завершения area, series и events читаются из одной `result_version`.
- График различает наблюдения, восстановление и пропуски; события показывают факты, гипотезу и ограничения.
- Основной сценарий работает на ноутбуке и телефоне.
- Production использует один origin через Go, PostgreSQL и реальный B1/ML wiring.

## Отложено до живого сценария FI-04

- Разбиение большого frontend bundle и lazy loading карт/графиков.
- Обновление ECharts до `6.1.0` после проверки совместимости: текущий production audit сообщает одну moderate уязвимость.
- Hardening под Node 26; CI и `.nvmrc` сейчас используют Node 22.
- Очистка jsdom warning `window.scrollTo` и перенос одноразового `scripts/fe3-check.mjs` в нормальный Playwright suite выполняются вместе с FI-06, если не мешают живой интеграции.

Эти пункты не блокируют первое живое подключение. Сначала закрываются FI-00…FI-04 и минимальный пользовательский сценарий; визуальные оптимизации не должны задержать интеграцию.
