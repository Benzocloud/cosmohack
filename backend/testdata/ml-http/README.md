# Фикстуры HTTP-контракта Go ↔ ML (v1)

Общие примеры запросов и ответов для проверок Go-клиента (`internal/service/ml`)
и HTTP-адаптера ML (`backend/ml/`). Каталог принадлежит B4 (`globalarray`);
ML и B1 передают свои примеры через B4, параллельно файлы не меняют.
Контракт — .agent/contracts/go-ml-http.md.

| Файл | Назначение |
|---|---|
| `request_success.json` | Успешный запрос: usable-наблюдения, пропуск, погода и reference, контекстная точка вне периода |
| `response_success.json` | Успешный ответ на `request_success.json`: observed/imputed/missing и событие candidate |
| `request_insufficient.json` | Условный запрос без пригодных наблюдений (пример из контракта) |
| `response_insufficient.json` | Ответ `insufficient_data` на `request_insufficient.json` |
| `readyz_ready.json` | Тело `200` для `GET /readyz` |
| `readyz_not_ready.json` | Тело `503` для `GET /readyz`, модель не загружена |
| `error_busy.json` | Ошибка `429 busy` |
| `error_invalid_input.json` | Ошибка `422 invalid_input` |
| `error_unsupported_contract.json` | Ошибка `422 unsupported_contract` |

Идентификаторы фикстур условные (`job-fixture-1`), география не подразумевается.
Примеры фиксируют форму контракта, не качество модели.
