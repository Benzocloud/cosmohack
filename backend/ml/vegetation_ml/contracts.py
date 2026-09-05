"""Схемы запроса и ответа HTTP-контракта Go ↔ ML v1.

Модели умышленно строгие: `extra="forbid"` отклоняет неизвестные поля, а
проверки согласованности вынесены сюда, чтобы вычислительный код не получал
заведомо некорректный вход. Всё, что контракт называет ошибкой `invalid_input`,
должно падать здесь, а не в середине расчёта.
"""

from __future__ import annotations

import math
import re
from datetime import date, datetime, timezone
from itertools import pairwise
from typing import Annotated, Literal

from pydantic import (
    BaseModel,
    ConfigDict,
    Field,
    StrictInt,
    StrictStr,
    field_validator,
    model_validator,
)

SCHEMA_VERSION = "1.0"
SCHEMA_VERSION_EXTENDED = "1.1"
SUPPORTED_SCHEMA_VERSIONS = (SCHEMA_VERSION, SCHEMA_VERSION_EXTENDED)
PRODUCTION_SCHEMA_VERSIONS = (SCHEMA_VERSION,)

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

Id = Annotated[StrictStr, Field(min_length=1, max_length=MAX_ID_LENGTH)]
NonEmptyText = Annotated[StrictStr, Field(min_length=1)]


class Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


def _finite(value, field: str):
    """Validate a JSON number while preserving integer values for float fields."""
    if value is None:
        return value
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"{field} must be a number or null")  # noqa: TRY004
    if isinstance(value, float) and not math.isfinite(value):
        raise ValueError(f"{field} must be finite")
    return value


