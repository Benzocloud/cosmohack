# B4 — выполнение анализа, интеграция и CI/CD (ответственный: `globalarray`)

План рассчитан на 48 часов. Общий порядок этапов, два независимых ревью Sol,
апрув, commit/push и правила публикации определены в [общих инструкциях](../instructions.md).
B4 владеет `backend/cmd/server/`, `backend/internal/service/analysis/`, `backend/internal/service/ml/`,
общими типами, обоими Dockerfile, CI/CD, `deploy/`, README и сквозной проверкой;
B3 (`tsuckermandev`) владеет API/store, B1 (`semennejo`) — source, ML (`xsqclown`) — Python HTTP-сервис, пакет, артефакты и
исследование, frontend (`Prosteyshiyyy`) — `frontend/`. Контракт B4/ML — [go-ml-http.md](../contracts/go-ml-http.md).

Главный результат B4 — принятая задача проходит сбор данных → ML → сохранение результата с наблюдаемым прогрессом и корректными ошибками. B3 принимает запросы и хранит состояния/результаты; B4 выполняет задачу и сообщает переходы через операции B3. B4 начинает на согласованных примерах и заглушках; готовность остальных модулей, SSH или merge не является условием начала локальной интеграции. Ошибки внутри чужого модуля возвращаются его владельцу с воспроизводимым примером.

## Журнал выполнения

**Старт без ожидания других владельцев:** первым результатом подготовить общий Go-каркас, затем проверять HTTP-клиент своим тестовым HTTP-сервером по опубликованному v1. Для исполнителя использовать заглушки источника/хранилища в своей тестовой зоне. Подключить настоящий Python-сервис по его готовности. Готовые B1, B3, ML и frontend не являются входом для этих локальных проверок; общие изменения и локальную детализацию вести по инструкциям.

Для каждого ID фиксировать статус, базовый SHA, настоящий nickname и ветку,
проверки, оба отчёта Sol и исправления, личное ревью, апрув, SHA/push,
послепубликационные результаты и blockers. Все этапы изначально planned.

## B4-01 — каркас выполнения, контракты и HTTP-клиент ML (0–2 ч)

**Статус:** published — **B4-01 опубликован:** commit `62a7e57` `feat: add backend server and ML client`
в ветке `globalarray/feat/b4_cicd_integration` от базы `ac7eb25`; commit и push выполнены
владельцем (`globalarray`) после апрува отчёта R2, push подтверждён `git ls-remote`
(origin = `62a7e57`); опубликованное дерево проверено в изолированном worktree:
`go build ./...` + `go test -race ./...` — зелёные. Состав коммита: 33 файла B4
(backend-каркас, фикстуры, пути в 5 документах .agent), чужих файлов нет.
Статус `done` не присваивается: интеграционный критерий (настоящий HTTP-вызов к
Python-заглушке ML-01) не выполнен и зависит от ML; постпубликационные CI-проверки
появятся с B4-02. Следующий этап — B4-02 (образы, Compose, CI, исполнитель на заглушках).
Журнал DDD-рефакторинга и записи R0–R2 вошли в этот же коммит.

**База:** SHA `ac7eb25`, main; ветка `globalarray/feat/b4_cicd_integration` (переименована из `globalarray/task/b4_cicd_integration` в R0).
**Проверки локально:** `gofmt -l` чисто, `go vet ./...`, `go test -race ./...` — зелёные;
smoke-запуск `go run ./cmd/server` с `HTTP_ADDR=:18080`: `GET /readyz` → 200 JSON,
`POST /readyz` → 405, graceful shutdown по SIGTERM.

**Вход:** решения `.agent/plan.md`; контракты и небольшие примеры B1/B3/ML/frontend согласуются в этом этапе, готовые реализации не требуются.

