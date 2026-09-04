"""Схемы запроса и ответа HTTP-контракта Go ↔ ML v1.

Модели умышленно строгие: `extra="forbid"` отклоняет неизвестные поля, а
проверки согласованности вынесены сюда, чтобы вычислительный код не получал
заведомо некорректный вход. Всё, что контракт называет ошибкой `invalid_input`,
должно падать здесь, а не в середине расчёта.
"""

from __future__ import annotations

import math
from datetime import date
from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

SCHEMA_VERSION = "1.0"
FEATURE_PROFILE = "ndvi-weather-v1"
MODE = "retrospective"

MAX_OBSERVATIONS = 4096
MAX_ID_LENGTH = 128

Id = Annotated[str, Field(min_length=1, max_length=MAX_ID_LENGTH)]


class Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


def _finite(value, field: str):
    """NaN и Inf запрещены во всех числах контракта."""
    if value is not None and isinstance(value, float) and not math.isfinite(value):
        raise ValueError(f"поле {field} должно быть конечным числом")
    return value


class Period(Strict):
    from_: date = Field(alias="from")
    to: date

    model_config = ConfigDict(extra="forbid", populate_by_name=True)

    @model_validator(mode="after")
    def check_order(self):
        if self.from_ > self.to:
            raise ValueError("начало периода позже конца")
        return self


class Source(Strict):
    id: Id
    kind: Literal["satellite", "weather", "reference"]
    provider: Id
    dataset: Id
    mapping: str
    retrieved_at: str
    license: str | None


class Weather(Strict):
    source_id: Id
    temperature_mean_c: float | None
    precipitation_sum_mm: float | None

    @field_validator("temperature_mean_c", "precipitation_sum_mm")
    @classmethod
    def finite(cls, v, info):
        return _finite(v, info.field_name)

    @model_validator(mode="after")
    def check_precip(self):
        if self.precipitation_sum_mm is not None and self.precipitation_sum_mm < 0:
            raise ValueError("осадки не могут быть отрицательными")
        return self


class Reference(Strict):
    source_id: Id
    mean: float
    std: float
    n_reference_years: int
    target_year_excluded: Literal[True]

    @model_validator(mode="after")
    def check_values(self):
        _finite(self.mean, "reference.mean")
        _finite(self.std, "reference.std")
        if self.std < 0:
            raise ValueError("reference.std не может быть отрицательным")
        if self.n_reference_years <= 0:
            raise ValueError("reference.n_reference_years должно быть положительным")
        return self


class Observation(Strict):
    date: date
    primary_ndvi: float | None
    quality: Literal["usable", "unusable", "missing"] | None
    ndvi_source_id: str | None
    interval: Period | None
    valid_fraction: float | None
    missing_reason: str | None
    weather: Weather | None
    reference: Reference | None

    @field_validator("primary_ndvi", "valid_fraction")
    @classmethod
    def finite(cls, v, info):
        return _finite(v, info.field_name)

    @model_validator(mode="after")
    def check_point(self):
        quality = self.quality or "missing"
        if self.valid_fraction is not None and not 0.0 <= self.valid_fraction <= 1.0:
            raise ValueError("valid_fraction должен лежать в диапазоне от 0 до 1")
        if quality in ("usable", "unusable"):
            if self.primary_ndvi is None:
                raise ValueError(f"точка {self.date} с quality={quality} требует числовой primary_ndvi")
            if self.ndvi_source_id is None or self.interval is None:
                raise ValueError(f"точка {self.date} с quality={quality} требует источник и интервал")
            if not self.interval.from_ <= self.date <= self.interval.to:
                raise ValueError(f"дата {self.date} лежит вне своего интервала агрегации")
        if quality == "unusable" and not self.missing_reason:
            raise ValueError(f"точка {self.date} с quality=unusable требует missing_reason")
        if quality == "missing":
            if not self.missing_reason:
                raise ValueError(f"точка {self.date} с quality=missing требует missing_reason")
            if self.primary_ndvi is not None:
                raise ValueError(f"точка {self.date} с quality=missing не может иметь primary_ndvi")
        return self

    @property
    def effective_quality(self) -> str:
        return self.quality or "missing"


class AnalyzeRequest(Strict):
    schema_version: str
    request_id: Id
    area_id: Id
    input_revision: Id
    mode: str
    feature_profile: str
    analysis_period: Period
    sources: list[Source]
    observations: list[Observation] = Field(max_length=MAX_OBSERVATIONS)

    @model_validator(mode="after")
    def check_consistency(self):
        ids = [s.id for s in self.sources]
        if len(ids) != len(set(ids)):
            raise ValueError("идентификаторы источников должны быть уникальны")
        kinds = {s.id: s.kind for s in self.sources}

        dates = [o.date for o in self.observations]
        if len(dates) != len(set(dates)):
            raise ValueError("даты наблюдений должны быть уникальны")
        if any(b <= a for a, b in zip(dates, dates[1:])):
            raise ValueError("даты наблюдений должны строго возрастать")

        for o in self.observations:
            if o.ndvi_source_id is not None and kinds.get(o.ndvi_source_id) != "satellite":
                raise ValueError(f"ndvi_source_id {o.ndvi_source_id} не ссылается на источник satellite")
            if o.weather is not None and kinds.get(o.weather.source_id) != "weather":
                raise ValueError(f"weather.source_id {o.weather.source_id} не ссылается на источник weather")
            if o.reference is not None and kinds.get(o.reference.source_id) != "reference":
                raise ValueError(f"reference.source_id {o.reference.source_id} не ссылается на источник reference")

        inside = [d for d in dates if self.analysis_period.from_ <= d <= self.analysis_period.to]
        if not inside:
            raise ValueError("в analysis_period не попадает ни одна переданная точка")
        return self


class UnsupportedContract(ValueError):
    """Версия схемы, режим или профиль признаков не поддерживаются."""


def check_contract(payload: dict) -> None:
    """Проверяет поля, несовпадение которых даёт unsupported_contract, а не invalid_input."""
    if payload.get("schema_version") != SCHEMA_VERSION:
        raise UnsupportedContract(
            f"поддерживается только schema_version={SCHEMA_VERSION}")
    if payload.get("mode") != MODE:
        raise UnsupportedContract(f"поддерживается только mode={MODE}")
    if payload.get("feature_profile") != FEATURE_PROFILE:
        raise UnsupportedContract(
            f"поддерживается только feature_profile={FEATURE_PROFILE}")
