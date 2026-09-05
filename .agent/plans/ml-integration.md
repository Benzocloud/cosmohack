# Интеграция `ndvi-gapfill` в cosmohack

**Владельцы:** ML — `xsqclown`; Go/CI/CD — `globalarray`; входные данные —
`semennejo`; публичный API и persistence — `tsuckermandev`.

**База cosmohack:** `dd682f66e5f7f648f69d847da0f453662339c4d0`.
**Upstream:** `git@github.com:xsqclown/ndvi-gapfill.git`, `main` на
`840a5677df3187cbfe9303ddee058dd0bbc34743`.
**Рабочая ветка импорта:** `xsqclown/feat/import_ndvi_gapfill`.

## Цель и порядок включения

Перенести ML-разработку в монорепозиторий без squash и переписывания авторов,
затем заменить production stub настоящим Python-сервисом. Сначала подключается
уже совместимый путь `schema_version=1.0`, `feature_profile=ndvi-weather-v1`.
Расширенный `v1.1/ndvi-multisensor-v1` вводится отдельно, потому что текущий Go
не передаёт `indices`, `area_context` и `peers`, необходимые обученной модели.

Итоговый pipeline первого выпуска:

```text
frontend
  -> Go API
  -> PostgreSQL job
  -> B1: CDSE NDVI + Open-Meteo
  -> Go analysis worker
  -> Python POST /v1/analyze (v1.0, ndvi-weather-v1)
  -> Go validates and persists AnalysisResult
  -> frontend polls and renders series/events
```

Это запускает настоящий код восстановления и аномалий вместо stub. Обученный
`HistGradientBoostingRegressor` начнёт применяться в веб-пути только после
MLI-05; до этого HTTP-сервис честно возвращает резервный метод и ограничения.

## Найденные расхождения

| Стык | Текущее состояние | Что требуется |
|---|---|---|
| История | В upstream 25 коммитов автора `Danil <106477538+Shusha228@users.noreply.github.com>` | Merge без squash/rebase/filter, сохранить исходные SHA и автора |
| Entry point | Dockerfile запускает отсутствующий `vegetation_ml.http` | Запускать `python -m vegetation_ml` |
| Путь модели | Сервис читает `VEGETATION_ML_MODEL`, Dockerfile задаёт неиспользуемый `MODEL_ARTIFACT` | Одна переменная и один путь `/app/ml/artifacts/restoration.pkl` |
| Артефакт | Upstream содержит `artifacts/restoration.pkl`, `.dockerignore` исключает его, Compose перекрывает каталог пустым bind mount | Выбрать один воспроизводимый способ поставки и проверить SHA-256 |
| Версия | Go/deploy ожидают `MODEL_VERSION`, Python возвращает вычисленную строку `ndvi-gapfill-0.2.0+hgb` или `+fallback` | Один release manifest; readiness, ответ ML и Go ожидают одно значение |
| Контракт | Go поддерживает только v1.0; Python дополнительно реализует v1.1 | Сначала доказать v1.0; v1.1 не включать неявно |
| Признаки модели | B1/Go передают итоговый NDVI и погоду | Для HGB нужны сенсорные индексы, культура, история и при возможности peers |
| Runtime-тексты | В Python есть русские ошибки и логи | Ошибки и логи привести к английскому по правилам проекта |
| Runtime | README upstream указывает Python 3.14, CI/Docker используют 3.12 | Зафиксировать и проверить один поддерживаемый runtime |
| Контрактные фикстуры | Общие Go-фикстуры и Python-модели развивались независимо | Прогнать один и тот же JSON через Python и Go-validator |

## MLI-00 — импорт с сохранением происхождения

**Статус:** awaiting_approval.

**Область:** история Git, `backend/ml/`, `reports/`, batch-результаты,
`artifacts/`, корневые `README.md` и `.gitignore`.

- [x] Проверить upstream HEAD, дерево, размер артефакта и авторов.
- [x] Получить upstream в `refs/remotes/ndvi-gapfill/main`.
- [x] Выполнить `merge --no-commit --no-ff --allow-unrelated-histories` без
  squash, rebase и переписывания исходных commit objects.
- [x] Оставить главный README проекта в корне; сохранить документацию автора в
  `backend/ml/README.md` и поправить только монорепозиторные ссылки.