- [x] В первые 0–30 минут локально подготовить один Go-модуль `github.com/Benzocloud/cosmohack/backend`, версию Go и минимальные типы `backend/internal/domain/` по общему плану. Подготовить каркас к передаче B1/B3 по общему циклу, не ждать HTTP-сервиса ML или CI/CD. Затем поднять `backend/cmd/server/` и `/readyz`; команды запускаются из `backend/`.
- [x] Согласовать и закрепить общие типы для геометрии, наблюдений, ключа набора, состояний задачи, происхождения, флагов качества/данных, сезонных периодов и объяснения. Типы закреплены в `domain/` (contract.go, area.go, domain.go); согласование с B1/B3/ML идёт параллельно по общему циклу.
- [ ] Согласовать операции исполнителя с B3 и вызовы источников с B1; зафиксировать один успешный и один ошибочный пример полного прохода, доступные без реальных провайдеров и готового frontend. Примеры прохода через ML зафиксированы в `backend/testdata/ml-http/`; операции исполнителя — B4-02.
- [x] Реализовать клиент `POST /v1/analyze` и `GET /readyz` по [HTTP-контракту](../contracts/go-ml-http.md): конфигурация `ML_BASE_URL`, версии, типы, лимиты, тайм-аут/отмена, проверка ответа и безопасные ошибки. Адрес сервиса не принимать из пользовательского ввода.
- [x] До готовности ML-01 проверять клиент собственным тестовым HTTP-сервером в service/ml по документированному v1; к 2 ч подключить Python-заглушку ML. B4 единолично вносит согласованные общие фикстуры в `backend/testdata/ml-http/`, включая request_id/input_revision, пропуски и несовпадение версий; ML/B1 передают примеры. Go не запускает Python-команду для каждого запроса. Подключение Python-заглушки — блокер: ждёт ML-01.
- [x] Использовать один воркер внутри Go и отдельный ML-сервис без брокера; обучение остаётся отдельной командой, общие Python-функции используются для HTTP и batch. Воркер реализуется в B4-02/03; структура каркаса ему соответствует.
- [x] Зафиксировать манифест выпуска: два digest, schema_version, feature_profile и model_version; артефакт входит в ML-образ либо монтируется только для чтения по версии. Тип `domain.ReleaseManifest` и пример `backend/testdata/release_manifest.example.json`.

**Готово локально:** каркас и HTTP-клиент проходят свой тестовый сервер, неверный JSON/тайм-аут/версия дают ошибку. **Интеграционный критерий:** Go вызывает Python-заглушку ML-01 по настоящему HTTP; заглушка явно помечена и не считается рабочей моделью. Ожидание ML-01 не блокирует локальную часть B4-02/03; полноту приёмки фиксировать отдельно.

**Журнал DDD-рефакторинга среза** ([globalarray/ddd-refactor.md](globalarray/ddd-refactor.md), подэтапы R0–R2, ревью `globalarray` после каждого):

