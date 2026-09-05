# Production checkup — 2026-09-05

## Версия проверки

- Окружение: `http://benzomind.tech:8080`
- Ветка: `main`
- Commit: `42b367e95585a4f9881df7b917450e42620c3b93`
- Короткая версия: `42b367e` (`fix: run deploy script without runner directory access`)
- Режим анализа: production Go + development ML stub (`model_version=dev-stub`)
- Проверка выполнена: 2026-09-05

## Проверенные компоненты

| Область | Проверка | Результат |
|---|---|---|
| Frontend | `GET /` | `200`, HTML TerraLens, ссылки на JS/CSS присутствуют |
| Go readiness | `GET /readyz` | `200`, `status=ready`, `schema_version=1.0`, профиль `ndvi-weather-v1` |
| Areas API | `GET /api/areas` | `200`, после очистки `areas=[]` |
| Runtime config | `GET /api/config` | `200`, лимиты площади/вершин/периода возвращаются |
| Static assets | JS, CSS, Inter font | Все ответы `200` |
| Contours API | валидный bbox | `200`, корректный пустой результат `contours=[]` |
| Error contract | неизвестные area/job | `404`, структурированный код `not_found` |
| Error contract | неверный bbox | `400`, структурированный код `invalid_bbox` |
| Error contract | пустой POST area | `400`, структурированный код `invalid_source` |
| Method contract | `PUT /api/areas` | `405` |
| Browser UI | список участков | Отображается пустое состояние и основные действия |
| Browser UI | режим рисования | Показываются инструкции, минимальное число вершин и отмена |
| Browser UI | экран анализа | Отображаются блоки графика NDVI, погоды и объяснения |

## Сквозной сценарий

Создан временный полигон с коротким периодом `2024-06-01`—`2024-06-03`, после чего запущен анализ через публичный API.

1. `POST /api/areas/{area_id}/analyses` создал задачу.
2. Polling задачи показал переход `running`, этап `collect_satellite`.
3. Задача завершилась со статусом `completed`, без ошибки.
4. `GET /series` вернул три точки временного ряда и три погодные точки.
5. Результат помечен `status=insufficient_data`, `model_version=dev-stub`, `method=dev-ml-stub`.
6. `GET /events` вернул корректный ответ без событий (`status=insufficient_data`, `0` событий).
7. Временный участок удалён; финальная проверка `GET /api/areas` вернула `{"areas":[]}`.

## Быстродействие

- `/readyz`: около `0.14 s`
- `/api/areas`: около `0.12 s`
- `/`: около `0.17 s`

## Ограничения проверки

- Реальная ML-модель и качество восстановления/выявления аномалий не оценивались: в production подключён контрактный ML stub.
- Проверены HTTP-контракты, очередь, polling, persistence, UI-состояния и сквозное прохождение анализа.
- Проверка не заменяет контрольный запуск с реальными CDSE credentials и production ML image.

## Итог

Веб-приложение доступно, Go API готов, PostgreSQL persistence работает, сквозной анализ проходит на stub-контракте, временные данные удалены. Отчёт относится к commit `42b367e`; при следующем деплое его нужно обновить новым SHA и фактическими результатами.
