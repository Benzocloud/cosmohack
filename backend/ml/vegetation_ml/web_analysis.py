"""Расчёт веб-анализа одного участка по HTTP-контракту v1.

Веб-вход и пакетный инференс пользуются общими функциями восстановления и
детекции: сглаживание берётся из `features`, разбор периодов — из `anomalies`.
Различаются только адаптеры входа и доступность признаков.

Профиль `ndvi-weather-v1` не даёт ни спутниковых индексов по сенсорам, ни сеток
пролётов, ни соседних полигонов, поэтому обученный на CSV бустинг здесь
неприменим. Используется явно обозначенный резервный метод — сглаживание ряда
наблюдений, — а ограничение указывается в ответе, а не умалчивается.
"""

from __future__ import annotations

from datetime import date

import numpy as np
import pandas as pd

from . import __version__, anomalies, web_features
from .contracts import PROFILE_MULTISENSOR, AnalyzeRequest
from .features import _gauss_at

MODEL_VERSION = f"ndvi-gapfill-{__version__}+fallback"
MODEL_VERSION_TRAINED = f"ndvi-gapfill-{__version__}+hgb"
METHOD_SMOOTH = "gaussian_smoothing_h8"
METHOD_NEIGHBOURS = "nearest_neighbour_mean"
METHOD_NONE = "no_estimate"

SMOOTH_BANDWIDTH = 8.0
MIN_NEIGHBOURS_FOR_SMOOTHING = 3
MAX_EXTRAPOLATION_DAYS = 16
OWN_BASELINE_MIN_YEARS = 3
OWN_BASELINE_DOY_WINDOW = 10

SEVERITY_BY_DETECTOR = {"biomass_stress": "moderate", "critical": "high"}
SEVERITY_ORDER = {"none": 0, "moderate": 1, "high": 2}


def warm_up() -> None:
    """Прогоняет расчёт на крошечном ряде, чтобы готовность не объявлялась до
    того, как код действительно работоспособен."""
    probe = np.array([0, 4, 8])
    values = np.array([0.4, np.nan, 0.5])
    reliable = np.array([True, False, True])
    _restore(probe, values, reliable)


def _to_epoch(d: date) -> int:
    return (d - date(2010, 1, 1)).days


def _reliable(observations) -> np.ndarray:
    """Надёжным считается только quality=usable: unusable сохраняет значение,
    но по контракту не является наблюдением."""
    return np.array([o.effective_quality == "usable" and o.primary_ndvi is not None
                     for o in observations], dtype=bool)


def _restore(epochs: np.ndarray, values: np.ndarray, reliable: np.ndarray):
    """Восстанавливает ряд по надёжным наблюдениям.

    Возвращает значения и применённый к каждой точке метод. Точка без
    достаточного контекста остаётся без оценки: выдавать число там, где ряд
    нечем поддержать, хуже, чем честно вернуть missing.
    """
    n = len(epochs)
    out = np.full(n, np.nan)
    method: list[str | None] = [None] * n
    ke = epochs[reliable]
    kv = values[reliable]
    if len(ke) == 0:
        return out, method

    smooth, count = _gauss_at(epochs, ke, kv, (SMOOTH_BANDWIDTH,))
    pos = np.searchsorted(ke, epochs)
    for i in range(n):
        if reliable[i]:
            continue
        left = pos[i] - 1
        right = pos[i]
        while right < len(ke) and ke[right] <= epochs[i]:
            right += 1
        has_left = left >= 0
        has_right = right < len(ke)
        gap_left = epochs[i] - ke[left] if has_left else None
        gap_right = ke[right] - epochs[i] if has_right else None

        if not has_left and not has_right:
            continue
        if not has_left and gap_right > MAX_EXTRAPOLATION_DAYS:
            continue
        if not has_right and gap_left > MAX_EXTRAPOLATION_DAYS:
            continue

        if count[i] >= MIN_NEIGHBOURS_FOR_SMOOTHING and np.isfinite(smooth[i, 0]):
            out[i] = smooth[i, 0]
            method[i] = METHOD_SMOOTH
        elif has_left and has_right:
            out[i] = (kv[left] + kv[right]) / 2
            method[i] = METHOD_NEIGHBOURS
        else:
            out[i] = kv[left] if has_left else kv[right]
            method[i] = METHOD_NEIGHBOURS
    return out, method