- **R0 выполнено, ждёт ревью.** Baseline: HEAD `ac7eb25`, backend/ не отслеживается git (новые файлы), изменены журнал B4-01; корневой `.gitignore` появился от другого участника — в diff B4 не включается. Файловая карта соответствует плану. Ветка переименована в `globalarray/feat/b4_cicd_integration` по обновлённому правилу имён в instructions.md; checkout моим процессом единственный.
- Гейты R0: `gofmt -l .` — чисто; `go vet ./...` — ок; `go test -race -count=1 ./...` — ok (cmd/server, mlbridge). Blockers: `golangci-lint` установлен, но бинарник несовместим с Go 1.27 (`export data version 4 is greater than maximum supported version 2`) — требуется апгрейд инструмента вне этого рефакторинга; `govulncheck` не найден в PATH. Новых конфигураций ради галочек не добавляем.
- Пути синхронизированы: instructions.md (владение B1/B3/B4, локальные testdata-зоны), plan.md (дерево монорепозитория, блок исполнителя), backend-1-data.md, backend-3-api-storage.md, backend-4-cicd-integration.md. Дерево B1/B3 пусто (код в старых каталогах не создавался), передача ответственности не менялась; владельцам пути сообщаются этим журналом.
- Тесты `client_test.go` зафиксированы как characterization; новые тесты — только для перенесённых HTTP/lifecycle-границ (handler, app).
- **R1.1 выполнено, ждёт ревью.** `internal/mlbridge` → `internal/service/ml`, пакет `ml`; поведение, API клиента (`Config`, `DefaultConfig`, `ConfigFromEnv`, `Client`, `New`, `Ready`, `Analyze`, `Error`), error mapping и cancellation-семантика не менялись. Wire-типы ошибок переведены в приватные `mlErrorBody`/`mlErrorResponse` внутри пакета ml и удалены из `domain/contract.go`; pass-through `NewFromEnv` удалён. Потребитель `cmd/server` переведён на `service/ml.ConfigFromEnv`; старый каталог удалён, shim не оставлен. Путь фикстур в тесте обновлён на `../../../testdata/ml-http`; JSON-фикстуры не менялись (в README фикстур обновлён только путь к клиенту — его checksum в R2 сверяется с новым значением). Проверки: `gofmt -l .` чисто, `go vet ./...` ок, `go test -race -count=1 ./...` ok; `rg 'internal/mlbridge|package mlbridge'` по backend пуст. Линтер-находки golangci-lint v2.13.2 зафиксированы, но исправления отложены по решению владельца.
- **R1.2 выполнено, ждёт ревью.** Создан `internal/handler/ready.go`: приватные `readyResponse`/`handleReady`, экспортированная `Register(*http.ServeMux)` с точным паттерном `GET /readyz`; публичный `/readyz` ML не вызывает. Тест GET (200/JSON/поля) и POST (405) перенесён в `handler/ready_test.go` и проходит через реальный mux с `httptest.NewRecorder`. Из `cmd/server` удалены `readyResponse`, `handleReady`, импорты `encoding/json` и `domain`; `net/http` в cmd остаётся до R1.3. Проверки: `go test -race -count=1 ./internal/handler ./cmd/server` ok; `rg 'handleReady|readyResponse' backend/cmd` пуст; `gofmt`/`go vet`/`go test -race ./...` зелёные.
- **fixes по двум Sol-ревью выполнены (caveman-review + ponytail-review), ждут ревью.** Корректность: `precipitation_sum_mm ≥ 0`; `retrieved_at` строго UTC; входная `usable` точка обязана остаться `observed`; `confirmed`-событие требует непустого `evidence_dates`; `events`/`evidence_dates: null` отклоняются (симметрия с facts/limitations); `Ready` на 503 требует `status=not_ready` (иначе `ml_invalid_response`) и проверяет Content-Type; media type разбирается `mime.ParseMediaType` точно. Тесты: хелпер `mustMLError` (−11 повторов), структурные JSON-мутации, новые кейсы (usable→imputed, candidate/confirmed/normal/insufficient×events, start>end, вне периода, null events/evidence/facts, unusable без reason, отрицательные осадки, retrieved_at не UTC, too many observations на возрастающих датах). Ponytail: удалён мёртвый `errResponseBodyTooLarge`; слиты дублирующие ветки usable/unusable; слит второй проход по events (validateEventRangesInPeriod удалён); упрощён reader в `do`; удалены комментарии-пересказы. **Опровергнуто:** «too many observations бьёт не в свою ветку» — проверка лимита стоит первой строкой `validateObservations`, ветка достигается. **Отложено с обоснованием:** `DisallowUnknownFields` на ответ (дрейф ловится сверкой версии/модели манифеста; решение согласовать с ML); конфликт лимитов 4096 точек vs 1 MiB тела (~1.2 MiB на максимальном запросе) — открытый вопрос к ML-владельцу по правилам изменения лимитов; unused-символы domain (Stage*, InterruptReason, ReleaseManifest, Area/Job/JobStatus) сохранены по решению владельца — их подключают B1/B3/B4-02.
- **R1.3 выполнено, ждёт ревью.** Создан `internal/app` (`Run(ctx)`: `ConfigFromEnv` до слушателя, mux + `handler.Register`, `ReadHeaderTimeout=5s`, graceful shutdown 10s, serve-goroutine дожидается и её ошибка не проглатывается — находка caveman-review закрыта здесь). `cmd/server` истончён до signal context + `app.Run` + лог ошибки; удалены `main_test.go` (тест перенесён в `app_test.go`) и прямые импорты net/http/encoding/json/domain/service/ml. Проверки: `go build ./...` ok; `go vet` app/cmd/domain/ml ok; `go test -race ./internal/app ./internal/domain ./internal/service/ml` ok; smoke: `GET /readyz` 200 JSON, `POST /readyz` 405, SIGTERM → «server stopped gracefully», `ML_BASE_URL=not-a-url` → fail-fast exit 1. Checksums фикстур после fixes/R1.3: все JSON идентичны R0; изменён только README.md (путь клиента, запланировано в R1.1): `1c40889a…2e94dba`.
- **Blocker координации (не B4): РАЗРЕШЁН интеграцией B3-среза.** В общий checkout параллельно лег незавершённый код B3 — `internal/handler/{areas,analyses,series,jobs,mux,project,body,errors,validate,stubs}.go` + тесты и `internal/handler/testdata/`. `areas_test.go:26` ссылался на неопределённый `api` — `go vet ./...` и `go test ./internal/handler` падали на чужом файле. По решению владельца в side-chat поставка B3 интегрирована в DDD-пути (пункт ниже); гейты всего модуля зелёные.
- **R2 выполнено, ждёт апрув B4-01.** Гейты: `gofmt -l .` чисто; `go build ./...` ok; `go vet`/`go test -race` по пакетам B4 (app, cmd/server, domain, service/ml) зелёные; полные `go vet ./...`/`go test ./...` остаются заблокированы тестовым файлом B3 в handler (blocker выше). `golangci-lint` v2.13.2 по пакетам B4: 199 находок отложенного пласта (wsl_v5 141, revive 30, gosec 15 — все в тестах: G304 фикстуры, G104 `w.Write`; в production-коде gosec чист), исправления отложены владельцем. `govulncheck` (установлен в R2): уязвимостей нет. Smoke R2: `GET /readyz` 200 JSON, `POST /readyz` 405, SIGTERM → graceful. Импорт-граф (`go list -deps`): domain → stdlib; service/ml → domain; handler → domain (+ service/store — проводка B3); app → handler, service/ml; cmd → app. Контракт `git diff` пуст; checksums фикстур: 9 JSON идентичны R0, README.md = `1c40889a` (запланированная правка пути).
- **Вторая пара Sol-ревью (caveman + ponytail) по финальному дереву — исправлено.** Корректность: candidate-результат с confirmed-событием отклоняется; тяжесть результата сверяется с максимумом по событиям; `series: null` отклоняется; 503-readiness требует `reason` (константа `domain.MLNotReadyStatus`); Content-Type JSON требуется и на error-пути ML; `DisableCompression: true` («без сжатия в v1»); при провале Shutdown вызывается `srv.Close()`. Тесты: foreign request_id/schema_version в error-конверте, candidate+confirmed, severity ниже максимума, null series, ready charset/text-plain/503 без reason, lifecycle `RunGracefulShutdown` (ephemeral порт + cancel). Ponytail: удалён комментарий-урок в url.go; `validatePeriod` без параметра field (один сайт вызова). **Принято как есть:** повторный сигнал во время shutdown не форсирует выход (известное поведение MVP); `retryable` из тела ML доверяется как есть (решение зафиксировано); сентинелы url.go сохранены по прямому решению владельца; fail-fast `ConfigFromEnv` в Run предписан планом R1.3.
- **Интеграция среза B3-01 (по решению владельца в side-chat, ждёт ревью).** Поставка `tsuckermandev` перенесена в DDD-пути: `internal/api` → `internal/handler` (package `handler`), `internal/store` → `internal/service/store` (package `store`); testdata едут с пакетами. Их `go.mod` (`go 1.22`) не копировался — действует общий `go 1.27.0`. Изменения механические: package-clause, импорт-пути, квалификатор `api.` → `handler.` в тестах; алгоритмы, JSON-контракт и фикстуры B3 не тронуты. Роутер B3 (`NewMux`) в `app` не смонтирован: нужны экземпляры store/queue/contours (заглушки B3 — тестовые); склейка `/api/*` с `Register` в одном mux — B4-02, форму согласовать с B3. Координация с B3: store объявляет собственные Area/Job/Result/SeriesPoint/Event — дублирование общих типов против `internal/domain`, унификация отдельным согласованным diff. Гейты: `gofmt -l .` чисто; `go build ./...`, `go vet ./...` ok; `go test -race -count=1 ./...` ok (app, handler, service/ml, service/store). `golangci-lint run ./...` (v2.13.2, конфиг `backend/.golangci.yml`): 443 находки — 297 wsl_v5, 48 revive, 31 paralleltest, 25 gosec, 10 gocyclo, 10 forcetypeassert, 7 goconst; содержательные: bodyclose 2, errcheck 1, errorlint 1, nilerr 1, copyloopvar 1, nestif 1. Исправления отложены по решению владельца, отдельным проходом.
- Checksums фикстур R0 (`shasum -a 256 backend/testdata/ml-http/*`), для сравнения в R2; README.md обновлён в R1.1 (только путь к клиенту), его новое значение сверяется отдельно:
  - `6abead26…ecc519b` README.md
  - `c3811239…383471c` error_busy.json
  - `9f355bf9…68bf50cee` error_invalid_input.json
  - `447868af…717b5cff` error_unsupported_contract.json
  - `978fef58…9f83c872` readyz_not_ready.json
  - `9047f6f7…854d02f` readyz_ready.json
  - `e638d264…86486b3` request_insufficient.json
  - `44c49be1…15dcc8a5a` request_success.json
  - `fd2d5264…1b19c11` response_insufficient.json
  - `48b0df70…26a251` response_success.json

