# cosmohack — восстановление NDVI и мониторинг сельхозтерриторий

Go-монолит с frontend, отдельный Python ML-сервис в `backend/ml/`, два образа
и один выпуск Compose. Архитектура, роли и этапы — [.agent/plan.md](.agent/plan.md);
HTTP-контракт Go ↔ ML v1 — [.agent/contracts/go-ml-http.md](.agent/contracts/go-ml-http.md).

## Быстрый старт (веб-сервис Go)

```bash
cd backend
go test -race ./...          # проверки
DATABASE_URL='postgres://cosmohack:cosmohack@127.0.0.1:5432/cosmohack?sslmode=disable' \
  go run ./cmd/server       # HTTP_ADDR=:8080, ML_BASE_URL=http://127.0.0.1:8000
```

Переменные окружения Go-сервера:

| Переменная | По умолчанию | Смысл |
|---|---|---|
| `HTTP_ADDR` | `:8080` | адрес слушателя |
| `DATABASE_URL` | — | обязательный URL PostgreSQL |
| `DB_TIMEOUT` | `5s` | таймаут проверки подключения к PostgreSQL |
| `ML_BASE_URL` | `http://127.0.0.1:8000` | адрес ML; задаётся оператором, не из запросов |
| `ML_MODEL_VERSION` | пусто | ожидаемая версия модели; сверяется с ответом ML |

Готовность: `curl http://127.0.0.1:8080/readyz` — собственная готовность Go;
готовность ML проверяется отдельно (`GET /readyz` ML-сервиса).

Веб-приложение собрано в одном frontend-пакете: `/` открывает лендинг
TERRALENS, а `/panel.html` — рабочую панель. Кнопки лендинга переходят в панель
на том же origin и сохраняют demo-параметры в query string.

## ML-сервис (`backend/ml`)

Код `vegetation_ml`, HTTP-сервис, batch CLI, тесты и результаты экспериментов
поставлены ML-владельцем `xsqclown`. Запуск и устройство модели описаны в
[backend/ml/README.md](backend/ml/README.md), этапы подключения к Go и deploy —
в [.agent/plans/ml-integration.md](.agent/plans/ml-integration.md). Канонический
контракт production остаётся в [.agent/contracts/go-ml-http.md](.agent/contracts/go-ml-http.md).

Образ собирается через `backend/ml/Dockerfile`; обучение при сборке и старте
HTTP-сервиса не выполняется. Независимая batch-команда работает без Go и
frontend.

## Docker и Compose

Публикация и доставка — приватный `ghcr.io` (пакеты
`ghcr.io/benzocloud/cosmohack/go`, `/ml` и временный `/ml-stub`, видимость
private; первый push создаёт их приватными). Базовые слои
`node/golang/alpine` — публичные library-образы Docker Hub; при блокировке
Docker Hub они зеркалируются в тот же приватный GHCR отдельным решением.

```bash
docker build -t cosmohack-go -f Dockerfile .              # из корня репозитория
docker build -t cosmohack-ml -f backend/ml/Dockerfile .
cd deploy
GO_IMAGE=cosmohack-go ML_IMAGE=cosmohack-ml MODEL_VERSION=dev docker compose up -d
```

- Сеть: Go обращается к ML по `ML_BASE_URL=http://ml:8000`; порт ML не публикуется.
- PostgreSQL хранится в named volume `postgres-data`; одноразовый сервис
  `migrate` применяет `backend/migrations` до запуска Go. ML получает только
  read-only каталог артефактов (`ML_ARTIFACTS_DIR_HOST`).
- `frontend/dist` попадает в образ Go в `/app/public`; корневой `index.html` —
  лендинг, `panel.html` — рабочая панель.

## Деплой и откат

```bash
GO_IMAGE=ghcr.io/<org>/cosmohack/go@sha256:... \
ML_IMAGE=ghcr.io/<org>/cosmohack/ml@sha256:... \
MODEL_VERSION=<версия> \
DATABASE_URL='postgres://user:password@postgres-host:5432/cosmohack?sslmode=disable' \
./deploy/deploy.sh
```

Порядок: логин в приватный GHCR (на сервере нужен токен `read:packages`:
`GHCR_USER`/`GHCR_TOKEN`) → pull двух digest → применить forward-миграции
PostgreSQL, пока текущий Go остаётся доступен →
остановить приём анализов → обновить ML и проверить его `/readyz` (status/schema/profile/model_version) →
запустить Go и проверить `/readyz` + API. `current-manifest.json` хранит пару
digest и версии; при провале возвращается предыдущая совместимая пара
(`previous-manifest.json`). Первая неудачная установка без предыдущей версии
считается ошибкой.

## CI/CD

`.github/workflows/pipeline.yml`: PR — Go (fmt/vet/tests с race, golangci-lint,
govulncheck), Python и frontend, сборка Go и ML без публикации. Main публикует
и развёртывает настоящий ML только после появления проверенного маркера
`backend/ml/PRODUCTION_READY`; до завершения плана интеграции production
остаётся на `ml-stub`. `packages:write` есть только у publish job. Deploy использует `DATABASE_URL`, `GHCR_USER`,
`GHCR_TOKEN`, `CDSE_CLIENT_ID`, `CDSE_CLIENT_SECRET` и `MODEL_VERSION`.
Проверки и публикация выполняются на GitHub-hosted runner; deploy — на
self-hosted runner. Actions зафиксированы на проверенных SHA.

## Лимиты и совместимость выпусков

- Очередь анализа: 8 ожидающих задач, один воркер; переполнение — 429.
- HTTP Go → ML: соединение 3 с, полный вызов 120 с, readiness 2 с; тело
  запроса/ответа 1/4 MiB; ≤4096 наблюдений; без сжатия.
- Версии выпуска: `schema_version=1.0`, `feature_profile=ndvi-weather-v1`,
  `model_version` — из манифеста; Go сверяет их с ML перед анализом.
- Изменение лимитов согласуют B4 и ML и фиксируют здесь.

## Данные и лицензии

Каталоги `data/`, `artifacts/`, файл `submission.csv` — рабочие, вне Git.
Источники контуров/спутников/погоды, лицензии и параметры сбора описывает B1
по мере подключения; происхождение данных сохраняется в снимках Go и
передаётся в `provenance` результата.
