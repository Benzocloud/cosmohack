"""Выделение негативных периодов вегетации и их интерпретация.

Аномалия здесь — отклонение вниз от сезонной нормы того же полигона в ту же
фазу сезона, а не просто низкий NDVI.

Три ограничения заложены намеренно: восстановленная моделью точка не считается
независимым наблюдением, погодная гипотеза выдаётся только вместе с
наблюдаемыми основаниями и никогда не объявляется причиной, а при короткой
истории возвращается insufficient_data вместо выдуманной нормы.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np
import pandas as pd

Z_STRESS = -1.0
Z_CRITICAL = -2.0

MIN_REFERENCE_YEARS = 3

MIN_PERIOD_DAYS = 5

DRY_RATIO = 0.5

@dataclass
class AnomalyPeriod:

    start: str
    end: str
    days: int
    min_z: float
    mean_z: float
    severity: str
    status: str
    n_observed: int
    n_imputed: int
    grounds: dict = field(default_factory=dict)
    hypotheses: list = field(default_factory=list)
    limitations: list = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "start": self.start, "end": self.end, "days": self.days,
            "min_z": round(self.min_z, 3), "mean_z": round(self.mean_z, 3),
            "severity": self.severity, "status": self.status,
            "n_observed": self.n_observed, "n_imputed": self.n_imputed,
            "grounds": self.grounds, "hypotheses": self.hypotheses,
            "limitations": self.limitations,
        }

def severity_of(z: float) -> str:
    """Тяжесть отклонения по z-оценке относительно сезонной нормы."""
    if not np.isfinite(z) or z >= Z_STRESS:
        return "normal"
    if z >= Z_CRITICAL:
        return "biomass_stress"
    return "critical"

def seasonal_zscore(series: pd.DataFrame, clim_mean: np.ndarray,
                    clim_std: np.ndarray, min_std: float = 0.03) -> np.ndarray:
    """z-оценка относительно нормы той же фазы сезона.

    min_std не даёт крошечному разбросу нормы превратить обычный шум
    измерения в аномалию с z = −8."""
    value = series["value"].to_numpy(dtype=float)
    std = np.where(np.isfinite(clim_std), np.maximum(clim_std, min_std), np.nan)
    return (value - clim_mean) / std

def detect_periods(dates: pd.Series, z: np.ndarray, is_observed: np.ndarray,
                   clim_n: np.ndarray) -> list[AnomalyPeriod]:
    """Собирает подряд идущие дни ниже порога в периоды и назначает статус.

    Период, собранный только из восстановленных значений, получает candidate,
    а не confirmed: иначе модель подтверждает собственную же аномалию."""
    dates = pd.Series(pd.to_datetime(pd.Series(dates).to_numpy())).reset_index(drop=True)
    below = np.isfinite(z) & (z < Z_STRESS)
    periods: list[AnomalyPeriod] = []
    i = 0
    while i < len(below):
        if not below[i]:
            i += 1
            continue
        j = i
        while j < len(below) and below[j]:
            j += 1
        sl = slice(i, j)
        days = int((dates.iloc[j - 1] - dates.iloc[i]).days) + 1
        if days >= MIN_PERIOD_DAYS:
            zz = z[sl]
            n_obs = int(is_observed[sl].sum())
            n_imp = int((~is_observed[sl]).sum())
            enough_norm = np.nanmedian(clim_n[sl]) >= MIN_REFERENCE_YEARS
            if not enough_norm:
                status = "insufficient_data"
            elif n_obs == 0:

                status = "candidate"
            elif n_obs >= 2:
                status = "confirmed"
            else:
                status = "candidate"
            periods.append(AnomalyPeriod(
                start=str(dates.iloc[i].date()), end=str(dates.iloc[j - 1].date()),
                days=days, min_z=float(np.nanmin(zz)), mean_z=float(np.nanmean(zz)),
                severity=severity_of(float(np.nanmin(zz))), status=status,
                n_observed=n_obs, n_imputed=n_imp,
            ))
        i = j
    return periods

def add_weather_context(periods: list[AnomalyPeriod], frame: pd.DataFrame) -> None:
    """Добавляет наблюдаемые погодные основания и осторожные гипотезы.

    Совпадение засушливого окна со спадом не доказывает причину: уборка
    урожая, смена культуры и остаточная облачность объясняют тот же спад."""
    if not len(frame):
        return
    dates = pd.to_datetime(frame["date"])
    precip = frame.get("precip_prev30")
    temp = frame.get("temp_prev30")
    for p in periods:
        m = (dates >= p.start) & (dates <= p.end)
        if not m.any():
            continue
        g = {}
        if precip is not None:
            v = pd.to_numeric(precip[m], errors="coerce")
            norm = pd.to_numeric(precip, errors="coerce").median()
            if v.notna().any():
                g["precip_30d_mm"] = round(float(v.mean()), 2)
                if np.isfinite(norm) and norm > 0:
                    g["precip_vs_median"] = round(float(v.mean() / norm), 2)
        if temp is not None:
            v = pd.to_numeric(temp[m], errors="coerce")
            if v.notna().any():
                g["temp_30d_c"] = round(float(v.mean()), 2)
        p.grounds = g
        ratio = g.get("precip_vs_median")
        if ratio is not None and ratio < DRY_RATIO:
            p.hypotheses.append({
                "hypothesis": "дефицит осадков в предшествующие 30 дней",
                "evidence": f"осадки составили {ratio:.0%} от медианы полигона",
                "confidence": "не подтверждена",
            })
        if p.n_imputed > p.n_observed:
            p.limitations.append(
                "большая часть периода восстановлена моделью, а не измерена")
        p.limitations.append(
            "совпадение погодных условий со спадом не устанавливает причину; "
            "уборка урожая, смена культуры и облачность объясняют его так же")

def analyse_polygon(frame: pd.DataFrame) -> dict:
    """Полный разбор одного полигона за период.

    Ожидает колонки date, value, is_observed, clim_mean, clim_std, clim_n и,
    при наличии, погодные агрегаты. Результат пригоден и для HTTP-ответа,
    и для отчёта."""
    required = {"date", "value", "is_observed", "clim_mean", "clim_std", "clim_n"}
    missing = required - set(frame.columns)
    if missing:
        raise ValueError(f"для разбора аномалий не хватает колонок: {sorted(missing)}")

    z = seasonal_zscore(frame, frame["clim_mean"].to_numpy(dtype=float),
                        frame["clim_std"].to_numpy(dtype=float))
    periods = detect_periods(frame["date"], z, frame["is_observed"].to_numpy(bool),
                             frame["clim_n"].to_numpy(dtype=float))
    add_weather_context(periods, frame)

    if np.isfinite(z).sum() == 0:
        status = "insufficient_data"
    elif any(p.status == "confirmed" for p in periods):
        status = "confirmed"
    elif periods:
        status = "candidate"
    else:
        status = "normal"

    return {
        "status": status,
        "zscore": [None if not np.isfinite(v) else round(float(v), 3) for v in z],
        "periods": [p.to_dict() for p in periods],
        "n_observed": int(frame["is_observed"].sum()),
        "n_imputed": int((~frame["is_observed"].astype(bool)).sum()),
    }