## B4-02 — образы, первая доставка и проход на заглушках (2–6 ч)

**Статус:** in_progress — локальная часть B4-02 реализована и прошла два
Sol-ревью с исправлениями; ждёт ревью и апрув `globalarray`. Удалённые проверки
(GHCR publish, SSH deploy, merge в main) — внешние зависимости.

**Журнал B4-02:**

- Реализовано: исполнитель `internal/service/analysis` (очередь ≤8, один воркер, стадии через колбэк коллектора, отмена active/queued с немедленной помечкой cancelled, ошибки ML → коды контракта, source_failed для сбора, без позднего сохранения — PutResult store, FailInterrupted при старте); проводка app (store.Open(DATA_DIR), ml.New, executor через адаптер executorQueue → handler.ErrQueueFull = 429, NewMux + Register + serveStatic(PUBLIC_DIR, off при отсутствии), помеченные placeholderCollector/placeholderContours до B1); mux.go → `*http.ServeMux` + слияние NewMux/NewMuxWithLimits (координировано с B3, тест B3 обновлён одной строкой); transport-сообщения ml → английский; Dockerfile (многоэтапный, frontend-фолбэк с пометкой, non-root uid 10001, go.sum- wildcard), .dockerignore, backend/ml/Dockerfile, deploy/compose.yaml (ML_MODEL_VERSION для go, healthcheck ml, порт не публикуется), deploy/deploy.sh (GHCR login по release-ghcr.env 0600, два digest, stop go → ml readiness с JSON-проверкой версий → go readyz + API, mkdir тома под uid 10001, манифест пишется только после успеха, откат предыдущей пары), .github/workflows/pipeline.yml (concurrency, SHA-pinned actions, golangci/govulncheck через go install с зафиксированными версиями, job-условия python/frontend до поставки, publish только main в приватный GHCR с lowercase-репо и packages:write, deploy по SSH с known_hosts и DEPLOY_ENABLED), README.md, .gitignore (data/, artifacts/, submission.csv, deploy/release.env и манифесты). По решению владельца runtime-стадия Go-образа — `scratch` (CGO_ENABLED=0): числовой USER 10001, CA-сертификаты из build-стадии для будущих HTTPS-вызовов B1; итоговый образ 17 MB, контейнерный smoke повторён.
- Две пары замечаний закрыты (caveman + ponytail): 4 блокирующих CI/инфра-находки (setup-python/node на отсутствующих файлах → job-условия; uppercase репо в GHCR-теге → lowercase; root-owned том под uid 10001 → install -d в deploy.sh), дубль сентинела ErrQueueFull (был 500 вместо 429) → адаптер executorQueue, немедленная помечка cancelled при Cancel активной задачи, манифест деплоя только после успеха, JSON-парсинг readiness, ML_MODEL_VERSION в compose, concurrency main, pinned-инструменты; ponytail: ErrNotRunning/base/jobTerminal/StagePrepareInput-write/провenance-guard удалены, Sink-интерфейс → конкретный *store.Store, слияние NewMux.
- Проверки: `gofmt -l .` чисто; `go vet ./...` ok; `go test -race -count=1 ./...` ok (6 пакетов); `bash -n deploy.sh` ok; `docker compose config` ok; workflow YAML валиден (pyyaml); `docker build` Go-образа ok ×2 + контейнерный smoke (readyz 200, статика 200, POST area 201, DELETE 204, graceful stop).
- Blockers: сборка ML-образа и полный `compose up` — ждут поставку ML-01; фактический GHCR publish/SSH deploy — ждут merge в main и доступов (DEPLOY_ENABLED, SSH_*, GHCR_*). Согласование с B3: дубли store-типов против domain — отдельный diff.

