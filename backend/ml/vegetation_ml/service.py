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

import hashlib
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

from . import __version__, contracts, paths, web_analysis
from . import model as M
from .contracts import AnalyzeRequest, UnsupportedContract

MAX_REQUEST_BYTES = 1 * 1024 * 1024
MAX_REQUEST_BYTES_EXTENDED = 8 * 1024 * 1024
MAX_RESPONSE_BYTES = 4 * 1024 * 1024

MODEL_PATH_ENV = "VEGETATION_ML_MODEL"
MODEL_MANIFEST_ENV = "VEGETATION_ML_MANIFEST"
ALLOW_FALLBACK_ENV = "VEGETATION_ML_ALLOW_FALLBACK"
ENABLE_MULTISENSOR_ENV = "VEGETATION_ML_ENABLE_MULTISENSOR"

log = logging.getLogger("vegetation_ml.service")

_slot = threading.Lock()
_state = {"ready": False, "reason": "service is starting"}
_model = {"artifact": None}


def _allow_fallback() -> bool:
    return os.environ.get(ALLOW_FALLBACK_ENV, "").strip().lower() in {"1", "true", "yes"}


def _multisensor_enabled() -> bool:
    return os.environ.get(ENABLE_MULTISENSOR_ENV, "").strip().lower() in {"1", "true", "yes"}


def _load_artifact() -> tuple[object | None, str]:
    """Загрузить и проверить артефакт модели один раз на процесс."""
    path = Path(os.environ.get(MODEL_PATH_ENV, paths.MODEL_ARTIFACT))
    if not path.exists():
        reason = f"model artifact not found: {path}"
        log.warning(reason)
        return None, reason
    try:
        manifest_path = Path(os.environ.get(MODEL_MANIFEST_ENV, path.with_name("manifest.json")))
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        expected_sha = manifest.get("sha256")
        actual_sha = hashlib.sha256(path.read_bytes()).hexdigest()
        if expected_sha != actual_sha:
            raise ValueError("model artifact checksum does not match manifest")
        if manifest.get("schema_version") != contracts.SCHEMA_VERSION:
            raise ValueError("model manifest schema_version is unsupported")
        if manifest.get("feature_profile") != contracts.PROFILE_MULTISENSOR:
            raise ValueError("model manifest feature_profile is unsupported")
        if manifest.get("model_version") != web_analysis.MODEL_VERSION_TRAINED:
            raise ValueError("model manifest version does not match the service")
        configured_version = os.environ.get("MODEL_VERSION", "").strip()
        if configured_version and manifest.get("model_version") != configured_version:
            raise ValueError("MODEL_VERSION does not match the model manifest")
        return M.RestorationModel.load(path), ""
    except Exception as exc:
        reason = f"model artifact is invalid: {exc}"
        log.exception("model artifact could not be loaded: %s", path)
        return None, reason


def available_profiles() -> tuple[str, ...]:
    """Профили, которые процесс действительно может обслужить."""
    if _model["artifact"] is None or not _multisensor_enabled():
        return (contracts.FEATURE_PROFILE,)
    return (contracts.FEATURE_PROFILE, contracts.PROFILE_MULTISENSOR)


@asynccontextmanager
async def _lifespan(_: FastAPI):
    """Готовность объявляется после загрузки методов расчёта, а не при импорте."""
    web_analysis.warm_up()
    _model["artifact"], reason = _load_artifact()
    fallback = _allow_fallback()
    _state["ready"] = _model["artifact"] is not None or fallback
    _state["reason"] = "" if _state["ready"] else reason
    if _state["ready"]:
        log.info("service ready, model_version=%s, profiles=%s, fallback=%s",
                 web_analysis.model_version(_model["artifact"]), available_profiles(), fallback)
    else:
        log.error("service is not ready: %s", _state["reason"])
    yield
    _state["ready"] = False
    _state["reason"] = "service stopped"


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
            "reason": _state["reason"] or "model artifact is not loaded",
        })
    return JSONResponse(status_code=200, content={
        "status": "ready",
        "schema_version": contracts.SCHEMA_VERSION,
        "schema_versions": list(
            contracts.SUPPORTED_SCHEMA_VERSIONS
            if _multisensor_enabled()
            else contracts.PRODUCTION_SCHEMA_VERSIONS
        ),
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
        return _error(503, "not_ready", "Model artifact is not loaded", True, None)

    media_type = http_request.headers.get("content-type", "").split(";")[0].strip().lower()
    if media_type != "application/json":
        return _error(415, "unsupported_media_type",
                      "Content-Type must be application/json", False, None)

    transport_limit = MAX_REQUEST_BYTES_EXTENDED
    declared = http_request.headers.get("content-length")
    if declared and declared.isdigit() and int(declared) > transport_limit:
        return _error(413, "payload_too_large",
                      "Request body exceeds 8 MiB", False, None)

    body = await http_request.body()
    if len(body) > transport_limit:
        return _error(413, "payload_too_large",
                      "Request body exceeds 8 MiB", False, None)

    try:
        payload = json.loads(body, parse_constant=_reject_constant)
    except _NonFiniteNumber:
        return _error(422, "invalid_input", "NaN and Infinity are not allowed",
                      False, None)
    except ValueError:
        return _error(400, "invalid_json", "Request body is not valid JSON",
                      False, None)
    if not isinstance(payload, dict):
        return _error(400, "invalid_json", "Request body must be a JSON object",
                      False, None)

    extended = (payload.get("schema_version") == contracts.SCHEMA_VERSION_EXTENDED
                and payload.get("feature_profile") == contracts.PROFILE_MULTISENSOR)
    limit = MAX_REQUEST_BYTES_EXTENDED if extended else MAX_REQUEST_BYTES
    if len(body) > limit:
        return _error(413, "payload_too_large",
                      f"Request body exceeds {limit // (1024 * 1024)} MiB", False, None)

    raw_id = payload.get("request_id")
    if isinstance(raw_id, str) and 0 < len(raw_id) <= contracts.MAX_ID_LENGTH:
        request_id = raw_id

    try:
        supported_versions = (
            contracts.SUPPORTED_SCHEMA_VERSIONS
            if _multisensor_enabled()
            else contracts.PRODUCTION_SCHEMA_VERSIONS
        )
        contracts.check_contract(payload, available_profiles(), supported_versions)
    except UnsupportedContract as exc:
        return _error(422, "unsupported_contract", str(exc), False, request_id)

    try:
        request = AnalyzeRequest.model_validate(payload)
    except ValidationError as exc:
        return _error(422, "invalid_input", _validation_message(exc), False, request_id)

    if not _slot.acquire(blocking=False):
        return _error(429, "busy", "The computation slot is busy; try again later",
                      True, request_id)

    try:
        result = await anyio.to_thread.run_sync(_compute, request)
    except Exception:
        log.exception("calculation failed, request_id=%s", request_id)
        return _error(500, "internal_error", "Internal calculation error", True, request_id)

    encoded = json.dumps(result, ensure_ascii=False).encode("utf-8")
    if len(encoded) > MAX_RESPONSE_BYTES:
        log.error("response exceeds limit, request_id=%s, bytes=%d", request_id, len(encoded))
        return _error(500, "internal_error", "Response exceeds the allowed size",
                      True, request_id)
    return JSONResponse(status_code=200, content=result)
