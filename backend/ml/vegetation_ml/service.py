"""HTTP-сервис ML по контракту Go ↔ ML v1.

Реализованы два маршрута: синхронный `POST /v1/analyze` и `GET /readyz`.
Очереди, job API и обратных вызовов в Go здесь нет — они принадлежат Go.

Два требования контракта определяют устройство сервиса:

* один вычислительный слот на весь сервис без внутренней очереди. Слот
  удерживается до фактического окончания расчёта, даже если клиент уже ушёл по
  тайм-ауту: иначе следующий POST запустил бы второй расчёт параллельно;
* readiness обязан отвечать во время расчёта. Поэтому вычисление уходит в
  рабочий поток, а событийный цикл остаётся свободным.
"""

from __future__ import annotations

import json
import logging
import os
import threading
from contextlib import asynccontextmanager
from pathlib import Path

import anyio
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from pydantic import ValidationError

from . import __version__, contracts, model as M, paths, web_analysis
from .contracts import AnalyzeRequest, UnsupportedContract

MAX_REQUEST_BYTES = 1 * 1024 * 1024
MAX_REQUEST_BYTES_EXTENDED = 8 * 1024 * 1024
MAX_RESPONSE_BYTES = 4 * 1024 * 1024

MODEL_PATH_ENV = "VEGETATION_ML_MODEL"

log = logging.getLogger("vegetation_ml.service")

_slot = threading.Lock()
_state = {"ready": False, "reason": "сервис ещё не запущен"}
_model = {"artifact": None}


def _load_artifact():
    """Загружает артефакт один раз на процесс.

    Отсутствие артефакта не делает сервис неготовым: базовый профиль работает
    и без него. Меняется только список профилей, объявляемый в /readyz.
    """
    path = Path(os.environ.get(MODEL_PATH_ENV, paths.MODEL_ARTIFACT))
    if not path.exists():
        log.warning("артефакт модели не найден: %s", path)
        return None
    try:
        return M.RestorationModel.load(path)
    except Exception:
        log.exception("артефакт модели не загружен: %s", path)
        return None


def available_profiles() -> tuple[str, ...]:
    """Профили, которые процесс действительно может обслужить."""
    if _model["artifact"] is None:
        return (contracts.FEATURE_PROFILE,)
    return (contracts.FEATURE_PROFILE, contracts.PROFILE_MULTISENSOR)


@asynccontextmanager
async def _lifespan(_: FastAPI):
    """Готовность объявляется после загрузки методов расчёта, а не при импорте."""
    web_analysis.warm_up()
    _model["artifact"] = _load_artifact()
    _state["ready"] = True
    _state["reason"] = ""
    log.info("сервис готов, model_version=%s, профили=%s",
             web_analysis.model_version(_model["artifact"]), available_profiles())
    yield
    _state["ready"] = False
    _state["reason"] = "сервис остановлен"


app = FastAPI(title="vegetation-ml", version=__version__, docs_url=None, redoc_url=None,
              lifespan=_lifespan)


class _NonFiniteNumber(ValueError):
    """В теле запроса встретился литерал NaN или Infinity."""


def _reject_constant(name: str):
    raise _NonFiniteNumber(name)


def _error(http_status: int, code: str, message: str, retryable: bool,
           request_id: str | None) -> JSONResponse:
    """Ошибка в формате контракта: без traceback и без внутренних деталей."""
    return JSONResponse(
        status_code=http_status,
        content={
            "schema_version": contracts.SCHEMA_VERSION,
            "request_id": request_id,
            "error": {"code": code, "message": message, "retryable": retryable},
        },
    )


def _validation_message(exc: ValidationError) -> str:
    """Короткое безопасное описание первой ошибки схемы."""
    err = exc.errors()[0]
    loc = ".".join(str(p) for p in err.get("loc", ()) if p != "__root__")
    msg = err.get("msg", "некорректное значение")
    msg = msg.removeprefix("Value error, ")
    return f"{loc}: {msg}" if loc else msg