**Вход:** локально проверенный каркас B4-01 и согласованные примеры. HTTP-сервис ML и источники сначала доступны как заглушки; до готовности frontend использовать явно обозначенную минимальную страницу.

- [ ] Подготовить корневой многоэтапный Dockerfile для Go и `frontend/dist` в `/app/public`; `backend/ml/Dockerfile` — для Python-сервиса, пакета и артефакта. Обе сборки используют корневой context `.`.
- [ ] Не обучать модель при сборке/старте сервиса; загружать готовую версию один раз на процесс. Первое развёртывание допускает baseline fixture с явной пометкой.
- [ ] С 4-го часа собирать исполнитель: принятая задача → заглушка источников → тестовый HTTP-сервер ML → тестовый получатель результата по операциям B3. Проверить этот проход локально до готовности источников, ML и store; затем подключить реальные модули. Тестовый получатель находится в jobs-тестах и не является вторым производственным хранилищем. Ожидание удалённой доставки не блокирует работу.
- [ ] Подготовить Compose с сервисами Go и `ml`, `ML_BASE_URL=http://ml:8000`, без публикации порта ML. Постоянный том снимков принадлежит Go; ML не получает доступ к нему.
- [ ] Подготовить deploy по двум digest: остановить приём анализов/Go, обновить ML и проверить readiness/версии, затем запустить Go и проверить API. Сохранить предыдущий манифест и реализовать откат совместимой пары с моделью.
- [ ] До deploy проверить фактические SSH fingerprint/key, каталог, порт, registry permissions; отсутствующие доступы записать blocker, scaffold не считать deploy.
- [ ] Реализовать проверки PR/main: тесты Go с race detector, lint, govulncheck, проверку типов и сборку frontend, lint Python, целевые тесты CSV/HTTP-контракта, сборку обоих образов и проверку Go → ML. Публиковать оба образа GHCR только для main через `GITHUB_TOKEN`, выдав `packages:write` только задаче публикации.
- [ ] Перенести в `README.md` команды запуска Go, ML-сервиса, Compose и независимой batch CLI, настройки лимитов, соответствие версий и откат уже на этом этапе (0–6 ч).

