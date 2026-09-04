# Примеры и проверки источников B1

Зона Backend №1 (`semennejo`). Файлы используются только тестами `backend/internal/service/source/**`.
Общие фикстуры Go ↔ ML находятся в `backend/testdata/ml-http/` и вносятся B4; отсюда ему
передаётся проверенный пример запроса.

## Происхождение файлов

| Файл | Тип | Как получен |
|---|---|---|
| `overpass/farmland_krasnodar.json` | реальный ответ | `POST https://overpass-api.de/api/interpreter`, `data=[out:json][timeout:60];way["landuse"="farmland"](45.20,38.90,45.35,39.10);out geom 3;`, 04.09.2026, HTTP 200, 3763 байта |
| `overpass/empty_area.json` | реальный ответ | тот же адрес, bbox `(45.02,38.97,45.06,39.02)`, 04.09.2026, HTTP 200, пустой `elements` |
| `openmeteo/era5_krasnodar.json` | реальный ответ | `GET https://archive-api.open-meteo.com/v1/archive?latitude=45.2056541&longitude=38.9746397&start_date=2025-06-01&end_date=2025-06-10&daily=temperature_2m_mean,precipitation_sum&timezone=UTC&models=era5`, 04.09.2026, HTTP 200 |
| `cdse/token_synthetic.json` | синтетический | доступ к CDSE не выдан; форма ответа взята из документации OAuth2 client credentials |
| `cdse/statistics_synthetic.json` | синтетический | форма ответа Statistical API; покрывает пригодный интервал, низкую долю пригодной площади, интервал без пригодных пикселей и интервал с ошибкой расчёта |
| `geojson/*.json` | синтетические | входы пользовательского полигона: Feature, FeatureCollection, MultiPolygon, полигон с отверстием, недопустимая широта |
| `ml-http/analyze_request_example.json` | сгенерирован тестом | `POST /v1/analyze` по контракту v1 из собранного снимка; обновляется `UPDATE_GOLDEN=1 go test ./internal/service/source/` |

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
go test ./internal/service/source/...
```

Проверки с реальными провайдерами (сеть обязательна, CDSE пропускается без ключей):

```bash
go test -tags=live -run TestLive ./internal/service/source/factory/
```