def _own_baseline(dates, values, reliable):
    """Сезонный фон по собственной истории запроса, если она достаточна.

    Используется, только когда Go не передал проверенный `reference`. История
    берётся из тех же наблюдений запроса, но год самой точки исключается.
    """
    years = np.array([d.year for d in dates])
    doys = np.array([d.timetuple().tm_yday for d in dates])
    mean = np.full(len(dates), np.nan)
    std = np.full(len(dates), np.nan)
    n_years = np.zeros(len(dates))
    if reliable.sum() < OWN_BASELINE_MIN_YEARS:
        return mean, std, n_years
    r_doy, r_year, r_val = doys[reliable], years[reliable], values[reliable]
    for i in range(len(dates)):
        m = (np.abs(r_doy - doys[i]) <= OWN_BASELINE_DOY_WINDOW) & (r_year != years[i])
        uniq = np.unique(r_year[m])
        if len(uniq) >= OWN_BASELINE_MIN_YEARS:
            mean[i] = r_val[m].mean()
            std[i] = r_val[m].std()
            n_years[i] = len(uniq)
    return mean, std, n_years


def _weather_windows(dates, temps, precips):
    """Средняя температура и сумма осадков за 30 дней до точки."""
    n = len(dates)
    epochs = np.array([_to_epoch(d) for d in dates])
    t30 = np.full(n, np.nan)
    p30 = np.full(n, np.nan)
    for i in range(n):
        m = (epochs >= epochs[i] - 30) & (epochs < epochs[i])
        tv = temps[m]
        pv = precips[m]
        tv = tv[np.isfinite(tv)]
        pv = pv[np.isfinite(pv)]
        if tv.size:
            t30[i] = tv.mean()
        if pv.size:
            p30[i] = pv.sum()
    return t30, p30


def model_version(model) -> str:
    """Версия набора методов процесса. Постоянна между запросами, чтобы Go мог
    сверить её с манифестом выпуска; что применялось в конкретном запросе,
    сообщает поле `method`."""
    return MODEL_VERSION_TRAINED if model is not None else MODEL_VERSION


def _choose_restoration(request: AnalyzeRequest, model, epochs, values, reliable):
    """Выбирает путь восстановления и объясняет выбор.

    Обученная модель применяется только на расширенном профиле, при загруженном
    артефакте и достаточной истории участка. Во всех остальных случаях работает
    резервный метод, а причина попадает в limitations: выдавать результат
    резервного метода за результат модели нельзя.
    """
    notes: list[str] = []
    if not request.is_multisensor:
        notes.append(
            f"Профиль {request.feature_profile} не содержит признаков по отдельным "
            f"сенсорам: применён резервный метод восстановления. Обученная модель "
            f"доступна в профиле {PROFILE_MULTISENSOR}")
        return (*_restore(epochs, values, reliable), notes)
    if model is None:
        notes.append("Артефакт модели не загружен, применён резервный метод")
        return (*_restore(epochs, values, reliable), notes)

    applicable, reason = web_features.is_model_applicable(request)
    if not applicable:
        notes.append(f"Контекста недостаточно для обученной модели ({reason}), "
                     "применён резервный метод")
        return (*_restore(epochs, values, reliable), notes)

    restored, methods, model_notes = web_features.restore_with_model(request, model)
    fallback, fallback_methods = _restore(epochs, values, reliable)
    gap = ~np.isfinite(restored) & np.isfinite(fallback)
    if gap.any():
        restored = np.where(gap, fallback, restored)
        methods = [fallback_methods[i] if gap[i] else m for i, m in enumerate(methods)]
        model_notes.append(f"Точек, восстановленных резервным методом вместо модели: "
                           f"{int(gap.sum())}")
    return restored, methods, notes + model_notes