@app.get("/readyz")
async def readyz() -> JSONResponse:
    if not _state["ready"]:
        return JSONResponse(status_code=503, content={
            "status": "not_ready",
            "schema_version": contracts.SCHEMA_VERSION,
            "reason": _state["reason"] or "модель не загружена",
        })
    return JSONResponse(status_code=200, content={
        "status": "ready",
        "schema_version": contracts.SCHEMA_VERSION,
        "schema_versions": list(contracts.SUPPORTED_SCHEMA_VERSIONS),
        "feature_profiles": list(available_profiles()),
        "model_version": web_analysis.model_version(_model["artifact"]),
    })


def _compute(request: AnalyzeRequest) -> dict:
    """Выполняет расчёт и освобождает слот в том же потоке.

    Освобождение именно здесь, а не в обработчике: если клиент отвалился и
    корутина снята, поток всё равно доработает и отпустит слот сам.
    """
    try:
        return web_analysis.analyse(request, _model["artifact"])
    finally:
        _slot.release()


@app.post("/v1/analyze")
async def analyze(http_request: Request):
    request_id = None
    if not _state["ready"]:
        return _error(503, "not_ready", "Модель не загружена", True, None)

    media_type = http_request.headers.get("content-type", "").split(";")[0].strip().lower()
    if media_type != "application/json":
        return _error(415, "unsupported_media_type",
                      "Ожидается Content-Type: application/json", False, None)

    limit = (MAX_REQUEST_BYTES_EXTENDED if _model["artifact"] is not None
             else MAX_REQUEST_BYTES)
    limit_mib = limit // (1024 * 1024)
    declared = http_request.headers.get("content-length")
    if declared and declared.isdigit() and int(declared) > limit:
        return _error(413, "payload_too_large",
                      f"Тело запроса превышает {limit_mib} MiB", False, None)

    body = await http_request.body()
    if len(body) > limit:
        return _error(413, "payload_too_large",
                      f"Тело запроса превышает {limit_mib} MiB", False, None)

    try:
        payload = json.loads(body, parse_constant=_reject_constant)
    except _NonFiniteNumber:
        return _error(422, "invalid_input", "NaN и Infinity запрещены во всех числах",
                      False, None)
    except ValueError:
        return _error(400, "invalid_json", "Тело запроса не является корректным JSON",
                      False, None)
    if not isinstance(payload, dict):
        return _error(400, "invalid_json", "Тело запроса должно быть JSON-объектом",
                      False, None)

    raw_id = payload.get("request_id")
    if isinstance(raw_id, str) and 0 < len(raw_id) <= contracts.MAX_ID_LENGTH:
        request_id = raw_id

    try:
        contracts.check_contract(payload, available_profiles())
    except UnsupportedContract as exc:
        return _error(422, "unsupported_contract", str(exc), False, request_id)

    try:
        request = AnalyzeRequest.model_validate(payload)
    except ValidationError as exc:
        return _error(422, "invalid_input", _validation_message(exc), False, request_id)

    if not _slot.acquire(blocking=False):
        return _error(429, "busy", "Вычислительный слот занят, повторите позже",
                      True, request_id)

    try:
        result = await anyio.to_thread.run_sync(_compute, request)
    except Exception:
        log.exception("расчёт не выполнен, request_id=%s", request_id)
        return _error(500, "internal_error", "Внутренняя ошибка расчёта", True, request_id)

    encoded = json.dumps(result, ensure_ascii=False).encode("utf-8")
    if len(encoded) > MAX_RESPONSE_BYTES:
        log.error("ответ превышает лимит, request_id=%s, байт=%d", request_id, len(encoded))
        return _error(500, "internal_error", "Результат превышает допустимый размер ответа",
                      True, request_id)
    return JSONResponse(status_code=200, content=result)