**Готово к approval, если:** локальные сборки и проверки, оба образа, Compose и скрипт доставки проверены. После approval выполняются commit/push рабочей ветки, отдельный разрешённый merge в main, зелёный CI, фактическая публикация двух образов GHCR и первый SSH deploy с readiness обеих служб к 6-му часу; после успешного push этап становится published, после проверки обоих digest, версий и доступности стенда — done. При ожидании доступа или merge остаётся published с указанной зависимостью. Автодеплой после зелёного main выполняется разрешённым workflow без отдельного подтверждения каждого deploy.

## B4-03 — исполнитель анализа, прогресс и первая реальная интеграция (6–12 ч)

**Статус:** planned

**Вход:** локальный проход на заглушках из B4-02. Реальные B1 source, B3 API/store, ML и frontend подключаются по готовности; ожидание публикации или SSH не блокирует этап. Реальные модули нужны для приёмки полного сценария, а не для начала разработки исполнителя.

- [ ] Соединить bbox карты и поиск контуров, пользовательский полигон, сбор спутниковых и погодных данных B1, отправку/состояние/результат B3 и карту/график интерфейса; текстовый поиск и растровые слои оставить необязательными.
- [ ] Координировать задачи одним воркером-goroutine и очередью до 8 ожидающих внутри Go; B3 сохраняет состояние. Переполнение — 429 без зависшей задачи. Отменённые задачи — `cancelled`, прерванные queued/running и ошибочные — `failed` с причиной; повторный запуск явный; удаление участка блокирует сохранение позднего ответа.
- [ ] Выполнить последовательность B1 → ML → проверка ответа → сохранение через B3. Сообщать B3 фактические стадии сбора спутников, погоды, подготовки и анализа; подробности стадии показывать только если их сообщает выполняемый модуль.
- [ ] Выполнять синхронный HTTP-вызов ML из воркера; не повторять POST автоматически. Проверить `ml_busy`, тайм-аут, недоступность и неверную схему/версию ответа. Отмена context прекращает ожидание Go, но не обещает остановку Python; занятый ML сохраняет слот до фактического завершения.
- [ ] Проверить основной путь: очищенный исходный и восстановленный primary_ndvi, сезонный фон, периоды аномалий и объяснение, с независимыми флагами/стилем исходных, восстановленных и отсутствующих данных и четырьмя статусами интерфейса.
- [ ] Передать B3 проверенный ответ по общему HTTP-контракту и метаданные B1 для публичного AnalysisResult; веб и batch используют одни функции Python, batch не зависит от работающего HTTP.
- [ ] Принять от ML-владельца независимую неинтерактивную batch CLI: `private_features.csv` → `submission.csv`, конечные числовые значения, уникальные ключи и детерминированная схема; B4 интегрирует, проверяет и документирует её, но не реализует вторую CLI.
- [ ] Запустить до 12 ч первый end-to-end сценарий и зафиксировать, что реально проверено, а что зависит от внешних доступов.