def analyse(request: AnalyzeRequest, model=None) -> dict:
    """Строит ответ контракта по проверенному запросу."""
    obs = request.observations
    dates = [o.date for o in obs]
    epochs = np.array([_to_epoch(d) for d in dates])
    values = np.array([np.nan if o.primary_ndvi is None else float(o.primary_ndvi) for o in obs])
    reliable = _reliable(obs)

    restored, methods, limitations = _choose_restoration(
        request, model, epochs, values, reliable)

    ref_mean = np.full(len(obs), np.nan)
    ref_std = np.full(len(obs), np.nan)
    ref_n = np.zeros(len(obs))
    for i, o in enumerate(obs):
        if o.reference is not None:
            ref_mean[i] = o.reference.mean
            ref_std[i] = o.reference.std
            ref_n[i] = o.reference.n_reference_years
    if not np.isfinite(ref_mean).any():
        ref_mean, ref_std, ref_n = _own_baseline(dates, values, reliable)
        if np.isfinite(ref_mean).any():
            limitations.append(
                "Сезонный фон построен самим ML по наблюдениям запроса "
                f"(окно ±{OWN_BASELINE_DOY_WINDOW} дней по дню года, год точки исключён), "
                "проверенная климатология источником не передана")

    final = np.where(reliable, values, restored)
    z = np.full(len(obs), np.nan)
    usable_norm = np.isfinite(ref_mean) & np.isfinite(ref_std) & (ref_std > 0)
    z[usable_norm] = (final[usable_norm] - ref_mean[usable_norm]) / ref_std[usable_norm]

    inside = np.array([request.analysis_period.from_ <= d <= request.analysis_period.to
                       for d in dates])

    series = []
    for i, o in enumerate(obs):
        if not inside[i]:
            continue
        if reliable[i]:
            state, value, method = "observed", float(values[i]), None
        elif np.isfinite(restored[i]):
            state, value, method = "imputed", float(restored[i]), methods[i]
        else:
            state, value, method = "missing", None, None
        series.append({
            "date": o.date.isoformat(),
            "primary_ndvi": None if o.primary_ndvi is None else float(o.primary_ndvi),
            "value": value,
            "state": state,
            "method": method,
            "baseline": float(ref_mean[i]) if np.isfinite(ref_mean[i]) else None,
            "z_score": float(z[i]) if np.isfinite(z[i]) else None,
        })

    temps = np.array([o.weather.temperature_mean_c if o.weather and
                      o.weather.temperature_mean_c is not None else np.nan for o in obs])
    precips = np.array([o.weather.precipitation_sum_mm if o.weather and
                        o.weather.precipitation_sum_mm is not None else np.nan for o in obs])
    t30, p30 = _weather_windows(dates, temps, precips)

    idx = np.where(inside)[0]
    events = []
    if len(idx):
        periods = anomalies.detect_periods(
            pd.Series([dates[i] for i in idx]), z[idx], reliable[idx], ref_n[idx])
        frame = pd.DataFrame({
            "date": [dates[i] for i in idx],
            "temp_prev30": t30[idx],
            "precip_prev30": p30[idx],
        })
        anomalies.add_weather_context(periods, frame)
        for p in periods:
            if p.status == "insufficient_data":
                limitations.append(
                    f"Период {p.start}–{p.end} не оценён: менее "
                    f"{anomalies.MIN_REFERENCE_YEARS} опорных лет сезонного фона")
                continue
            ev_dates = [dates[i].isoformat() for i in idx
                        if p.start <= dates[i].isoformat() <= p.end and reliable[i]]
            facts = [f"минимальный z-score {p.min_z:.2f}",
                     f"надёжных наблюдений в периоде: {p.n_observed}",
                     f"восстановленных точек в периоде: {p.n_imputed}"]
            if "precip_30d_mm" in p.grounds:
                facts.append(f"осадки за 30 дней: {p.grounds['precip_30d_mm']} мм")
            if "temp_30d_c" in p.grounds:
                facts.append(f"средняя температура за 30 дней: {p.grounds['temp_30d_c']} °C")
            hypothesis = p.hypotheses[0]["hypothesis"] if p.hypotheses else None
            events.append({
                "start_date": p.start,
                "end_date": p.end,
                "status": "confirmed" if (p.status == "confirmed" and ev_dates) else "candidate",
                "severity": SEVERITY_BY_DETECTOR.get(p.severity, "moderate"),
                "min_z_score": float(p.min_z),
                "evidence_dates": ev_dates,
                "facts": facts,
                "hypothesis": hypothesis,
                "limitations": p.limitations,
            })

    n_missing = sum(1 for s in series if s["state"] == "missing")
    if n_missing:
        limitations.append(f"Не восстановлено точек: {n_missing} из {len(series)} — "
                           "недостаточно надёжных наблюдений рядом")
    if not np.isfinite(z[inside]).any():
        limitations.append("Сезонный фон неизвестен или имеет нулевую дисперсию, "
                           "z-score не определён")

    if not np.isfinite(z[inside]).any():
        status, severity = "insufficient_data", None
    elif events:
        status = "confirmed" if any(e["status"] == "confirmed" for e in events) else "candidate"
        severity = max((e["severity"] for e in events), key=lambda s: SEVERITY_ORDER[s])
    else:
        status, severity = "normal", "none"

    used = [m for m in methods if m]
    for candidate in (web_features.METHOD_MODEL, METHOD_SMOOTH, METHOD_NEIGHBOURS):
        if candidate in used:
            method_used = candidate
            break
    else:
        method_used = METHOD_NONE

    return {
        "schema_version": request.schema_version,
        "request_id": request.request_id,
        "area_id": request.area_id,
        "input_revision": request.input_revision,
        "mode": request.mode,
        "feature_profile": request.feature_profile,
        "model_version": model_version(model),
        "method": method_used,
        "status": status,
        "severity": severity,
        "series": series,
        "events": events,
        "limitations": limitations,
    }
