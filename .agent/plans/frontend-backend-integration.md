# Интеграция frontend с актуальным backend

**Статус:** in_progress
**Backend baseline:** `main` / `da9063c`
**Frontend baseline:** `origin/frontend` / `6daeadf`
**Ответственные:** frontend — `Prosteyshiyyy`; публичный API — `tsuckermandev`; источники — `semennejo`; composition root и поставка — `globalarray`; ML — `xsqclown`.

План рассчитан на интеграцию уже написанного frontend с текущим Go backend. Общий порядок этапа, два независимых review, апрув, Conventional Commit и push описаны в [общих инструкциях](../instructions.md). Канонической стороной публичного контракта считается фактический backend в `main`: его domain, handler и repository уже собраны и покрыты тестами. Frontend меняет transport types, adapters и mocks под этот контракт. Backend расширяется только там, где для пользовательского сценария действительно не хватает route или данных.

## Текущее состояние

На текущей интеграционной ветке выполнены FI-01–FI-03 в объёме контракта и composition root:

- frontend materialized in the monorepo, adapters/mocks aligned with canonical backend JSON;
- backend routes `GET /api/areas/{id}`, `GET /api/config` and `?version=` for series/events added;
- B1 source factory, collector, contour finder and queue wiring connected in `internal/app`;
- placeholders removed from the Go application composition root;
- frontend checks (typecheck, lint, Vitest, production build) and backend compile/focused tests pass.

FI-04 remains blocked on the real `backend/ml` service: the repository currently contains only its Dockerfile.
Для промежуточной проверки добавлен `deploy/ml-stub` и Compose-профиль `dev-ml-stub`; он проверяет полный Go/PG/B1/frontend-контур по ML HTTP-контракту, не подменяя production-сервис.

- Ветка `origin/frontend` отстаёт от `origin/main`, но пробный трёхсторонний merge не показывает текстовых конфликтов.
- Frontend на Node из `frontend/.nvmrc` проходит typecheck, lint, 50 Vitest-тестов и production build.
- Mock-режим и живой API используют единые adapters; продуктовые карточка, series/events, polling и запуск анализа подключены к backend-контракту.
- Production использует same-origin: корневой Dockerfile собирает `frontend/dist`, Go отдаёт его из `/app/public`, а браузер обращается только к `/api/*`.
- В локальном Vite-режиме `/api` и `/readyz` проксируются на Go `127.0.0.1:8080`.
- В `internal/app` подключён B1 collector; отдельный поиск контуров остаётся границей B1 и проверяется собственным сценарием.
- Полного Python ML-сервиса в `backend/ml` пока нет, поэтому сквозной анализ до результата зависит от поставки `xsqclown`.
- Реализованная часть интерфейса соответствует FI-05A локально: карта, рисование, добавление участка, карточка результата, график/события и polling подключены к живому API. Перед публикацией остаётся повторить production smoke после деплоя и отдельно обработать подтверждённые повреждённые legacy-записи.

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

- Принимать `shown_result`, `source.kind`, `source.contour_id`, `source.provider`, необязательный `source.crop_type`, `period` и краткий `active_job:{job_id,status,stage}`.
- Не переиспользовать полный job adapter для `active_job`; сделать отдельный узкий adapter.
- Для нарисованной области отправлять `source:{kind:"drawn"}`. Для выбранного контура отправлять `source:{kind:"contour",contour_id,provider}`.
- Передавать заполненную культуру как `source.crop_type`; пустое значение не отправлять.
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

- Принимать top-level series envelope: `area_id`, `result_version`, `period`, `computed_at`, версии, `method`, `status`, `severity`, `series`, `weather`, `provenance`, `limitations`, а также `usable_count`, `imputed_count` и `missing_count` для объяснения результата.
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

До поставки настоящего Python-сервиса этот этап можно принимать через `dev-ml-stub`: stub эхо-возвращает идентификаторы запроса и строит минимальный валидный ряд, а режимы `busy`, `timeout` и `invalid` покрывают ошибки ML-клиента.

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

## FI-05A — недочёты production flow по Chrome DevTools

**Статус:** awaiting_approval
**Аудит:** 05.09.2026, `http://benzomind.tech:8080`, baseline `8309810`
**Исполнители:** frontend — `Prosteyshiyyy`; состояние результатов и повреждённые записи — `tsuckermandev` совместно с `globalarray`.

