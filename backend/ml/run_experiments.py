"""Локальная проверка методов восстановления NDVI.

Проверка идёт в двух режимах, которые встречаются в private_features:

  * новые полигоны — оценка по отложенным полигонам (GroupKFold по полигону).
    Так проверяются 39 из 78 полигонов, у которых нет истории в train;
  * новый сезон — обучение на сезонах до 2023 включительно, проверка на
    искусственных пропусках сезона 2024. Так проверяются остальные 39
    полигонов, у которых в private только сезон 2025.

Результат пишется в reports/experiments.csv. Это локальная оценка на
искусственной маске; официальный результат платформы считается отдельно и
здесь не заменяется.
"""

from __future__ import annotations

import argparse
import sys
import time
from pathlib import Path

import numpy as np
import pandas as pd

sys.path.insert(0, str(Path(__file__).resolve().parent))

from vegetation_ml import masking, metrics, model as M, panel, paths, pipeline


def build_masked_sets(train_df, grids, seeds):
    Xs, Ms = [], []
    for s in seeds:
        m = masking.simulate_gaps(train_df, seed=s)
        _, X, _, _ = pipeline.build_matrices(train_df, m, grids)
        Xs.append(X)
        Ms.append(m)
    return Xs, Ms


def reference_predictions(X):
    base = X["baseline_corr"].to_numpy() + X["expected_offset"].to_numpy()
    return {
        "baseline организаторов": X["val_left_1"].to_numpy() * 0.5 + X["val_right_1"].to_numpy() * 0.5,
        "baseline + смещение сенсора": base,
        "линейная интерполяция": X["linear_corr"].to_numpy() + X["expected_offset"].to_numpy(),
        "сглаживание + смещение (anchor)": X["anchor"].to_numpy(),
    }


def run_polygon_cv(train_df, grids, Xs, Ms, y, polys, n_folds=5, model_seeds=(0, 1)):
    uniq = np.array(sorted(set(polys)))
    rng = np.random.default_rng(0)
    rng.shuffle(uniq)
    folds = np.array_split(uniq, n_folds)
    oof = {"y": [], "model": [], "X": []}
    refs = {k: [] for k in reference_predictions(Xs[0]).keys()}
    for fold in folds:
        fold = set(fold)
        trX, trY = [], []
        for X, m in zip(Xs, Ms):
            sel = m & ~np.isin(polys, list(fold))
            trX.append(X[sel])
            trY.append(y[sel])
        mdl = M.fit_ensemble(pd.concat(trX, ignore_index=True), np.concatenate(trY),
                             seeds=model_seeds)
        X, m = Xs[0], Ms[0]
        sel = m & np.isin(polys, list(fold))
        pred = mdl.predict(X[sel])
        pred = np.where(np.isfinite(pred), pred, M.fallback_prediction(X[sel]))
        oof["y"].append(y[sel])
        oof["model"].append(pred)
        oof["X"].append(X[sel])
        for k, v in reference_predictions(X[sel]).items():
            refs[k].append(v)
    Y = np.concatenate(oof["y"])
    P = np.concatenate(oof["model"])
    out = [metrics.report(Y, np.concatenate(v), k) for k, v in refs.items()]
    out.append(metrics.report(Y, P, "модель"))
    return out, Y, P, pd.concat(oof["X"], ignore_index=True)


def run_season_holdout(train_df, grids, Xs, Ms, y, season, holdout=2024, model_seeds=(0, 1)):
    trX, trY = [], []
    for X, m in zip(Xs, Ms):
        sel = m & (season < holdout)
        trX.append(X[sel])
        trY.append(y[sel])
    mdl = M.fit_ensemble(pd.concat(trX, ignore_index=True), np.concatenate(trY),
                         seeds=model_seeds)
    X, m = Xs[0], Ms[0]
    sel = m & (season == holdout)
    pred = mdl.predict(X[sel])
    pred = np.where(np.isfinite(pred), pred, M.fallback_prediction(X[sel]))
    out = [metrics.report(y[sel], v, k) for k, v in reference_predictions(X[sel]).items()]
    out.append(metrics.report(y[sel], pred, "модель"))
    return out


