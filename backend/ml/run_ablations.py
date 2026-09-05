"""Абляции групп признаков в одинаковых условиях.

Используется тот же протокол и тот же набор контрольных строк, что и в
run_experiments.py: контрольные строки скрыты всегда, обучение идёт по всем
файлам организаторов. Отличается ровно одна вещь — какая группа признаков
убрана. Иначе строки таблицы нельзя сравнивать между собой.

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

from run_experiments import MODEL_SEEDS, build_evaluation, load_frames, polygon_groups
from vegetation_ml import metrics, model as M, panel, paths, pipeline, schema as S

GROUPS = {
    "эффект даты": lambda c: c.startswith("date_effect"),
    "эффект даты по сенсорам и схожести": lambda c: c.startswith("date_effect") and c != "date_effect",
    "климатология": lambda c: c.startswith("clim_"),
    "погода ERA5": lambda c: c.startswith(("temp_", "precip_")),
    "сетки и смещения сенсоров": lambda c: c.startswith(("on_grid_", "p_src_", "offset_", "expected_offset")),
    "сглаженные кривые": lambda c: c.startswith("smooth_"),
    "остатки соседей": lambda c: c.startswith("resid_"),
    "соседние наблюдения": lambda c: c.startswith(("dt_left", "dt_right", "val_left", "val_right", "src_left", "src_right")),
}


def rmse_without(pool, X_eval, truth, drop=()) -> float:
    """RMSE на контрольных строках при удалённом наборе колонок."""
    keep = [c for c in pool["X"].columns if c not in drop]
    model = M.fit_ensemble(pool["X"][keep], pool["y"], seeds=MODEL_SEEDS)
    block = X_eval[keep]
    predicted = model.predict(block)
    predicted = np.where(np.isfinite(predicted), predicted, M.fallback_prediction(block))
    return metrics.rmse(truth, predicted)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", default=paths.TRAIN_CSV)
    ap.add_argument("--targets", nargs="*", default=[paths.TEST_CSV, paths.PRIVATE_CSV])
    ap.add_argument("--out", default=str(paths.REPORTS_DIR / "ablations.csv"))
    args = ap.parse_args()

    started = time.time()
    present = [p for p in args.targets if Path(p).exists()]
    train_df, targets = load_frames(args.train, present)
    frames = [train_df, *targets.values()]
    grids = pipeline.fit_grids(frames)
    ctx = pipeline.sort_context(panel.concat_context(*frames))
    y = ctx[S.TARGET].to_numpy(dtype=float)
    polys = ctx["anon_polygon_id"].to_numpy()

    _, X_eval, truth, _, pool = build_evaluation(ctx, grids, y, polys)
    print(f"контрольных строк {len(truth)}, обучающих примеров {len(pool['X'])}, "
          f"{round(time.time() - started, 1)} с")

    full = rmse_without(pool, X_eval, truth)
    rows = [{"вариант": "полная модель", "убрано колонок": 0,
             "rmse": round(full, 5), "gap_score": metrics.gap_score(full),
             "вклад группы": 0.0}]
    print(f"полная модель: {full:.5f}")

    for name, matches in GROUPS.items():
        drop = tuple(c for c in pool["X"].columns if matches(c))
        if not drop:
            continue
        value = rmse_without(pool, X_eval, truth, drop=drop)
        rows.append({"вариант": f"без группы: {name}", "убрано колонок": len(drop),
                     "rmse": round(value, 5), "gap_score": metrics.gap_score(value),
                     "вклад группы": round(value - full, 5)})
        print(f"без «{name}»: {value:.5f}  ({value - full:+.5f})")

    table = pd.DataFrame(rows).sort_values("вклад группы", ascending=False)
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    table.to_csv(args.out, index=False, encoding="utf-8", lineterminator="\n")
    print(f"\nзаписано в {args.out}, всего {round(time.time() - started, 1)} с")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