- [x] Объединить локальные и Python-исключения `.gitignore`; не добавлять
  `.idea/` и локальный production checkup.
- [x] Защитить текущий production: наличие `requirements.txt` запускает проверки
  ML, но publish/deploy переключатся со stub только по отдельному
  `backend/ml/PRODUCTION_READY`, который появится в MLI-03.
- [x] Запустить Python-тесты и lint в доступном Python 3.14: 68 tests passed,
  `ruff check .` passed после минимальных lint-исправлений в merge-слое.
- [ ] Доказать происхождение после merge-коммита:
  `git merge-base --is-ancestor 840a5677... HEAD`, вывести 25 upstream-коммитов
  и убедиться, что их author name/email не изменились.
- [x] Пройти два read-only review, исправить подтверждённые замечания,
  выполнить self-review и предъявить этап на апрув.

**Готово:** в дереве есть полный ML-пакет и отчёты, upstream HEAD является
предком merge-коммита, исходные SHA и авторство сохранены, конфликтов нет.

**Rollback:** до публикации — `git merge --abort`; после публикации — обычный
revert merge-коммита. Upstream-репозиторий остаётся неизменным.

## MLI-01 — воспроизводимый ML-образ и артефакт

**Статус:** review. **Зависимость:** MLI-00.

**Область:** `backend/ml/Dockerfile`, `.dockerignore`,
`backend/ml/vegetation_ml/service.py`, `deploy/compose.yaml`, manifest модели.

- [x] Исправить entry point на `python -m vegetation_ml` и добавить
  непривилегированного runtime-пользователя.
- [x] Зафиксировать Python 3.12 как runtime, если установка всех pinned wheels и
  68 тестов проходит; иначе синхронно изменить Docker, CI и README.
- [x] Для хакатонного выпуска упаковать уже импортированный
  `artifacts/restoration.pkl` в ML-образ вместе с manifest. Не обучать модель в
  Docker build или startup. Будущие артефакты этим исключением не добавлять.
- [x] Добавить SHA-256 артефакта в manifest и проверять его перед загрузкой.
- [x] Передать сервису `VEGETATION_ML_MODEL=/app/ml/artifacts/restoration.pkl`;
  убрать bind mount, который перекрывает файл внутри image.
- [x] Сделать production readiness `503`, если заявленный HGB-артефакт отсутствует,
  повреждён или не загружается. Локальный fallback запускать явным режимом.
- [x] Вынести `model_version` в один manifest и использовать его в `/readyz`,
  `/v1/analyze`, Go `ML_MODEL_VERSION` и release manifest.

**Проверки:** Docker build; запуск контейнера без volume; `/readyz` сообщает
`ready`, v1.0, профиль `ndvi-weather-v1` и ожидаемую версию; checksum внутри
контейнера совпадает с manifest; отсутствие/повреждение модели даёт 503.

**Готово:** один digest ML-образа самодостаточен и не зависит от файлов,
случайно оставленных на runner/сервере.

## MLI-02 — контракт v1.0 между Go и Python

**Статус:** review. **Зависимость:** MLI-01.

**Область:** `.agent/contracts/go-ml-http.md`, `backend/testdata/ml-http/`,
`backend/ml/vegetation_ml/contracts.py`, `service.py`, Python tests и Go ML
client tests. Математику модели в Go не переносить.

- [x] Зафиксировать v1.0 единственным production-профилем первого выпуска;
  дополнительные поля readiness считать совместимыми, но не активировать v1.1.
- [x] Взять реальный JSON из `AnalyzeRequestBuilder`, а не вручную написанный
  похожий объект, и принять его строгой Pydantic-схемой.
- [x] Ответ Python тем же запросом прогнать через Go `validateResult`: точные
  echo-поля, даты, original/imputed/missing, status/severity/events и ограничения.
- [x] Свести `Source.mapping` к одному типу во всех документах, Go DTO,
  Python-модели и общих фикстурах.
- [x] Проверить JSON `null`, пустые observations, NaN/Inf, неизвестные поля,
  лимит 1 MiB, busy 429, not-ready 503 и internal error 500.
- [x] Все runtime error messages и logs в Python перевести на английский;
  пользовательскую локализацию оставить frontend.
- [x] Проверить, что отмена Go-запроса не освобождает Python-slot до окончания
  расчёта и автоматического повторного POST нет.