**Готово, если:** новая область и рисование работают автоматически, пакетный запуск воспроизводим отдельно от веба, а реальный и демонстрационный пути имеют проверяемые результаты без ложного успеха.

## B4-04 — реальные модули, устойчивость и staging (12–24 ч)

**Статус:** planned

**Вход:** первый проход B4-03. Доступы к repo/registry/staging требуются для удалённых проверок; подключение и локальная проверка реальных модулей продолжаются независимо от них.

- [ ] Проверить PR checks и main-only публикацию, версии из `backend/go.mod`, `backend/go.sum`, `frontend/package-lock.json`; Docker build всегда с корневым context `.`.
- [ ] Проверить на staging оба digest, `/readyz` Go и ML, совпадение схемы/профиля/модели, реальный запрос данных, сохранность тома, отмену без позднего сохранения, busy после разрыва HTTP, откат пары и повторный запуск; подтвердить доказательство первой доставки B4-02.
- [ ] Убедиться, что frontend не содержит секретов, host уже доступен извне, deploy не перекрывает более новый main, а SSH fingerprint проверяется.
- [ ] Проверить устойчивость и второй реальный регион к 24 ч; historical review не выдавать за live alert.

**Готово, если:** staging реально обновлён и проверен, CI даёт требуемые проверки, данные переживают обновление, откат проверен и два региона имеют доказательства качества. Если внешний доступ отсутствует, этап остаётся незавершённым с явно записанным blocker; локальная работа не выдаётся за actual deploy.