**Локальная реализация:** FI-05A.1–FI-05A.4 и FI-05A.6 выполнены; FI-05A.5 закрывает валидацию новых полигонов и regression tests. Production cleanup выполнен 05.09.2026 после временного backup: удалены только ID `b4f765a1-e41f-43dc-b4f5-e31fce7f38dd` и `cbe5feee-886c-42db-a857-bc88402f6467`, API после очистки возвращает 9 участков. Временный backup удалён по запросу после проверки.

Проверка выполнена в изолированном Chrome через DevTools Protocol на desktop viewport 1440×900. Создание и удаление участков не выполнялись. Production работает без авторизации, cookies и CSRF; frontend и API используют один origin, поэтому CORS и preflight в проверенных сценариях не участвуют.

### FI-05A.1 — сохранять маршрут панели при выборе участка — P0

**Факт:** переход с лендинга по ссылке «Платформа» корректно открывает `/panel.html`, но выбор «Участок 7» меняет URL на `/?area=<id>`. XHR/fetch при самом выборе отсутствуют. После обновления этого URL сервер отдаёт лендинг с заголовком «Нарисуйте участок. Остальное сделает спутник.», и пользователь теряет панель.

- В `frontend/src/app/router.tsx` не навигировать безусловно на `/`; search-параметры `area`, `event`, `date`, `mock` и `dev` должны обновляться на маршруте `/panel.html`.
- Добавить router test: открыть `/panel.html`, выбрать участок, проверить `/panel.html?area=<id>`, обновить страницу и получить React-панель с тем же выбранным участком.
- Проверить тот же сценарий для выбора участка на карте и для demo URL с `mock=1`.

**Готово, если:** любое изменение selection сохраняет `/panel.html`, а refresh/deep link не возвращает пользователя на лендинг.

### FI-05A.2 — подключить выбранный участок к карточке и результатам — P0

**Факт:** production UI загружает только `GET /api/areas` и `GET /api/config`. После выбора участка не выполняются `GET /api/areas/{id}`, `GET /api/areas/{id}/series` и `GET /api/areas/{id}/events`; карточка, график и погода остаются статическими заглушками. `useArea` и `useResultBundle` существуют, но в production-компонентах не используются; полный bundle сейчас показывается только на dev `StatesPage`.

- Заменить статические `CardPanel`, `ChartPlaceholder` и `WeatherPlaceholder` в `frontend/src/app/AppShell.tsx` продуктовыми компонентами на `useArea` и `useResultBundle`.
- Читать series и events одной `result_version`, показывать loading, empty, insufficient data, failed и version mismatch без вымышленных значений.
- Для выбранного участка с `shown_result` запросить area/series/events и отобразить статус, NDVI, погоду, события, ограничения и provenance.
- На desktop, tablet и mobile проверить, что выбор из списка и с карты приводит к одинаковому набору read-only запросов и состоянию UI.

**Готово, если:** выбор проанализированного участка вызывает три ожидаемых GET и показывает сохранённый результат; участок без результата получает явное empty-state без запросов на запуск анализа.

### FI-05A.3 — реализовать запуск анализа из header — P0

**Факт:** после выбора участка кнопка «Запустить анализ» становится активной, но у неё нет обработчика. DevTools не фиксирует `POST /api/areas/{id}/analyses`; выбор периода остаётся disabled-заглушкой.

- Подключить выбор периода из участка или `PeriodPicker` с лимитами `GET /api/config`.
- Подключить `useStartAnalysis`; отправлять `POST /api/areas/{id}/analyses` ровно один раз с `{period:{from,to}}`.
- После `202 {job_id}` показывать job stage, опрашивать `GET /api/jobs/{id}` и после terminal status обновлять area/list/result bundle.
- На `409`, `422`, `429`, source failure и ML failure показывать нормализованную ошибку; не повторять POST автоматически.
- Пока обработчик не подключён, кнопка не должна выглядеть рабочей.

**Готово, если:** клик создаёт одну job, UI показывает фактический прогресс и после завершения читает опубликованную версию результата.

### FI-05A.4 — разобраться с отсутствующим результатом «Участка 3» — P1

**Факт:** надпись «Не анализировался» соответствует текущему backend: `GET /api/areas/91757111-7cf3-4fad-95e7-d145404db588` возвращает `shown_result:null` и `active_job:null`; series и events отвечают `200`, но содержат `result_version:null` и пустые массивы. Если анализ этого участка ранее запускался, его terminal state не опубликован как shown result.

- По PostgreSQL job history определить последнюю job участка и её terminal error; не менять frontend-текст до подтверждения backend-расхождения.
- Для completed job проверить транзакцию сохранения `AnalysisRecord`, публикацию `shown_result_version` и очистку `active_job_id`.
- Для failed/cancelled job показывать последний известный статус/ошибку отдельно от «Не анализировался», если такая история доступна публичному контракту.
- Добавить repository/handler integration test: completed job делает результат видимым в list/get/series/events после restart.