**Проверки:** `ruff check .`, `python -m pytest tests -q`, `go test ./...`,
контрактный тест с настоящим FastAPI process и Go client.

**Готово:** один v1.0 request, созданный production Go builder, завершается
реальным Python result и принимается Go без специальной заглушки.

## MLI-03 — CI/CD настоящего ML

**Статус:** review. **Зависимости:** MLI-01/02.

**Область:** `.github/workflows/pipeline.yml`, `deploy/`, README.

- [x] Python job устанавливает только зафиксированные runtime/dev зависимости,
  запускает ruff и полный pytest.
- [x] Docker job запускает собранный образ, ждёт `/readyz` и выполняет один
  контрактный POST; успешной сборки image без запуска недостаточно.
- [x] После прохождения MLI-01/02 добавить `backend/ml/PRODUCTION_READY`:
  только этот явный marker переключает publish/deploy со stub на реальный ML.
- [ ] После первого успешного production rollout оставить stub только в явном
  локальном Compose-профиле и как проверенный rollback image.
- [x] Deploy передаёт согласованную model version, поднимает ML, проверяет
  readiness и только затем переключает Go на новый digest.
- [x] Rollback возвращает предыдущую совместимую пару Go+ML; схема PostgreSQL в
  этом переходе не меняется.
- [x] Секреты и данные организаторов не включать в image, logs или artifacts CI.

**Готово:** зелёный main публикует два digest и deploy использует настоящий ML;
stub не может быть выбран production job автоматически.

## MLI-04 — сквозной v1.0 и браузерная приёмка

**Статус:** planned. **Зависимость:** MLI-03 и успешный deploy.

- [ ] Создать участок через UI, запустить анализ, проверить polling стадий и
  публикацию результата после PostgreSQL restart.
- [ ] В network/logs доказать путь Go → Python и `method`, отличный от
  `dev-ml-stub`; показать limitations резервного метода.
- [ ] Проверить usable, missing и unusable наблюдения, отсутствие данных,
  busy, timeout и рестарт ML без потери job consistency.
- [ ] Сверить график и события frontend с сохранённым `AnalysisResult`.
- [ ] После реализации получить апрув, commit/push, дождаться deploy; production
  браузерный тест запускать после сообщения пользователя о готовом deploy.

**Готово:** production пользовательский flow проходит через настоящий Python
сервис, результат переживает рестарт и виден после повторного открытия панели.

## MLI-05 — обученная модель в веб-пути (v1.1)

**Статус:** done with limitations, P1. **Не блокирует MLI-04.**

- [x] До кода обновить канонический контракт: `schema_version=1.1`, профиль
  `ndvi-multisensor-v1`, `indices`, `area_context`, `peers`, лимиты и ошибки.
- [x] B1 проверить, какие S2/Landsat/MODIS NDVI/EVI/NDWI реально доступны для
  одного полигона. Не синтезировать отсутствующие сенсорные значения.
- [x] Определить честный источник `crop_type` и правила выбора соседей; при их
  отсутствии передавать null/пустой список и отражать ограничение.
- [x] Расширить Go domain/builder/client после контракта; поднять request limit
  до 8 MiB только для согласованного профиля и сохранить строгую валидацию.
- [x] Провести реальные S2 NDVI/EVI/NDWI из CDSE через `SatelliteSample`,
  snapshot и Go request builder в поле `observations[].indices`.
- [x] Добавить negotiation по `/readyz`: Go выбирает v1.1 только если ML
  объявляет профиль и артефакт готов; иначе остаётся на v1.0.
- [x] Доказать по ответу `method=gradient_boosting_residual`, что HGB реально
  применился, а не только был загружен.
- [x] Повторить сквозные и браузерные проверки, отдельно записать качество
  web-профиля и отличие от локальной batch-оценки.

**Готово:** production smoke-тест на AOI `4cf5db6c-5288-43b3-97c5-5a95bd609383`
получил 70 usable S2-наблюдений за 731 день; ответ содержит
`method=gradient_boosting_residual`, а результат отображается в браузере.
Go принимает результат v1.1, а fallback явно наблюдаем и не выдаётся за HGB.
Landsat/MODIS, crop context и peers пока не подключены; ограничения записываются
в `limitations`.
