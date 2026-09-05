# Production checkup — 2026-09-05

## Версия проверки

- Окружение: `http://benzomind.tech:8080`
- Ветка: `main`
- Commit: `7be63ab5acde5bfa08248b64655a86ab2f2056c6`
- Режим анализа: production Go + development ML stub
- Проверка выполнена: 2026-09-05

## Базовые endpoints

| Проверка | Результат |
|---|---|
| `GET /readyz` | `200`, `status=ready`, schema `1.0`, profile `ndvi-weather-v1` |
| `GET /api/areas` | `200`, после cleanup `{"areas":[]}` |
| Region A contours, bbox `38.90,45.20,39.10,45.35` | `200`, 200 contours |
| Region B initial bbox, `37.50,50.55,37.60,50.65` | `200`, 0 contours |
| Region B fallback bbox, `37.30,50.40,37.55,50.65` | `200`, 23 contours |

## Сквозной B1 → Go → PostgreSQL → ML stub

Созданы два временных полигона из первых контуров регионов A и B:

- `fe4b151e-2ef9-4d6e-bfe1-22e4b1da2c21`
- `ca3fb03d-b367-43fb-9228-8b550c1c7f9c`

Анализы завершились контролируемой ошибкой source stage:

- jobs `9e6f1980-62d9-4fd2-a3a8-296e2534e609` и `550df0bb-4d52-4b48-bfc9-11c976e93db8`;
- `status=failed`;
- `error.code=source_failed`;
- `error.message=copernicus-dataspace: invalid_request: HTTP 400`;
- поздний результат не сохранён (`result_version=null`).

После проверки оба временных участка удалены (`DELETE 204`), финальный список снова
вернул `{"areas":[]}`.

## Ограничения

- Этот запуск не доказывает успешный CDSE collection: upstream CDSE вернул HTTP 400.
- ML stub и PostgreSQL pipeline были доступны, но до ML расчёта оба job не дошли.
- Требуется отдельный повтор BE-06 после исправления CDSE request/credentials; старый
  успешный stub checkup `production-checkup-2026-09-05.md` относится к commit `42b367e`.

## Повтор после CDSE numeric-resolution fix

- Deploy проверен через `GET /readyz`: `200`, `status=ready`.
- Commit, отправленный в `main`: `eab76834ba2ea062899da698f117895c632f1516`.
- Оба региона снова созданы и удалены после проверки.
- Jobs `4efd1a6e-35e4-424a-a790-3e3b87a669a5` и
  `6b0524cc-194d-4534-8933-25d5b0af0a0b` дошли до CDSE и завершились
  `source_failed: copernicus-dataspace: invalid_request: http status 400`.
- Значит, numeric `resx/resy` не устранил весь несовместимый payload; успешный
  CDSE collection пока не доказан.
