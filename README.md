# cosmohack — восстановление NDVI и мониторинг сельхозтерриторий

Go-монолит с frontend, отдельный Python ML-сервис в `backend/ml/`, два образа
и один выпуск Compose. Архитектура, роли и этапы — [.agent/plan.md](.agent/plan.md);
HTTP-контракт Go ↔ ML v1 — [.agent/contracts/go-ml-http.md](.agent/contracts/go-ml-http.md).

## Быстрый старт (веб-сервис Go)

```bash
cd backend
go test -race ./...          # проверки
go run ./cmd/server          # HTTP_ADDR=:8080, DATA_DIR=./data, ML_BASE_URL=http://127.0.0.1:8000
```

Переменные окружения Go-сервера:

| Переменная | По умолчанию | Смысл |
|---|---|---|
| `HTTP_ADDR` | `:8080` | адрес слушателя |
| `DATA_DIR` | `./data` | каталог постоянного хранилища снимков (вне Git) |
| `ML_BASE_URL` | `http://127.0.0.1:8000` | адрес ML; задаётся оператором, не из запросов |
| `ML_MODEL_VERSION` | пусто | ожидаемая версия модели; сверяется с ответом ML |

Готовность: `curl http://127.0.0.1:8080/readyz` — собственная готовность Go;
готовность ML проверяется отдельно (`GET /readyz` ML-сервиса).

## ML-сервис (backend/ml) — ожидается от ML-владельца

Код сервиса, пакет `vegetation_ml` и независимая batch CLI принадлежат
`xsqclown`; здесь фиксируется способ запуска. Образ: `backend/ml/Dockerfile`
(Python runtime, зафиксированные зависимости, пакет и версионированный
артефакт; обучение при сборке/старте не выполняется). Маршруты: `POST
/v1/analyze`, `GET /readyz` — см. контракт. До поставки ML все проверки
Python в CI пропускаются автоматически.

Batch-команда `private_features.csv → submission.csv` будет задокументирована
вместе с поставкой ML (та же точка запуска не требует работающего веб-сервиса).

## Docker и Compose

Публикация и доставка — приватный `ghcr.io` (пакеты `ghcr.io/benzocloud/cosmohack/go`
и `/ml`, видимость private; первый push создаёт их приватными). Базовые слои
`node/golang/alpine` — публичные library-образы Docker Hub; при блокировке
Docker Hub они зеркалируются в тот же приватный GHCR отдельным решением.

```bash
docker build -t cosmohack-go -f Dockerfile .              # из корня репозитория
docker build -t cosmohack-ml -f backend/ml/Dockerfile .   # после поставки ML
cd deploy
GO_IMAGE=cosmohack-go ML_IMAGE=cosmohack-ml MODEL_VERSION=dev docker compose up -d
```

- Сеть: Go обращается к ML по `ML_BASE_URL=http://ml:8000`; порт ML не публикуется.
- Том снимков Go: `DATA_DIR_HOST:/data` (по умолчанию `./data`); ML получает
  только read-only подкаталог артефактов и не имеет доступа к снимкам.
  Контейнер работает под uid 10001: `deploy/deploy.sh` сам создаёт каталог
  тома с нужным владельцем, при ручном `docker compose up` создайте его
  сами (`install -d -o 10001 -g 10001 data`).
- `frontend/dist` попадает в образ Go в `/app/public`; до поставки frontend
  используется явно помеченная минимальная страница.

## Деплой и откат

```bash
GO_IMAGE=ghcr.io/<org>/cosmohack/go@sha256:... \
ML_IMAGE=ghcr.io/<org>/cosmohack/ml@sha256:... \
MODEL_VERSION=<версия> ./deploy/deploy.sh
```

Порядок: логин в приватный GHCR (на сервере нужен токен `read:packages`:
`GHCR_USER`/`GHCR_TOKEN`) → pull двух digest → пауза приёма анализов (stop go) →
обновить ML и проверить его `/readyz` (status/schema/profile/model_version) →
запустить Go и проверить `/readyz` + API. `current-manifest.json` хранит пару
digest и версии; при провале возвращается предыдущая совместимая пара
(`previous-manifest.json`). Первая неудачная установка без предыдущей версии
считается ошибкой.

## CI/CD

`.github/workflows/pipeline.yml`: PR — Go (fmt/vet/tests с race, golangci-lint,
govulncheck), Python и frontend (если соответствующие пакеты доставлены),
сборка обоих образов без публикации. Main — то же + публикация двух образов в
GHCR через `GITHUB_TOKEN` (`packages:write` только у задачи публикации) и
локальный деплой по digest на выделенном self-hosted runner, когда владельцем
включены `DEPLOY_ENABLED`, `MODEL_VERSION`, `GHCR_USER` и `GHCR_TOKEN`.
Проверки и публикация выполняются на GitHub-hosted runner; self-hosted runner
используется только для deploy. Actions зафиксированы на проверенных SHA.

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
