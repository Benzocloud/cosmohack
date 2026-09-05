from __future__ import annotations

from pathlib import Path

import numpy as np
import pandas as pd

from . import features, masking, metrics, model as model_mod, panel as panel_mod, schema as S


def sort_context(df: pd.DataFrame) -> pd.DataFrame:
    return df.sort_values(["anon_polygon_id", "epoch"]).reset_index(drop=True)


def build_matrices(context: pd.DataFrame, target_mask: np.ndarray,
                   grids: panel_mod.OverpassGrids):
    p = panel_mod.build_panel(context, target_mask)
    offsets, sigma = features.estimate_sensor_offsets(p)
    probs = features.fit_source_probabilities(p, grids)
    X = features.build_features(p, grids, offsets, probs, sigma)
    return p, X, offsets, probs


def fit_grids(frames: list[pd.DataFrame]) -> panel_mod.OverpassGrids:
    return panel_mod.fit_overpass_grids(frames)


def train_on_simulated(train_df: pd.DataFrame, grids: panel_mod.OverpassGrids,
                       seeds=(11, 12, 13), fraction: float = masking.MASK_FRACTION,
                       polygons: set | None = None, seasons: set | None = None,
                       model_seeds=(0, 1, 2), params: dict | None = None):
    """Обучает модель на искусственно скрытых наблюдениях переданного контекста.

    Контекстом может быть не только train: во всех файлах организаторов строки
    вне контрольных содержат открытые значения primary_ndvi, и они пригодны для
    обучения. Контрольные строки в обучение попасть не могут — их значение
    отсутствует, а маскируются только известные наблюдения.
    """
    train_df = sort_context(train_df)
    parts_X, parts_y = [], []
    for s in seeds:
        m = masking.simulate_gaps(train_df, fraction=fraction, seed=s, seasons=seasons)
        if polygons is not None:
            m &= train_df["anon_polygon_id"].isin(polygons).to_numpy()
        _, X, _, _ = build_matrices(train_df, m, grids)
        y = train_df[S.TARGET].to_numpy(dtype=float)
        parts_X.append(X[m])
        parts_y.append(y[m])
    X_all = pd.concat(parts_X, ignore_index=True)
    y_all = np.concatenate(parts_y)
    return model_mod.fit_ensemble(X_all, y_all, seeds=model_seeds, params=params)


def evaluate(train_df: pd.DataFrame, grids: panel_mod.OverpassGrids,
             mdl, mask: np.ndarray) -> dict:
    train_df = sort_context(train_df)
    _, X, _, _ = build_matrices(train_df, mask, grids)
    y = train_df[S.TARGET].to_numpy(dtype=float)
    pred = mdl.predict(X)
    fb = model_mod.fallback_prediction(X)
    pred = np.where(np.isfinite(pred), pred, fb)
    return {"y": y[mask], "pred": pred[mask], "X": X[mask]}


def predict_private(train_path, private_path, model_path=None, out_path="submission.csv",
                    seeds=(11, 12, 13), model_seeds=(0, 1, 2), extra_context=()):
    """Строит ответ по контрольным строкам целевого файла.

    extra_context — дополнительные файлы организаторов того же формата. Они не
    содержат целей этого запуска, но дают контекст: сезонную норму, оценку
    сенсорных смещений и эффект даты по соседним полигонам.
    """
    train_df = panel_mod.load_train(train_path)
    priv = panel_mod.load_private(private_path)
    extras = [panel_mod.load_private(p) for p in extra_context]
    frames = [train_df, priv, *extras]
    grids = fit_grids(frames)
    context = sort_context(panel_mod.concat_context(*frames))

    if model_path and Path(model_path).exists():
        mdl = model_mod.RestorationModel.load(model_path)
    else:
        mdl = train_on_simulated(context, grids, seeds=seeds, model_seeds=model_seeds)
        if model_path:
            mdl.save(model_path)
    key = pd.MultiIndex.from_arrays([context["anon_polygon_id"], context["date"]])
    gap_key = pd.MultiIndex.from_arrays([
        priv.loc[priv[S.GAP_FLAG], "anon_polygon_id"], priv.loc[priv[S.GAP_FLAG], "date"]])
    mask = key.isin(gap_key)
    if int(mask.sum()) != int(priv[S.GAP_FLAG].sum()):
        raise ValueError(
            f"контрольных строк в файле {int(priv[S.GAP_FLAG].sum())}, "
            f"а в собранном контексте найдено {int(mask.sum())}")

    _, X, offsets, probs = build_matrices(context, mask, grids)
    pred = mdl.predict(X)
    fb = model_mod.fallback_prediction(X)
    pred = np.where(np.isfinite(pred), pred, fb)

    sub = pd.DataFrame({
        "anon_polygon_id": context.loc[mask, "anon_polygon_id"].to_numpy(),
        "date": pd.to_datetime(context.loc[mask, "date"]).dt.strftime("%Y-%m-%d"),
        "primary_ndvi_pred": pred[mask],
    })
    validate_submission(sub, priv)
    Path(out_path).parent.mkdir(parents=True, exist_ok=True)
    sub.to_csv(out_path, index=False, encoding="utf-8", lineterminator="\n")
    return sub, mdl


def validate_submission(sub: pd.DataFrame, priv: pd.DataFrame) -> None:
    if list(sub.columns) != S.SUBMISSION_COLUMNS:
        raise ValueError(f"заголовок должен быть {S.SUBMISSION_COLUMNS}, получен {list(sub.columns)}")
    if sub.duplicated(["anon_polygon_id", "date"]).any():
        raise ValueError("в ответе есть дублирующиеся ключи")
    if not np.isfinite(sub["primary_ndvi_pred"].to_numpy(dtype=float)).all():
        raise ValueError("в ответе есть NaN или бесконечности")
    want = priv.loc[priv[S.GAP_FLAG], ["anon_polygon_id", "date"]].copy()
    want["date"] = pd.to_datetime(want["date"]).dt.strftime("%Y-%m-%d")
    a = set(map(tuple, want.to_numpy()))
    b = set(map(tuple, sub[["anon_polygon_id", "date"]].to_numpy()))
    if a != b:
        raise ValueError(
            f"множество ключей не совпадает: пропущено {len(a - b)}, лишних {len(b - a)}")
