# Примеры и проверки источников B1

Fixtures are stored next to the package that owns their contract. Shared Go ↔ ML fixtures live in
`backend/testdata/ml-http/`.

## Происхождение файлов

| Файл | Тип | Как получен |
|---|---|---|
| `internal/integration/overpass/testdata/*.json` | provider responses | Overpass API responses |
| `internal/integration/openmeteo/testdata/*.json` | provider responses | Open-Meteo API responses |
| `internal/integration/cdse/testdata/*.json` | synthetic provider responses | CDSE OAuth and Statistics API schemas |
| `internal/domain/geo/testdata/*.json` | synthetic geometry inputs | GeoJSON parsing cases |
| `testdata/ml-http/analyze_request_example.json` | generated contract fixture | canonical `POST /v1/analyze` request |

Синтетические файлы помечены в имени и не являются доказательством работы провайдера.

## Результат проверки доступа 04.09.2026

| Источник | Адрес | Результат | Ограничения |
|---|---|---|---|
| Контуры | `https://overpass-api.de/api/interpreter` (резерв `https://overpass.kumi.systems/api/interpreter`) | HTTP 200, реальные `landuse=farmland`, `timestamp_osm_base` возвращается | публичная квота Overpass, `[timeout:60]` в запросе, выдача ограничена `out geom N` |
| Погода | `https://archive-api.open-meteo.com/v1/archive` (резерв `https://archive-api.open-meteo.com/v1/era5`) | HTTP 200, `models=era5`, `timezone=GMT`, `utc_offset_seconds=0`, единицы `°C` и `mm` | архив отстаёт от текущей даты; ответ содержит центр ячейки реанализа, отличный от запрошенной точки |
| Спутник | `https://sh.dataspace.copernicus.eu/api/v1/statistics`, токены `https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token` | HTTP 401 без ключей — сервис доступен, доступ команды не выдан | блокер B1: нужны `CDSE_CLIENT_ID` и `CDSE_CLIENT_SECRET`; лимиты запросов будут уточнены после выдачи |

Резервный путь проверяется только после фактического отказа основного. Для спутника
резервным путём остаётся согласованный доступ участника к GEE; он не реализован,
пока не проверен основной путь.

## Переменные окружения

| Переменная | Назначение | Значение по умолчанию |
|---|---|---|
| `CDSE_CLIENT_ID`, `CDSE_CLIENT_SECRET` | доступ к CDSE, обязательны для спутниковых данных | нет |
| `CDSE_STATISTICS_URL`, `CDSE_TOKEN_URL` | адреса CDSE | рабочие адреса Copernicus |
| `OVERPASS_URL`, `OVERPASS_FALLBACK_URL` | адреса поиска контуров | overpass-api.de, overpass.kumi.systems |
| `WEATHER_URL`, `WEATHER_FALLBACK_URL` | адреса архива погоды | archive-api.open-meteo.com |
| `SATELLITE_AGGREGATION_DAYS` | интервал агрегации NDVI, дней | 5 |
| `SATELLITE_MIN_VALID_FRACTION` | нижняя граница доли пригодной площади | 0.5 |

Секреты передаются только окружением, не попадают в код, фикстуры и строковые
представления настроек.

## Команды проверки

Локальные проверки без сети:

```bash
go test ./internal/service/source/... ./internal/integration/...
```

Проверки с реальными провайдерами (сеть обязательна, CDSE пропускается без ключей):

```bash
go test -tags=live -run TestLive ./internal/service/source/factory/
```