## B4-05 — заморозка и защита воспроизводимости (24–48 ч)

**Статус:** planned

**Локальный вход:** имеющиеся команды/артефакты и результаты локальных проверок B4-04; README, упаковку и сценарий защиты готовить без ожидания staging. Фактические результаты B1/B3/ML/frontend и доступный стенд обязательны для итоговой приёмки, их отсутствие отмечать явно.

- [ ] До 32 ч заморозить scope: только критические fixes; primary UX не блокировать optional `/layers` или текстовым поиском.
- [ ] К 36 ч получить независимо воспроизводимые веб- и batch-точки запуска, отчёт `reports/research.md`, версии артефактов/весов и submission output; ML-владелец поставляет содержание исследования, B4 проверяет упаковку.
- [ ] Проверить презентацию 2–3 временных рядов, ограничения признаков/происхождения, provenance, стили исходных/восстановленных/отсутствующих данных, смысл score и маркировку синтетики.
- [ ] Провести финальный E2E с добавлением/удалением полигона, interruption/failure/retry, двумя реальными регионами и restart persistence.
- [ ] Завершить этап по общему циклу: два Sol-ревью, исправления, личная проверка и апрув. 36–48 ч оставить на репетицию и критические исправления; блокеры записать явно.
- [ ] Не заявлять успех без фактического зелёного CI, опубликованных двух digest и actual deploy; при отсутствии remote/SSH/registry указать точный внешний blocker и продолжить независимые локальные проверки.

**Критерии B4:** HTTP-клиент, общие типы и Go → ML-заглушка к 2 ч; локальная готовность двух образов, Compose и CI к 6 ч, затем фактические GHCR/SSH deploy и проверка обоих digest; веб-интеграция и batch к 12 ч; staging, устойчивость и второй регион к 24 ч; freeze к 32 ч; воспроизводимые точки запуска, `reports/research.md`, артефакты и submission к 36 ч; готовность защиты — 36–48 ч. Approval этапа разрешает только его согласованный commit/push; merge в main выполняется разрешённым процессом, а deploy запускается автоматически после зелёного main.

**R2 закрыт решением владельца (`globalarray`) без worktree-перепроверки.** Финальные гейты (основной checkout, до коммита): `gofmt -l .` чисто; `go vet ./...` ok; `go test -race -count=1 ./...` ok (app, handler, service/ml, service/store); `govulncheck ./...` — No vulnerabilities found (blocker №2 закрыт); `golangci-lint run` — 565 известных отложенных находок (в основном wsl_v5), исправления отложены владельцем. Импорт-граф по плану: domain → stdlib; handler → domain; service/ml → domain; app → handler + service/ml; cmd → app. Smoke R1.3: readyz 200/JSON, POST 405, SIGTERM graceful, fail-fast конфига. Checksums фикстур: JSON идентичны R0, изменён только README.md (путь клиента, план R1.1). Blocker координации с B3 снят: `areas_test.go` исправлен владельцем, handler-тесты зелёные. Два Sol-ревью (caveman-review, ponytail-review) выполнены, подтверждённые fixes внесены, опровержения и отложенные вопросы записаны выше. Интеграционный критерий B4-01 (настоящий вызов Python-заглушки ML-01) остаётся внешней зависимостью — этап после публикации не `done`.
