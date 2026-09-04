"""Модель восстановления primary_ndvi.

Опорная оценка anchor — сглаженная по сенсорам кривая полигона плюс ожидаемое
смещение того сенсора, который вероятнее всего дал бы наблюдение в этот день.
Бустинг предсказывает не само значение, а остаток от неё: так он занимается
только тем, что опорная оценка объяснить не может, а при отсутствии сигнала
результат не становится хуже неё.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
import json
import pickle

import numpy as np
import pandas as pd
from sklearn.ensemble import HistGradientBoostingRegressor

from . import schema as S

CATEGORICAL = ["src_left_1", "src_right_1", "crop_type"]

HGB_PARAMS = dict(
    loss="squared_error",
    max_iter=1200,
    learning_rate=0.03,
    max_leaf_nodes=63,
    min_samples_leaf=60,
    l2_regularization=2.0,
    max_bins=255,
    early_stopping=True,
    validation_fraction=0.12,
    n_iter_no_change=40,
    random_state=0,
)

CLIP_LOW = -0.2
CLIP_HIGH = 1.0

@dataclass
class RestorationModel:
    """Обученный ансамбль вместе со всем, что нужно для повторного инференса."""

    estimators: list = field(default_factory=list)
    feature_names: list = field(default_factory=list)
    train_rmse: float = float("nan")
    meta: dict = field(default_factory=dict)

    def predict(self, X: pd.DataFrame) -> np.ndarray:
        """Опорная оценка плюс предсказанный остаток, обрезанные по диапазону.

        Строки без опорной оценки остаются NaN: их заполняет резервное
        правило, а не модель."""
        anchor = X["anchor"].to_numpy(dtype=float)
        Z = self._matrix(X)
        residual = np.mean([e.predict(Z) for e in self.estimators], axis=0)
        pred = anchor + residual

        pred = np.where(np.isfinite(anchor), pred, np.nan)
        return np.clip(pred, CLIP_LOW, CLIP_HIGH)

    def _matrix(self, X: pd.DataFrame) -> pd.DataFrame:
        missing = [c for c in self.feature_names if c not in X.columns]
        if missing:
            raise ValueError(f"в матрице признаков нет колонок: {missing}")
        return X[self.feature_names]

    def save(self, path: str | Path) -> None:
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("wb") as fh:
            pickle.dump(self, fh, protocol=5)
        path.with_suffix(".json").write_text(
            json.dumps(self.meta, ensure_ascii=False, indent=2), encoding="utf-8")

    @staticmethod
    def load(path: str | Path) -> "RestorationModel":
        with Path(path).open("rb") as fh:
            return pickle.load(fh)

def feature_columns(X: pd.DataFrame) -> list:
    """Признаки, пригодные для обучения на данном наборе.

    Колонка, у которой в обучающей выборке нет ни одного конечного значения
    или всего одно уникальное, бесполезна и ломает биннинг бустинга. Такое
    случается на коротких рядах: часть признаков там просто не определена.
    """
    out = []
    for c in X.columns:
        if c.startswith("_"):
            continue
        v = pd.to_numeric(X[c], errors="coerce").to_numpy(dtype=float)
        v = v[np.isfinite(v)]
        if v.size and np.unique(v).size > 1:
            out.append(c)
    return out

def fit(X: pd.DataFrame, y: np.ndarray, sample_weight=None,
        params: dict | None = None, seed: int = 0) -> RestorationModel:
    """Обучает бустинг на остатке от опорной оценки."""
    anchor = X["anchor"].to_numpy(dtype=float)
    ok = np.isfinite(anchor) & np.isfinite(y)
    names = feature_columns(X.loc[ok])
    Z = X.loc[ok, names]
    target = y[ok] - anchor[ok]

    p = dict(HGB_PARAMS)
    if params:
        p.update(params)
    p["random_state"] = seed
    p["n_iter_no_change"] = max(5, min(p["n_iter_no_change"], p["max_iter"] // 4))
    if len(Z) < 2000:
        p["early_stopping"] = False
        p.pop("validation_fraction", None)
        p.pop("n_iter_no_change", None)
    cat = [c in CATEGORICAL for c in names]
    est = HistGradientBoostingRegressor(categorical_features=cat, **p)
    est.fit(Z, target, sample_weight=None if sample_weight is None else sample_weight[ok])

    model = RestorationModel(estimators=[est], feature_names=names)
    model.train_rmse = float(np.sqrt(((target - est.predict(Z)) ** 2).mean()))
    model.meta = {"n_train_rows": int(ok.sum()), "n_features": len(names),
                  "params": {k: v for k, v in p.items()}}
    return model

def fit_ensemble(X: pd.DataFrame, y: np.ndarray, seeds=(0, 1, 2),
                 params: dict | None = None) -> RestorationModel:
    """Усредняет несколько бустингов с разными seed.

    Разброс между отдельными обучениями на этих данных заметен, а усреднение
    снимает его практически бесплатно."""
    models = [fit(X, y, params=params, seed=s) for s in seeds]
    merged = RestorationModel(
        estimators=[m.estimators[0] for m in models],
        feature_names=models[0].feature_names,
        train_rmse=float(np.mean([m.train_rmse for m in models])),
        meta={**models[0].meta, "seeds": list(seeds), "n_estimators": len(models)},
    )
    return merged

def fallback_prediction(X: pd.DataFrame) -> np.ndarray:
    """Резервное правило: опорная оценка, затем baseline по соседям, затем
    сезонная норма. Если неизвестно и это, строка остаётся NaN и должна быть
    показана как невосстановимая, а не заполнена придуманным числом."""
    out = X["anchor"].to_numpy(dtype=float).copy()
    for col in ("baseline_corr", "smooth_corr_h16", "clim_mean"):
        if col in X.columns:
            v = X[col].to_numpy(dtype=float)
            out = np.where(np.isfinite(out), out, v)
    return np.clip(out, CLIP_LOW, CLIP_HIGH)
