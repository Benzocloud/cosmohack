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
SCHEMA_VERSION_EXTENDED = "1.1"
SUPPORTED_SCHEMA_VERSIONS = (SCHEMA_VERSION, SCHEMA_VERSION_EXTENDED)

FEATURE_PROFILE = "ndvi-weather-v1"
PROFILE_MULTISENSOR = "ndvi-multisensor-v1"
MODE = "retrospective"

MAX_OBSERVATIONS = 4096
MAX_PEERS = 8
MAX_TOTAL_POINTS = 32768
MAX_ID_LENGTH = 128

SENSOR_INDEX_FIELDS = ("s2_ndvi", "s2_evi", "s2_ndwi",
                       "landsat_ndvi", "landsat_evi", "landsat_ndwi",
                       "modis_ndvi", "modis_evi")
NDVI_PRIORITY = ("s2_ndvi", "landsat_ndvi", "modis_ndvi")
NDVI_MATCH_TOLERANCE = 1e-9

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


class Indices(Strict):
    """Значения отдельных сенсоров за один день.

    Профиль ndvi-multisensor-v1 передаёт их, потому что primary_ndvi собран из
    трёх сенсоров с разной калибровкой, и без знания источника снять эту разницу
    нельзя. Все ключи обязательны, отсутствие значения кодируется null.
    """

    s2_ndvi: float | None
    s2_evi: float | None
    s2_ndwi: float | None
    landsat_ndvi: float | None
    landsat_evi: float | None
    landsat_ndwi: float | None
    modis_ndvi: float | None
    modis_evi: float | None

    @model_validator(mode="after")
    def check_finite(self):
        for name in SENSOR_INDEX_FIELDS:
            _finite(getattr(self, name), f"indices.{name}")
        return self

    def primary(self) -> float | None:
        """Значение по приоритету s2 → landsat → modis."""
        for name in NDVI_PRIORITY:
            value = getattr(self, name)
            if value is not None:
                return value
        return None


class AreaContext(Strict):
    """Свойства участка, не меняющиеся по датам."""

    crop_type: str | None


class PeerObservation(Strict):
    """Точка соседнего участка, нужная только для оценки эффекта даты."""

    date: date
    primary_ndvi: float | None
    quality: Literal["usable", "unusable", "missing"] | None
    indices: Indices | None

    @field_validator("primary_ndvi")
    @classmethod
    def finite(cls, v, info):
        return _finite(v, info.field_name)


class PeerSeries(Strict):
    """Ряд соседнего участка.

    Эффект даты — общий сдвиг наблюдений всех участков в день съёмки — по
    абляции самый весомый признак после опорной оценки. Он вычисляется только
    по соседям, сам участок в него не входит.
    """

    area_id: Id
    observations: list[PeerObservation] = Field(max_length=MAX_OBSERVATIONS)

    @model_validator(mode="after")
    def check_dates(self):
        dates = [o.date for o in self.observations]
        if len(dates) != len(set(dates)):
            raise ValueError(f"даты соседнего участка {self.area_id} должны быть уникальны")
        if any(b <= a for a, b in zip(dates, dates[1:])):
            raise ValueError(f"даты соседнего участка {self.area_id} должны строго возрастать")
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
    indices: Indices | None = None

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
        if self.indices is not None:
            expected = self.indices.primary()
            if self.primary_ndvi is None and expected is not None:
                raise ValueError(
                    f"точка {self.date}: indices содержат наблюдение, а primary_ndvi пуст")
            if self.primary_ndvi is not None and expected is None:
                raise ValueError(
                    f"точка {self.date}: primary_ndvi задан, а все indices пусты")
            if (self.primary_ndvi is not None and expected is not None
                    and abs(self.primary_ndvi - expected) > NDVI_MATCH_TOLERANCE):
                raise ValueError(
                    f"точка {self.date}: primary_ndvi не совпадает с первым доступным "
                    "индексом в порядке s2 → landsat → modis")
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
    area_context: AreaContext | None = None
    peers: list[PeerSeries] | None = Field(default=None, max_length=MAX_PEERS)

    @property
    def is_multisensor(self) -> bool:
        return self.feature_profile == PROFILE_MULTISENSOR

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

        if not self.is_multisensor:
            if any(o.indices is not None for o in self.observations):
                raise ValueError(
                    f"поле indices доступно только в профиле {PROFILE_MULTISENSOR}")
            if self.area_context is not None or self.peers is not None:
                raise ValueError(
                    f"поля area_context и peers доступны только в профиле {PROFILE_MULTISENSOR}")

        if self.peers is not None:
            peer_ids = [p.area_id for p in self.peers]
            if len(peer_ids) != len(set(peer_ids)):
                raise ValueError("идентификаторы соседних участков должны быть уникальны")
            if self.area_id in peer_ids:
                raise ValueError("сам участок не может быть в списке соседей")

        total = len(self.observations) + sum(len(p.observations) for p in (self.peers or []))
        if total > MAX_TOTAL_POINTS:
            raise ValueError(
                f"суммарное число точек {total} превышает предел {MAX_TOTAL_POINTS}")
        return self


class UnsupportedContract(ValueError):
    """Версия схемы, режим или профиль признаков не поддерживаются."""


def check_contract(payload: dict, available_profiles=(FEATURE_PROFILE,)) -> None:
    """Проверяет поля, несовпадение которых даёт unsupported_contract.

    available_profiles — то, что процесс реально может обслужить: расширенный
    профиль требует загруженного артефакта модели. Список совпадает с тем, что
    объявляет GET /readyz, поэтому клиент не может запросить профиль, о котором
    ему не сообщили.
    """
    version = payload.get("schema_version")
    if version not in SUPPORTED_SCHEMA_VERSIONS:
        raise UnsupportedContract(
            "поддерживаются schema_version " + ", ".join(SUPPORTED_SCHEMA_VERSIONS))
    if payload.get("mode") != MODE:
        raise UnsupportedContract(f"поддерживается только mode={MODE}")

    profile = payload.get("feature_profile")
    if profile not in available_profiles:
        raise UnsupportedContract(
            "поддерживаются feature_profile " + ", ".join(available_profiles))
    if profile == PROFILE_MULTISENSOR and version == SCHEMA_VERSION:
        raise UnsupportedContract(
            f"профиль {PROFILE_MULTISENSOR} требует schema_version="
            f"{SCHEMA_VERSION_EXTENDED}")