**Готово, если:** статус списка объясняется сохранённой job history, а completed job не может остаться с `shown_result:null`.

### FI-05A.5 — обработать повреждённые legacy-полигоны — P1

**Факт:** первые две production-записи «Участок 1» содержат кольца из четырёх одинаковых координат. Они проходят через `GET /api/areas`, хотя не образуют полигон, и могут ломать fitBounds, карту или последующий анализ.

- Проверить, что актуальный `POST /api/areas` отклоняет кольцо после удаления дублей, если остаётся меньше трёх уникальных вершин, и не сохраняет запись частично.
- Добавить repository/domain regression test на закрытое кольцо, последовательные дубли, самопересечение и вырожденную площадь.
- Перед очисткой production сделать backup и определить точные ID повреждённых записей; затем удалить либо пометить их недоступными согласованным способом. Выполнено: backup был создан на production, затем оба подтверждённых ID удалены через штатный API; временный backup удалён после проверки.
- Не фильтровать произвольные adapter errors молча: повреждённая запись должна быть видна в логах/метриках и не скрывать все валидные участки.

**Готово, если:** новые вырожденные полигоны не сохраняются, а legacy-записи больше не попадают в пользовательский flow. Проверено после очистки: в `/api/areas` больше нет двух вырожденных записей.

### FI-05A.6 — показывать ошибку загрузки списка — P1

**Факт:** `AreasPanel` использует только `useAreas().data?.length`; при сетевой ошибке или ошибке adapter это превращается в состояние «нет участков». Пользователь не отличает пустой список от поломанного API/JSON.

- Развести `isPending`, `isError`, пустой успешный ответ и заполненный список.
- Для ошибки показать локализованное сообщение и retry; в диагностике сохранить HTTP status/code без вывода внутренних деталей пользователю.
- Добавить component tests для network error, 500, malformed JSON и успешного `{areas:[]}`.

**Готово, если:** сбой `GET /api/areas` не маскируется под отсутствие участков и может быть повторён без перезагрузки страницы.

### FI-05A.7 — не терять рисование после смены подложки — P0

**Факт:** `MapLibre setStyle` удаляет source/layer, зарегистрированные Terra Draw. После переключения «Карта → Спутник» экземпляр мог зарегистрироваться на старом style, поэтому кнопка карандаша включала режим без рабочих слоёв.

- Пересоздавать Terra Draw на том же экземпляре карты после `style.load` нового basemap.
- Не выставлять режим и не вызывать `clear` на экземпляре, который уже снят при смене style.
- Проверить сценарий `Карта → Спутник → карандаш → pointer/touch drawing` на desktop и mobile.

**Готово, если:** после переключения подложки карандаш снова принимает pointer/touch input и показывает черновик полигона.

### FI-05A.8 — прогрев scroll-scrub видео на мобильных — P1

**Факт:** мобильный браузер может не активировать декодер при одном изменении `video.currentTime`, поэтому спутниковая анимация лендинга оставалась на poster.

- Запускать muted `play()` с немедленной паузой после загрузки метаданных.
- Повторять прогрев на первом `touchstart`, если autoplay был заблокирован.
- Сохранить scroll-scrub и ветку `prefers-reduced-motion` без изменений.

**Готово, если:** на мобильном viewport после открытия лендинга и первого касания спутниковая анимация обновляется при прокрутке.

### Подтверждённые исправные границы

- `GET /panel.html`, `GET /api/config` и `GET /api/areas` отвечают `200`; production API base URL корректный и same-origin.
- Поиск контуров для bbox `59.93582654954042,57.01556393128024,59.95213438035111,57.02028307777823` вернул `200 {"contours":[]}`. UI правильно показал «Контуры не найдены в этой области»; пустой результат не считать provider failure.
- На чистой повторной загрузке панели постоянные browser console errors не воспроизвелись. Проверку консоли и всех non-2xx ресурсов повторить после реализации FI-05A.1–FI-05A.6.

## FI-06 — сквозная приёмка и поставка

**Исполнитель:** `globalarray`; участвуют все владельцы по своим модулям.

1. После закрытия FI-05A добавить воспроизводимый browser smoke: лендинг → `/panel.html` → выбор/refresh с сохранением маршрута → загрузка карточки → создание drawn/contour area → один POST analysis → polling → обновление area → series/events одной версии → удаление.
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