def _utc_rfc3339(value: str) -> str:
    """Require an RFC3339 timestamp with an explicit UTC offset."""
    if not isinstance(value, str):
        raise ValueError("retrieved_at must be a string")  # noqa: TRY004
    if not re.fullmatch(
            r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})",
            value):
        raise ValueError("retrieved_at must be an RFC3339 timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError("retrieved_at must be an RFC3339 timestamp") from exc
    if parsed.tzinfo is None or parsed.utcoffset() != timezone.utc.utcoffset(parsed):
        raise ValueError("retrieved_at must use UTC")
    return value


def _strict_date(value, field: str) -> date:
    """Require the wire format YYYY-MM-DD for contract dates."""
    if not isinstance(value, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value):
        raise ValueError(f"{field} must use YYYY-MM-DD")
    try:
        return date.fromisoformat(value)
    except ValueError as exc:
        raise ValueError(f"{field} must use a valid YYYY-MM-DD date") from exc


class Period(Strict):
    from_: date = Field(alias="from")
    to: date

    # The wire contract exposes the alias ``from`` only; accepting the Python
    # attribute name ``from_`` would silently widen the JSON schema.
    model_config = ConfigDict(extra="forbid", populate_by_name=False)

    @field_validator("from_", "to", mode="before")
    @classmethod
    def dates_are_strict(cls, value, info):
        return _strict_date(value, f"period.{info.field_name}")

    @model_validator(mode="after")
    def check_order(self):
        if self.from_ > self.to:
            raise ValueError("period start must not be after period end")
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

    @field_validator(*SENSOR_INDEX_FIELDS, mode="before")
    @classmethod
    def numbers_are_finite(cls, v, info):
        return _finite(v, f"indices.{info.field_name}")

    def primary(self) -> float | None:
        """Значение по приоритету s2 → landsat → modis."""
        for name in NDVI_PRIORITY:
            value = getattr(self, name)
            if value is not None:
                return value
        return None


class AreaContext(Strict):
    """Свойства участка, не меняющиеся по датам."""

    crop_type: StrictStr | None


class PeerObservation(Strict):
    """Точка соседнего участка, нужная только для оценки эффекта даты."""

    date: date
    primary_ndvi: float | None
    quality: Literal["usable", "unusable", "missing"] | None
    indices: Indices | None

    @field_validator("date", mode="before")
    @classmethod
    def date_is_strict(cls, value):
        return _strict_date(value, "peer observation date")

    @field_validator("primary_ndvi", mode="before")
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
            raise ValueError(f"peer area {self.area_id} observation dates must be unique")
        if any(b <= a for a, b in pairwise(dates)):
            raise ValueError(
                f"peer area {self.area_id} observation dates must be strictly ascending")
        return self


class Source(Strict):
    id: Id
    kind: Literal["satellite", "weather", "reference"]
    provider: Id
    dataset: Id
    mapping: NonEmptyText
    retrieved_at: StrictStr
    license: StrictStr | None

    @field_validator("retrieved_at")
    @classmethod
    def retrieved_at_is_utc(cls, v):
        return _utc_rfc3339(v)


class Weather(Strict):
    source_id: Id
    temperature_mean_c: float | None
    precipitation_sum_mm: float | None

    @field_validator("temperature_mean_c", "precipitation_sum_mm", mode="before")
    @classmethod
    def finite(cls, v, info):
        return _finite(v, info.field_name)

    @model_validator(mode="after")
    def check_precip(self):
        if self.precipitation_sum_mm is not None and self.precipitation_sum_mm < 0:
            raise ValueError("precipitation_sum_mm must not be negative")
        return self


class Reference(Strict):
    source_id: Id
    mean: float
    std: float
    n_reference_years: StrictInt
    target_year_excluded: Literal[True]

    @field_validator("mean", "std", mode="before")
    @classmethod
    def numbers_are_finite(cls, v, info):
        return _finite(v, f"reference.{info.field_name}")

    @model_validator(mode="after")
    def check_values(self):
        if self.std < 0:
            raise ValueError("reference.std must not be negative")
        if self.n_reference_years <= 0:
            raise ValueError("reference.n_reference_years must be positive")
        return self


class Observation(Strict):
    date: date
    primary_ndvi: float | None
    quality: Literal["usable", "unusable", "missing"] | None
    ndvi_source_id: Id | None
    interval: Period | None
    valid_fraction: float | None
    missing_reason: StrictStr | None
    weather: Weather | None
    reference: Reference | None
    indices: Indices | None = None

    @field_validator("date", mode="before")
    @classmethod
    def date_is_strict(cls, value):
        return _strict_date(value, "observation date")

    @field_validator("primary_ndvi", "valid_fraction", mode="before")
    @classmethod
    def finite(cls, v, info):
        return _finite(v, info.field_name)

    @model_validator(mode="after")
    def check_point(self):
        quality = self.quality or "missing"
        if self.valid_fraction is not None and not 0.0 <= self.valid_fraction <= 1.0:
            raise ValueError("valid_fraction must be between 0 and 1")
        if quality in ("usable", "unusable"):
            if self.primary_ndvi is None:
                raise ValueError(f"{self.date} quality={quality} requires numeric primary_ndvi")
            if self.ndvi_source_id is None or self.interval is None:
                raise ValueError(f"{self.date} quality={quality} requires source and interval")
            if not self.interval.from_ <= self.date <= self.interval.to:
                raise ValueError(f"{self.date} is outside its aggregation interval")
        if quality == "unusable" and not self.missing_reason:
            raise ValueError(f"{self.date} quality=unusable requires missing_reason")
        if quality == "missing":
            if not self.missing_reason:
                raise ValueError(f"{self.date} quality=missing requires missing_reason")
            if self.primary_ndvi is not None:
                raise ValueError(f"{self.date} quality=missing must not have primary_ndvi")
        if self.indices is not None:
            expected = self.indices.primary()
            if self.primary_ndvi is None and expected is not None:
                raise ValueError(
                    f"{self.date}: indices contain an observation but primary_ndvi is null")
            if self.primary_ndvi is not None and expected is None:
                raise ValueError(f"{self.date}: primary_ndvi is set but all indices are null")
            if (self.primary_ndvi is not None and expected is not None
                    and abs(self.primary_ndvi - expected) > NDVI_MATCH_TOLERANCE):
                raise ValueError(
                    f"{self.date}: primary_ndvi does not match the first available index "
                    "in s2 → landsat → modis order")
        return self

    @property
    def effective_quality(self) -> str:
        return self.quality or "missing"


class AnalyzeRequest(Strict):
    schema_version: Literal[SCHEMA_VERSION, SCHEMA_VERSION_EXTENDED]
    request_id: Id
    area_id: Id
    input_revision: Id
    mode: Literal[MODE]
    feature_profile: Literal[FEATURE_PROFILE, PROFILE_MULTISENSOR]
    analysis_period: Period
    sources: list[Source]
    observations: list[Observation] = Field(min_length=1, max_length=MAX_OBSERVATIONS)
    area_context: AreaContext | None = None
    peers: list[PeerSeries] | None = Field(default=None, max_length=MAX_PEERS)

    @property
    def is_multisensor(self) -> bool:
        return self.feature_profile == PROFILE_MULTISENSOR

    @model_validator(mode="after")
    def check_consistency(self):
        ids = [s.id for s in self.sources]
        if len(ids) != len(set(ids)):
            raise ValueError("source IDs must be unique")
        kinds = {s.id: s.kind for s in self.sources}

        dates = [o.date for o in self.observations]
        if len(dates) != len(set(dates)):
            raise ValueError("observation dates must be unique")
        if any(b <= a for a, b in pairwise(dates)):
            raise ValueError("observation dates must be strictly ascending")

        for o in self.observations:
            if o.ndvi_source_id is not None and kinds.get(o.ndvi_source_id) != "satellite":
                raise ValueError(
                    f"ndvi_source_id {o.ndvi_source_id} does not reference a satellite source")
            if o.weather is not None and kinds.get(o.weather.source_id) != "weather":
                raise ValueError(
                    f"weather.source_id {o.weather.source_id} does not reference a weather source")
            if o.reference is not None and kinds.get(o.reference.source_id) != "reference":
                raise ValueError(
                    f"reference.source_id {o.reference.source_id} does not reference a reference source")

        inside = [d for d in dates if self.analysis_period.from_ <= d <= self.analysis_period.to]
        if not inside:
            raise ValueError("analysis_period must contain at least one observation")

        if not self.is_multisensor:
            if any(o.indices is not None for o in self.observations):
                raise ValueError(
                    f"indices is only available in profile {PROFILE_MULTISENSOR}")
            if self.area_context is not None or self.peers is not None:
                raise ValueError(
                    f"area_context and peers are only available in profile {PROFILE_MULTISENSOR}")

        if self.peers is not None:
            peer_ids = [p.area_id for p in self.peers]
            if len(peer_ids) != len(set(peer_ids)):
                raise ValueError("peer area IDs must be unique")
            if self.area_id in peer_ids:
                raise ValueError("the analyzed area must not be listed as a peer")

        total = len(self.observations) + sum(len(p.observations) for p in (self.peers or []))
        if total > MAX_TOTAL_POINTS:
            raise ValueError(
                f"total observation points {total} exceed the limit of {MAX_TOTAL_POINTS}")
        return self


class UnsupportedContract(ValueError):
    """Версия схемы, режим или профиль признаков не поддерживаются."""


def check_contract(payload: dict, available_profiles=(FEATURE_PROFILE,),
                   supported_versions=SUPPORTED_SCHEMA_VERSIONS) -> None:
    """Проверяет поля, несовпадение которых даёт unsupported_contract.

    available_profiles — то, что процесс реально может обслужить: расширенный
    профиль требует загруженного артефакта модели. Список совпадает с тем, что
    объявляет GET /readyz, поэтому клиент не может запросить профиль, о котором
    ему не сообщили.
    """
    version = payload.get("schema_version")
    if version not in supported_versions:
        raise UnsupportedContract(
            "supported schema_version values: " + ", ".join(supported_versions))
    if payload.get("mode") != MODE:
        raise UnsupportedContract(f"only mode={MODE} is supported")

    profile = payload.get("feature_profile")
    if profile not in available_profiles:
        raise UnsupportedContract(
            "supported feature_profile values: " + ", ".join(available_profiles))
    if profile == PROFILE_MULTISENSOR and version == SCHEMA_VERSION:
        raise UnsupportedContract(
            f"profile {PROFILE_MULTISENSOR} requires schema_version="
            f"{SCHEMA_VERSION_EXTENDED}")
