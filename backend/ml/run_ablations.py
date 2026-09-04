"""Абляции групп признаков в одинаковых условиях.

Все варианты считаются на одном и том же разбиении (GroupKFold по полигону,
5 фолдов), на одних и тех же 8 масках и с одними параметрами бустинга.
Отличается ровно одна вещь — какая группа признаков убрана. Иначе строки
таблицы нельзя сравнивать между собой.

Результат пишется в reports/ablations.csv.
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

GROUPS = {
    "эффект даты": lambda c: c.startswith("date_effect"),
    "эффект даты по сенсорам и схожести": lambda c: c.startswith("date_effect") and c != "date_effect",
    "климатология": lambda c: c.startswith("clim_"),
    "погода ERA5": lambda c: c.startswith(("temp_", "precip_")),
    "сетки и смещения сенсоров": lambda c: c.startswith(("on_grid_", "p_src_", "offset_", "expected_offset")),
    "сглаженные кривые": lambda c: c.startswith("smooth_"),
    "остатки соседей": lambda c: c.startswith("resid_"),
}


def cv_rmse(Xs, Ms, y, polys, drop=(), n_folds=5, model_seeds=(0, 1)):
    """RMSE на отложенных полигонах при удалённом наборе колонок."""
    uniq = np.array(sorted(set(polys)))
    rng = np.random.default_rng(0)
    rng.shuffle(uniq)
    folds = np.array_split(uniq, n_folds)
    ys, ps = [], []
    for fold in folds:
        fold = set(fold)
        trX, trY = [], []
        for X, m in zip(Xs, Ms):
            sel = m & ~np.isin(polys, list(fold))
            trX.append(X.loc[sel, [c for c in X.columns if c not in drop]])
            trY.append(y[sel])
        mdl = M.fit_ensemble(pd.concat(trX, ignore_index=True), np.concatenate(trY),
                             seeds=model_seeds)
        X, m = Xs[0], Ms[0]
        sel = m & np.isin(polys, list(fold))
        Xe = X.loc[sel, [c for c in X.columns if c not in drop]]
        pred = mdl.predict(Xe)
        pred = np.where(np.isfinite(pred), pred, M.fallback_prediction(Xe))
        ys.append(y[sel])
        ps.append(pred)
    return metrics.rmse(np.concatenate(ys), np.concatenate(ps))


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", default=paths.TRAIN_CSV)
    ap.add_argument("--private", default=paths.PRIVATE_CSV)
    ap.add_argument("--out", default=str(paths.REPORTS_DIR / "ablations.csv"))
    ap.add_argument("--mask-seeds", type=int, default=8)
    args = ap.parse_args()

    t0 = time.time()
    tr = panel.load_train(args.train)
    pv = panel.load_private(args.private)
    grids = pipeline.fit_grids([tr, pv])
    tr = pipeline.sort_context(tr)
    y = tr["primary_ndvi"].to_numpy(dtype=float)
    polys = tr["anon_polygon_id"].to_numpy()

    Xs, Ms = [], []
    for s in range(11, 11 + args.mask_seeds):
        m = masking.simulate_gaps(tr, seed=s)
        _, X, _, _ = pipeline.build_matrices(tr, m, grids)
        Xs.append(X)
        Ms.append(m)

    full = cv_rmse(Xs, Ms, y, polys)
    rows = [{"вариант": "полная модель", "убрано колонок": 0,
             "rmse": round(full, 5), "gap_score": metrics.gap_score(full),
             "вклад группы": 0.0}]
    print(f"полная модель: {full:.5f}")

    for name, pred in GROUPS.items():
        drop = tuple(c for c in Xs[0].columns if pred(c))
        if not drop:
            continue
        r = cv_rmse(Xs, Ms, y, polys, drop=drop)
        rows.append({"вариант": f"без группы: {name}", "убрано колонок": len(drop),
                     "rmse": round(r, 5), "gap_score": metrics.gap_score(r),
                     "вклад группы": round(r - full, 5)})
        print(f"без «{name}»: {r:.5f}  (+{r - full:.5f})")

    df = pd.DataFrame(rows).sort_values("вклад группы", ascending=False)
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    df.to_csv(args.out, index=False, encoding="utf-8", lineterminator="\n")
    print(f"\nзаписано в {args.out}, всего {round(time.time() - t0, 1)} с")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
