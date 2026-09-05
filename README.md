# TerraLens

TerraLens — веб-система мониторинга вегетации сельскохозяйственных участков. Пользователь выбирает или рисует полигон на карте, задаёт период и получает восстановленный временной ряд NDVI, погодный контекст и оценку аномалий растительности.

**Рабочий стенд:** [http://benzomind.tech:8080](http://benzomind.tech:8080)

Решение состоит из React-интерфейса, Go API и фонового исполнителя, PostgreSQL и отдельного Python ML-сервиса. Браузер обращается только к Go по тому же origin; ключи внешних провайдеров и ML-сервис не публикуются во внешнюю сеть.

## Что умеет система

- показывает участки на обычной и спутниковой карте;
- позволяет найти сельскохозяйственные контуры через OpenStreetMap Overpass или нарисовать GeoJSON Polygon вручную;
- собирает по полигону Sentinel-2 L2A индексы NDVI, EVI и NDWI через Copernicus Data Space Ecosystem;
- получает среднюю температуру и сумму осадков из Open-Meteo;
- восстанавливает пропуски NDVI моделью HistGradientBoosting и использует погодные, мультисенсорные и соседние временные ряды, когда они доступны;
- классифицирует результат как `normal`, `candidate`, `confirmed` или `insufficient_data`;
- сохраняет участки, задания, исходные снимки данных и результаты в PostgreSQL;
- показывает ход фонового анализа, метод расчёта, число пригодных и восстановленных наблюдений, ограничения модели и найденные события.

## Как проходит анализ

```mermaid
flowchart LR
    Browser[React + MapLibre] -->|same-origin HTTP| Go[Go API]
    Go --> DB[(PostgreSQL)]
    Go --> Worker[Очередь и worker]
    Worker --> CDSE[CDSE Sentinel-2]
    Worker --> Weather[Open-Meteo]
    Worker --> ML[FastAPI + HGB]
    ML --> Worker
    Worker --> DB
    DB --> Go
    Go --> Browser
```

1. Go проверяет геометрию и период и сохраняет участок.
2. Анализ создаётся как асинхронный job; UI опрашивает его состояние.
3. Worker получает спутниковые индексы и погоду, формирует воспроизводимый входной снимок и при наличии подходящих участков добавляет соседние ряды.
4. ML-сервис восстанавливает пропуски, строит базовую динамику и ищет отклонения.
5. Go атомарно сохраняет результат и отдаёт UI ряд NDVI, погоду, события, provenance и ограничения.

## Стек

| Слой | Технологии |
|---|---|
| Frontend | React 18, TypeScript, Vite, TanStack Query, MapLibre GL, Terra Draw, ECharts, Tailwind CSS |
| Backend | Go 1.27, `net/http`, `sqlx`, `pgx`, один фоновый worker |
| ML | Python 3.12, FastAPI, pandas, NumPy, scikit-learn HistGradientBoosting |
| Хранилище | PostgreSQL 16, SQL-миграции через `migrate/migrate` |
| Источники | CDSE Sentinel Hub Statistical API, Open-Meteo Archive API, OpenStreetMap Overpass API |
| Доставка | Docker, Docker Compose, GitHub Actions, GHCR, self-hosted runner |

## Структура репозитория

```text
.
├── frontend/                 React-приложение: лендинг и рабочая панель
│   ├── src/api/              HTTP-клиент, DTO и адаптеры ответов Go
│   ├── src/features/         карта, участки, анализ и результаты
│   └── tests/                компонентные и интеграционные тесты UI
├── backend/
│   ├── cmd/server/           точка входа Go-сервера
│   ├── internal/app/         composition root и жизненный цикл
│   ├── internal/domain/      доменные сущности и контракты
│   ├── internal/handler/     публичный HTTP API
│   ├── internal/integration/ CDSE, Open-Meteo, Overpass
│   ├── internal/repository/  PostgreSQL repository
│   ├── internal/service/     сценарии участков, сбора и анализа
│   ├── migrations/           forward/down SQL-миграции
│   └── ml/                   Python ML-пакет, HTTP-сервис и тесты
├── artifacts/                зафиксированная модель и её manifest
├── deploy/                   Compose, deployment и ML stub
├── .github/workflows/        CI, публикация образов и deployment
├── .agent/contracts/         версия HTTP-контракта Go ↔ ML
└── Dockerfile                Go runtime со встроенным frontend
```

## Быстрый запуск всего решения

### Требования

- Git;
- Docker Engine с Docker Compose v2;
- свободный TCP-порт `8080`;
- доступ в интернет к Docker Hub, CDSE, Open-Meteo, Overpass и серверам карт;
- OAuth Client ID и Client Secret для Sentinel Hub в Copernicus Data Space Ecosystem.

OAuth-клиент создаётся в настройках Sentinel Hub Dashboard. Официальная инструкция: [CDSE Sentinel Hub Authentication](https://documentation.dataspace.copernicus.eu/APIs/SentinelHub/Overview/Authentication.html#registering-oauth-client). Это именно отдельные **Client ID** и **Client Secret**, а не логин, account ID или пароль пользователя.

### 1. Клонировать и задать credentials

```bash
git clone https://github.com/Benzocloud/cosmohack.git
cd cosmohack

export CDSE_CLIENT_ID='ваш-client-id'
export CDSE_CLIENT_SECRET='ваш-client-secret'
export GO_IMAGE='cosmohack-go:local'
export ML_IMAGE='cosmohack-ml:local'
export MODEL_VERSION="$(sed -n 's/.*"model_version": *"\([^"]*\)".*/\1/p' backend/ml/model-manifest.json)"
```

Credentials нужны уже при старте Go-сервера. Они остаются в окружении процесса и не попадают во frontend, PostgreSQL или Git.

### 2. Собрать два production-образа

Команды выполняются из корня репозитория:

```bash
docker build -t "$GO_IMAGE" -f Dockerfile .
docker build -t "$ML_IMAGE" -f backend/ml/Dockerfile .
```

Первая сборка скачивает зависимости и может занять несколько минут. Обучение модели не запускается: образ получает проверенный артефакт `artifacts/restoration.pkl` и manifest из репозитория.

### 3. Поднять PostgreSQL, миграции, ML, Go и frontend

```bash
cd deploy
docker compose up -d --wait
docker compose ps
```

Откройте [http://localhost:8080](http://localhost:8080). Compose создаёт PostgreSQL volume, применяет `backend/migrations`, проверяет `/readyz` ML и только после этого запускает Go. Наружу опубликован только `${APP_PORT:-8080}`; PostgreSQL и ML доступны лишь внутри Compose-сети.

Проверка готовности:

```bash
curl -fsS http://localhost:8080/readyz
curl -fsS http://localhost:8080/api/config
curl -fsS http://localhost:8080/api/areas
```

Ожидаемый первый ответ начинается с:

```json
{"status":"ready","schema_version":"1.0"}
```

`/readyz` Go проверяет соединение с PostgreSQL. Готовность ML проверяется Compose healthcheck и зависимостью запуска Go от healthy ML.

### Остановка и чистый перезапуск

```bash
docker compose down
```

Данные PostgreSQL при этом сохраняются. Чтобы удалить только локальные данные и начать с пустой базы:

```bash
docker compose down -v
```

## Сценарии проверки через интерфейс

### Готовый production-стенд

1. Откройте [лендинг TerraLens](http://benzomind.tech:8080).
2. В блоке «Выберите участок» переключите примеры результатов и нажмите «Открыть панель».
3. Убедитесь, что панель открыла и выделила выбранный участок.
4. Нажмите «Повторить анализ» и следите за стадиями получения спутниковых данных, погоды, выполнения ML и сохранения результата.
5. После завершения проверьте график NDVI, погодный контекст, метод, число пригодных наблюдений, ограничения и карточки событий.

### Локальный стенд с пустой базой

1. Откройте [http://localhost:8080/panel.html](http://localhost:8080/panel.html). Карта стартует в районе Ростова-на-Дону.
2. Нажмите карандаш, поставьте не менее трёх разных точек и замкните полигон нажатием на первую точку.
3. Задайте название и исторический период, например `2025-04-01` — `2025-09-30`.
4. Нажмите «Добавить и проанализировать».
5. Дождитесь терминального состояния job. Для типичного участка анализ занимает больше времени, чем обычный HTTP-запрос: данные собираются у внешних провайдеров, а UI опрашивает job асинхронно.
6. Проверьте состояния точек графика: исходные наблюдения, восстановленные значения и пропуски. Результат `insufficient_data` является валидным, если пригодных спутниковых наблюдений недостаточно для уверенного вывода.
7. Переключите подложку «Карта»/«Спутник» и повторно включите рисование.
8. Для проверки каталога контуров приблизьте сельскохозяйственный район и нажмите «Найти контуры в этой области».

Ограничения, которые UI получает из `GET /api/config`: площадь участка от 0,5 до 25 000 га, не более 512 вершин и период не более 1461 дня. GeoJSON использует WGS84 и порядок координат `[longitude, latitude]`; первая и последняя точки кольца должны совпадать.

## Проверка API без интерфейса

Создание небольшого участка:

```bash
curl -fsS -X POST http://localhost:8080/api/areas \
  -H 'Content-Type: application/json' \
  --data '{
    "name": "Проверка README",
    "period": {"from": "2025-04-01", "to": "2025-09-30"},
    "geometry": {
      "type": "Polygon",
      "coordinates": [[[39.00,47.20],[39.02,47.20],[39.02,47.22],[39.00,47.22],[39.00,47.20]]]
    },
    "source": {"kind": "drawn"}
  }'
```

Скопируйте поле `id` из ответа и запустите анализ. Период можно переопределить при повторном запуске:

```bash
AREA_ID='<id-участка>'

curl -fsS -X POST "http://localhost:8080/api/areas/${AREA_ID}/analyses" \
  -H 'Content-Type: application/json' \
  --data '{"period":{"from":"2025-04-01","to":"2025-09-30"}}'
```

Ответ `202 Accepted` содержит `job_id`. Используйте его для polling, затем запросите сохранённые данные:

```bash
JOB_ID='<job_id>'

curl -fsS "http://localhost:8080/api/jobs/${JOB_ID}"
curl -fsS "http://localhost:8080/api/areas/${AREA_ID}/series"
curl -fsS "http://localhost:8080/api/areas/${AREA_ID}/events"
```

Основные маршруты:

| Метод и путь | Назначение |
|---|---|
| `GET /readyz` | готовность Go и PostgreSQL |
| `GET /api/config` | публичные лимиты ввода |
| `GET/POST /api/areas` | список и создание участков |
| `GET/DELETE /api/areas/{id}` | чтение и удаление участка |
| `POST /api/areas/{id}/analyses` | асинхронный запуск или повтор анализа |
| `GET /api/jobs/{id}` | состояние и стадия job |
| `GET /api/areas/{id}/series` | NDVI, погода, provenance и метаданные результата |
| `GET /api/areas/{id}/events` | обнаруженные аномальные интервалы |
| `GET /api/regions/contours?bbox=minLon,minLat,maxLon,maxLat` | поиск контуров в текущей области карты |

Коды, полезные при проверке: `409` — у участка уже есть активный анализ, `429` — очередь из восьми ожидающих заданий заполнена, `400` — вход не прошёл проверку, `503` — зависимость временно не готова.

## Разработка и тесты

### Go

Для полного набора Go-тестов нужна доступная PostgreSQL с применёнными миграциями. Проще всего поднять отдельную локальную базу:

```bash
docker network create terralens-test
docker run --rm -d --name terralens-test-postgres \
  --network terralens-test \
  -e POSTGRES_DB=cosmohack \
  -e POSTGRES_USER=cosmohack \
  -e POSTGRES_PASSWORD=cosmohack \
  --health-cmd='pg_isready -U cosmohack -d cosmohack' \
  --health-interval=2s --health-retries=20 \
  -p 127.0.0.1:5432:5432 postgres:16-alpine

until [ "$(docker inspect -f '{{.State.Health.Status}}' terralens-test-postgres)" = healthy ]; do sleep 1; done

docker run --rm --network terralens-test \
  -v "$PWD/backend/migrations:/migrations:ro" \
  migrate/migrate:v4.18.3 \
  -path=/migrations \
  -database 'postgres://cosmohack:cosmohack@terralens-test-postgres:5432/cosmohack?sslmode=disable' up

cd backend
DATABASE_URL='postgres://cosmohack:cosmohack@127.0.0.1:5432/cosmohack?sslmode=disable' go test -race ./...
go vet ./...
```

После тестов удалите временную базу командами `docker rm -f terralens-test-postgres` и `docker network rm terralens-test`. В CI используется такой же PostgreSQL 16, а миграции применяются до тестов.

### Frontend

```bash
cd frontend
npm ci
npm run typecheck
npm run lint
npm test
npm run build
```

Для разработки интерфейса запустите полный Compose-стек на `8080`, затем `npm run dev`. Vite откроется на [http://localhost:5173](http://localhost:5173) и проксирует `/api` и `/readyz` в Go.

### ML

```bash
python3.12 -m venv .venv
source .venv/bin/activate
python -m pip install -r backend/ml/requirements-dev.txt
cd backend/ml
python -m pytest tests -q
```

Подробности обучения, batch-интерфейса, формата датасета и метрик находятся в [backend/ml/README.md](backend/ml/README.md). Версионированный HTTP-контракт Go ↔ ML: [.agent/contracts/go-ml-http.md](.agent/contracts/go-ml-http.md).

## Конфигурация

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | обязательна | PostgreSQL DSN |
| `CDSE_CLIENT_ID` | обязательна | OAuth Client ID Sentinel Hub |
| `CDSE_CLIENT_SECRET` | обязательна | OAuth Client Secret Sentinel Hub |
| `HTTP_ADDR` | `:8080` | адрес Go HTTP-сервера |
| `PUBLIC_DIR` | `/app/public` | собранная статика frontend |
| `LOG_LEVEL` | `info` | уровень логирования Go |
| `DB_TIMEOUT` | `5s` | таймаут подключения к PostgreSQL |
| `ML_BASE_URL` | `http://127.0.0.1:8000` | внутренний адрес ML; Compose задаёт `http://ml:8000` |
| `ML_MODEL_VERSION` | пусто | ожидаемая версия модели |
| `ANALYSIS_QUEUE_SIZE` | `8` | число ожидающих job |
| `SATELLITE_AGGREGATION_DAYS` | `5` | размер интервала спутниковой агрегации |
| `SATELLITE_MIN_VALID_FRACTION` | `0.5` | минимальная доля пригодных пикселей |
| `CDSE_STATISTICS_URL` | endpoint CDSE | переопределение Statistical API |
| `CDSE_TOKEN_URL` | endpoint CDSE OAuth | переопределение token API |
| `OVERPASS_URL`, `OVERPASS_FALLBACK_URL` | публичные Overpass endpoints | основной и резервный поиск контуров |
| `WEATHER_URL`, `WEATHER_FALLBACK_URL` | Open-Meteo Archive/ERA5 | основной и резервный источник погоды |
| `VEGETATION_ML_ENABLE_MULTISENSOR` | `true` в Compose | включает профиль `ndvi-multisensor-v1` |

## CI/CD и deployment

Workflow [.github/workflows/pipeline.yml](.github/workflows/pipeline.yml) на каждом push и pull request выполняет Go fmt/vet/race/lint/vulnerability checks, Python lint/tests, frontend typecheck/build, сборку обоих production-образов и контрактный smoke-тест Go ↔ ML.

Push в `main` после зелёных проверок публикует immutable Go и ML images в GHCR. Поскольку `backend/ml/PRODUCTION_READY` присутствует, deployment использует настоящую модель. Self-hosted runner на целевой машине применяет миграции, проверяет manifest и readiness новой пары образов и возвращает предыдущую совместимую пару при неуспешном запуске.

Для deployment job настроены:

- repository variables: `DEPLOY_ENABLED=true`, при необходимости `DEPLOY_DIR`;
- secrets: `GHCR_USER`, `GHCR_TOKEN` с `read:packages`, `DATABASE_URL`, `CDSE_CLIENT_ID`, `CDSE_CLIENT_SECRET`.

Ручной production deployment выполняется только digest-образами через [deploy/deploy.sh](deploy/deploy.sh); скрипт документирует обязательные переменные в своей секции «Использование».

## Ограничения решения

- сервис рассчитан на демонстрационный однопользовательский контур: аутентификации и разделения данных по пользователям нет;
- один Go worker выполняет задания последовательно, а ML-сервис допускает один расчёт одновременно;
- качество результата зависит от облачности, площади полигона, выбранного периода и доступности внешних API;
- `insufficient_data` означает, что система не стала выдавать уверенное заключение при недостатке пригодных наблюдений;
- найденная аномалия является аналитическим сигналом для проверки, а не агрономическим диагнозом.

## Дополнительная документация

- [ML: обучение, batch CLI, контракт и метрики](backend/ml/README.md)
- [HTTP-контракт Go ↔ ML](.agent/contracts/go-ml-http.md)
- [Общий архитектурный план](.agent/plan.md)
- [План интеграции ML](.agent/plans/ml-integration.md)

## Команда

- `globalarray` — backend, архитектура и CI/CD;
- `semennejo` — backend;
- `tsuckermandev` — backend;
- `xsqclown` — ML;
- `Prosteyshiyyy` — frontend.
