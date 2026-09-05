"""Мост между расширенным HTTP-запросом и признаками пакетной модели.

Профиль `ndvi-multisensor-v1` передаёт то же, что есть в CSV соревнования:
значения отдельных сенсоров, культуру участка и ряды соседних участков. Этого
достаточно, чтобы собрать ровно ту же панель и ту же матрицу признаков, на
которых обучалась пакетная модель, и вызвать её без изменений.

Соседние участки попадают в панель как обычные полигоны. Благодаря этому эффект
даты — самый весомый признак после опорной оценки — считается тем же кодом, что
и в пакетном режиме, а не отдельной веб-версией.
"""

from __future__ import annotations

import numpy as np
import pandas as pd

from . import model as M
from . import panel as panel_mod
from . import pipeline
from . import schema as S
from .contracts import AnalyzeRequest

METHOD_MODEL = "gradient_boosting_residual"

MODEL_MIN_SEASONS = 2
MODEL_MIN_OBSERVATIONS = 60


def _row(area_id, obs_date, indices, weather, crop_type, reliable,
         primary_ndvi=None):
    """Одна строка панели из точки запроса.

    Ненадёжная точка (`unusable`/`missing`) попадает в панель как отсутствие
    наблюдения: контракт запрещает считать её измерением, поэтому ни цель, ни
    индексы из неё не берутся.
    """
    row = {"anon_polygon_id": area_id, "date": pd.Timestamp(obs_date),
           "crop_type": crop_type}
    for col in S.SPECTRAL_COLUMNS:
        row[col] = np.nan
    for col in S.WEATHER_COLUMNS:
        row[col] = np.nan
    row[S.TARGET] = np.nan

    if reliable:
        if indices is not None:
            for col in S.SPECTRAL_COLUMNS:
                row[col] = getattr(indices, col, None)
            index_primary = indices.primary()
            row[S.TARGET] = (index_primary if index_primary is not None
                             else primary_ndvi)
        else:
            # Строки соседей могут содержать только уже объединённое основное значение. Оно
            # остаётся допустимым контекстом влияния даты; отсутствующие поля датчиков
            # не должны исключать иначе пригодного соседа.
            row[S.TARGET] = primary_ndvi
    if weather is not None:
        row["era5_temp_c"] = weather.temperature_mean_c
        row["era5_precip_mm"] = weather.precipitation_sum_mm
    return row


def build_context(request: AnalyzeRequest) -> pd.DataFrame:
    """Собирает панельный контекст: сам участок и переданные соседи."""
    crop = request.area_context.crop_type if request.area_context else None
    rows = [_row(request.area_id, o.date, o.indices, o.weather, crop,
                 o.effective_quality == "usable", o.primary_ndvi)
            for o in request.observations]
    for peer in request.peers or []:
        rows.extend(_row(peer.area_id, o.date, o.indices, None, None,
                         (o.quality or "missing") == "usable", o.primary_ndvi)
                    for o in peer.observations)
    return panel_mod._add_time_keys(pd.DataFrame(rows))


def is_model_applicable(request: AnalyzeRequest) -> tuple[bool, str]:
    """Хватает ли контекста, чтобы применять обученную модель.

    Модель опирается на сетки пролётов, сенсорные смещения и сезонную норму.
    На одном сезоне из десятка наблюдений всё это оценивается по шуму, поэтому
    короткая история честно уводится на резервный метод.
    """
    reliable = [o for o in request.observations
                if o.effective_quality == "usable" and o.indices is not None]
    if len(reliable) < MODEL_MIN_OBSERVATIONS:
        return False, (f"надёжных наблюдений с indices {len(reliable)}, "
                       f"нужно не менее {MODEL_MIN_OBSERVATIONS}")
    seasons = {o.date.year for o in reliable}
    if len(seasons) < MODEL_MIN_SEASONS:
        return False, (f"история покрывает сезонов: {len(seasons)}, "
                       f"нужно не менее {MODEL_MIN_SEASONS}")
    return True, ""


def restore_with_model(request: AnalyzeRequest, model: M.RestorationModel):
    """Восстанавливает точки участка обученной пакетной моделью.

    Возвращает значения по точкам запроса, применённый метод и замечания,
    которые попадут в limitations ответа.
    """
    context = pipeline.sort_context(build_context(request))

    key = pd.MultiIndex.from_arrays([context["anon_polygon_id"], context["date"]])
    need = [(request.area_id, pd.Timestamp(o.date)) for o in request.observations
            if o.effective_quality != "usable"]
    target_mask = key.isin(need)

    grids = pipeline.fit_grids([context])
    p, X, _, _ = pipeline.build_matrices(context, target_mask, grids)
    predicted = model.predict(X)
    predicted = np.where(np.isfinite(predicted), predicted, M.fallback_prediction(X))

    by_key = pd.Series(predicted, index=pd.MultiIndex.from_arrays(
        [p["anon_polygon_id"], p["date"]]))
    restored = np.full(len(request.observations), np.nan)
    methods: list[str | None] = [None] * len(request.observations)
    for i, o in enumerate(request.observations):
        if o.effective_quality == "usable":
            continue
        value = by_key.get((request.area_id, pd.Timestamp(o.date)))
        if value is not None and np.isfinite(value):
            restored[i] = float(value)
            methods[i] = METHOD_MODEL

    notes = []
    if not request.peers:
        notes.append("Соседние участки не переданы: эффект даты не используется, "
                     "качество восстановления ниже пакетного режима")
    if request.area_context is None or request.area_context.crop_type is None:
        notes.append("Культура участка не передана, признак культуры не определён")
    elif request.area_context.crop_type not in S.CROP_TYPES:
        notes.append(f"Культура «{request.area_context.crop_type}» не встречалась "
                     "в обучении, признак культуры отмечен как неизвестный")
    return restored, methods, notes
