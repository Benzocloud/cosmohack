from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd

from . import schema as S

def _add_time_keys(df: pd.DataFrame) -> pd.DataFrame:
    """Пересчитывает year и doy из даты, не доверяя одноимённым колонкам:
    на контрольных строках они обнулены вместе со всем остальным."""
    d = df["date"]
    df["epoch"] = (d - pd.Timestamp(S.EPOCH_ORIGIN)).dt.days.astype("int32")
    df["year_"] = d.dt.year.astype("int16")
    df["doy_"] = d.dt.dayofyear.astype("int16")

    df["season"] = df["year_"]
    return df

def load_train(path: str | Path) -> pd.DataFrame:
    df = pd.read_csv(path, parse_dates=["date"])
    missing = [c for c in S.KEY + [S.TARGET] if c not in df.columns]
    if missing:
        raise ValueError(f"в train отсутствуют обязательные колонки: {missing}")
    return _add_time_keys(df)

def load_private(path: str | Path) -> pd.DataFrame:
    df = pd.read_csv(path, parse_dates=["date"])
    if S.GAP_FLAG not in df.columns:
        raise ValueError(f"в private нет колонки {S.GAP_FLAG}")
    df[S.GAP_FLAG] = df[S.GAP_FLAG].astype(str).str.lower().isin(["true", "1"])
    return _add_time_keys(df)

def observed_source(df: pd.DataFrame) -> pd.Series:
    """Определяет сенсор, давший primary_ndvi: код 0/1/2 или -1, если строка
    без значения."""
    code = pd.Series(-1, index=df.index, dtype="int8")
    known = df[S.TARGET].notna()
    for i, col in reversed(list(enumerate(S.SENSOR_PRIORITY))):
        if col in df.columns:
            code[known & df[col].notna()] = i
    return code

@dataclass
class OverpassGrids:
    """Остатки epoch по модулю периода, на которых сенсор вообще появляется.

    Сетка задана геометрией витка, одинакова во все годы и восстанавливается
    по любым строкам признаков, включая private. Скрытую цель не использует."""

    residues: dict[tuple[str, str], set[int]]

    def flags(self, polygon: str, epoch: np.ndarray) -> dict[str, np.ndarray]:
        out = {}
        for col, period in S.SENSOR_PERIOD.items():
            grid = self.residues.get((polygon, col), set())
            out[col] = np.isin(epoch % period, list(grid)) if grid else np.zeros(len(epoch), bool)
        return out

def fit_overpass_grids(frames: list[pd.DataFrame], min_share: float = 0.02) -> OverpassGrids:
    """Восстанавливает сетки пролётов по объединению доступных наблюдений.

    min_share отсекает единичные кадры соседнего витка."""
    cat = pd.concat(
        [f[["anon_polygon_id", "epoch"] + [c for c in S.SENSOR_PERIOD if c in f.columns]] for f in frames],
        ignore_index=True,
    )
    residues: dict[tuple[str, str], set[int]] = {}
    for polygon, g in cat.groupby("anon_polygon_id", sort=False):
        for col, period in S.SENSOR_PERIOD.items():
            if col not in g.columns:
                continue
            e = g.loc[g[col].notna(), "epoch"]
            if len(e) == 0:
                residues[(polygon, col)] = set()
                continue
            share = (e % period).value_counts(normalize=True)
            residues[(polygon, col)] = set(int(x) for x in share.index[share >= min_share])
    return OverpassGrids(residues)

PANEL_COLUMNS = (
    ["anon_polygon_id", "date", "epoch", "year_", "doy_", "season", "crop_type"]
    + S.SPECTRAL_COLUMNS + S.WEATHER_COLUMNS + [S.TARGET, "src", "is_target"]
)

def build_panel(df: pd.DataFrame, target_mask: np.ndarray) -> pd.DataFrame:
    """Собирает панель и скрывает на целевых строках всё, что скрывают
    организаторы, а не только primary_ndvi.

    Иначе модель на обучении увидит спутниковые индексы и погоду того же дня,
    которых на контроле не будет, и локальная оценка станет недостижимой."""
    p = df.copy()
    p["src"] = observed_source(p)
    p["is_target"] = np.asarray(target_mask, bool)

    hide = [c for c in S.MASKED_ON_GAP if c in p.columns]
    p.loc[p["is_target"], hide] = np.nan
    p.loc[p["is_target"], "src"] = -1

    for c in PANEL_COLUMNS:
        if c not in p.columns:
            p[c] = np.nan
    p = p[PANEL_COLUMNS].sort_values(["anon_polygon_id", "epoch"]).reset_index(drop=True)
    return p

CONTEXT_COLUMNS = (
    ["anon_polygon_id", "date", "epoch", "year_", "doy_", "season", "crop_type"]
    + S.SPECTRAL_COLUMNS + S.WEATHER_COLUMNS + [S.TARGET]
)


def concat_context(*frames: pd.DataFrame) -> pd.DataFrame:
    """Объединяет выданные организаторами файлы в один контекст по полигонам.

    Файлы дополняют друг друга, а не дублируются: один и тот же полигон может
    иметь историю в одном файле и отдельный сезон в другом, и без склейки
    теряются сезонная норма, сенсорные смещения и эффект даты.

    Совпадение ключа (полигон, дата) между файлами означало бы либо дубль, либо
    раскрытие скрытого значения, поэтому оно не игнорируется, а прерывает
    работу: молча взять одну из двух версий строки хуже, чем остановиться.
    """
    parts = []
    for frame in frames:
        if frame is None or not len(frame):
            continue
        missing = [c for c in CONTEXT_COLUMNS if c not in frame.columns]
        if missing:
            raise ValueError(f"в источнике контекста нет колонок: {missing}")
        parts.append(frame[CONTEXT_COLUMNS].copy())
    if not parts:
        raise ValueError("не передано ни одного источника контекста")
    merged = pd.concat(parts, ignore_index=True)
    duplicated = merged.duplicated(["anon_polygon_id", "date"])
    if duplicated.any():
        example = merged.loc[duplicated, ["anon_polygon_id", "date"]].iloc[0]
        raise ValueError(
            f"источники контекста пересекаются по ключу: повторов {int(duplicated.sum())}, "
            f"например {example.anon_polygon_id} {example.date.date()}")
    return merged