def slice_report(y, pred, X):
    """Разрезы ошибки: где именно модель ошибается сильнее среднего.

    Разрезы выбраны по тому, что реально меняет задачу: расстояние между
    известными соседями, плотность контекста, фаза сезона и то, какой сенсор
    вероятнее всего дал скрытое наблюдение.
    """
    rows = []

    span = X["span"].to_numpy()
    for lo, hi in [(0, 3), (3, 6), (6, 12), (12, 30), (30, 1e9)]:
        m = (span > lo) & (span <= hi)
        if m.sum() >= 20:
            label = f"{lo}-{int(hi)}" if hi < 1e9 else f">{lo}"
            rows.append({"разрез": "расстояние между соседями, дн.", "значение": label,
                         "n": int(m.sum()), "rmse": round(metrics.rmse(y[m], pred[m]), 5)})

    ctx = X["n_obs_30"].to_numpy()
    for lo, hi in [(0, 4), (4, 8), (8, 14), (14, 1e9)]:
        m = (ctx > lo) & (ctx <= hi)
        if m.sum() >= 20:
            label = f"{lo}-{int(hi)}" if hi < 1e9 else f">{lo}"
            rows.append({"разрез": "наблюдений в окне ±30 дн.", "значение": label,
                         "n": int(m.sum()), "rmse": round(metrics.rmse(y[m], pred[m]), 5)})

    doy = X["doy"].to_numpy()
    for lo, hi, label in [(90, 150, "апрель-май"), (150, 220, "июнь-июль"),
                          (220, 275, "август-сентябрь"), (275, 310, "октябрь")]:
        m = (doy >= lo) & (doy < hi)
        if m.sum() >= 20:
            rows.append({"разрез": "фаза сезона", "значение": label,
                         "n": int(m.sum()), "rmse": round(metrics.rmse(y[m], pred[m]), 5)})

    probs = X[["p_src_s2", "p_src_landsat", "p_src_modis"]].to_numpy()
    likely = np.nanargmax(np.nan_to_num(probs, nan=-1.0), axis=1)
    for i, name in enumerate(["s2", "landsat", "modis"]):
        m = likely == i
        if m.sum() >= 20:
            rows.append({"разрез": "вероятный источник", "значение": name,
                         "n": int(m.sum()), "rmse": round(metrics.rmse(y[m], pred[m]), 5)})
    return rows


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", default=paths.TRAIN_CSV)
    ap.add_argument("--private", default=paths.PRIVATE_CSV)
    ap.add_argument("--out", default=str(paths.REPORTS_DIR / "experiments.csv"))
    ap.add_argument("--mask-seeds", type=int, default=8)
    args = ap.parse_args()

    t0 = time.time()
    tr = panel.load_train(args.train)
    pv = panel.load_private(args.private)
    grids = pipeline.fit_grids([tr, pv])
    tr = pipeline.sort_context(tr)
    y = tr["primary_ndvi"].to_numpy(dtype=float)
    polys = tr["anon_polygon_id"].to_numpy()
    season = tr["season"].to_numpy()

    seeds = list(range(11, 11 + args.mask_seeds))
    Xs, Ms = build_masked_sets(tr, grids, seeds)
    print(f"признаки построены: {Xs[0].shape[1]} колонок, {len(seeds)} масок, "
          f"{round(time.time() - t0, 1)} с")

    rows = []
    cv_rows, Y, P, Xoof = run_polygon_cv(tr, grids, Xs, Ms, y, polys)
    for r in cv_rows:
        r["режим"] = "новые полигоны (GroupKFold по полигону)"
        rows.append(r)
    for r in run_season_holdout(tr, grids, Xs, Ms, y, season):
        r["режим"] = "новый сезон 2024 (обучение до 2023)"
        rows.append(r)

    df = pd.DataFrame(rows)[["режим", "name", "n", "rmse", "gap_score"]]
    df.columns = ["режим", "метод", "n", "rmse", "gap_score"]
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    df.to_csv(args.out, index=False, encoding="utf-8", lineterminator="\n")
    print(df.to_string(index=False))

    slices = pd.DataFrame(slice_report(Y, P, Xoof))
    slice_path = Path(args.out).with_name("error_slices.csv")
    slices.to_csv(slice_path, index=False, encoding="utf-8", lineterminator="\n")
    print()
    print(slices.to_string(index=False))
    print(f"\nзаписано в {args.out} и {slice_path}, всего {round(time.time() - t0, 1)} с")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
