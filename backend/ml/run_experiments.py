"""Локальная проверка методов восстановления NDVI.

Основной протокол воспроизводит реальную подачу. Отобранные контрольные строки
скрыты всегда: их значения не попадают ни в один обучающий пример и не
участвуют в построении признаков. Модель при этом видит остальные наблюдения
тех же полигонов — ровно как при настоящем инференсе, где открытые строки
файлов организаторов доступны, а контрольные нет.

Второй протокол отвечает на другой вопрос: что будет на полигоне, которого не
было в обучении вообще. Он строже реальной подачи и измеряет переносимость, а
не ожидаемый результат.

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

from vegetation_ml import masking, metrics, panel, paths, pipeline
from vegetation_ml import model as M
from vegetation_ml import schema as S

EVAL_SEED = 99
TRAIN_MASK_SEEDS = (11, 12, 13, 14, 15, 16)
MODEL_SEEDS = (0, 1, 2)


def load_frames(train_path, target_paths):
    train_df = panel.load_train(train_path)
    targets = {Path(p).stem: panel.load_private(p) for p in target_paths}
    return train_df, targets


def polygon_groups(train_df, targets) -> dict:
    """Непересекающиеся группы полигонов для разрезов результата.

    Файлы организаторов пересекаются по полигонам: private_features содержит и
    те, что есть в train, и те, что есть в test_features. Пересекающиеся группы
    в отчёте бесполезны — одна из них оказалась бы надмножеством остальных, —
    поэтому каждый полигон относится ровно к одной группе, а порядок разбора
    идёт от самого раннего источника к самому позднему.
    """
    groups: dict[str, set] = {}
    assigned: set = set()
    for label, frame in [("train", train_df), *targets.items()]:
        members = set(frame.anon_polygon_id.unique()) - assigned
        if members:
            groups[f"полигоны {label}"] = members
            assigned |= members
    return groups


def build_evaluation(ctx, grids, y, polys, seeds=TRAIN_MASK_SEEDS):
    """Готовит общий набор контрольных строк и обучающие примеры вокруг него.

    Контрольные строки выбираются один раз и скрываются всегда: их значения не
    попадают ни в один обучающий пример и не участвуют в построении признаков.
    Оба протокола ниже пользуются одним и тем же набором, поэтому их числа
    сравнимы между собой — различается только состав обучения.
    """
    evaluation = masking.simulate_gaps(ctx, seed=EVAL_SEED)
    hidden = ctx.copy()
    hidden.loc[evaluation, [c for c in S.MASKED_ON_GAP if c in hidden.columns]] = np.nan

    _, X_eval, _, _ = pipeline.build_matrices(ctx, evaluation, grids)

    parts_x, parts_y, parts_p = [], [], []
    for seed in seeds:
        mask = masking.simulate_gaps(hidden, seed=seed) & ~evaluation
        _, Xi, _, _ = pipeline.build_matrices(hidden, mask, grids)
        parts_x.append(Xi[mask])
        parts_y.append(y[mask])
        parts_p.append(polys[mask])
    pool = {
        "X": pd.concat(parts_x, ignore_index=True),
        "y": np.concatenate(parts_y),
        "poly": np.concatenate(parts_p),
    }
    return evaluation, X_eval[evaluation], y[evaluation], polys[evaluation], pool


def score_rows(name, truth, values, group_of, groups):
    rows = [{**metrics.report(truth, values, name), "набор": "все контрольные строки"}]
    for label, members in groups.items():
        selected = np.isin(group_of, list(members))
        if selected.sum() >= 50:
            rows.append({**metrics.report(truth[selected], values[selected], name),
                         "набор": label})
    return rows


def as_submitted(X_eval, truth, group_of, groups, pool):
    """Протокол реальной подачи: модель видит остальные наблюдения тех же полигонов."""
    model = M.fit_ensemble(pool["X"], pool["y"], seeds=MODEL_SEEDS)
    predicted = model.predict(X_eval)
    predicted = np.where(np.isfinite(predicted), predicted, M.fallback_prediction(X_eval))

    methods = {
        "baseline организаторов": (X_eval["val_left_1"].to_numpy()
                                   + X_eval["val_right_1"].to_numpy()) / 2,
        "линейная интерполяция": (X_eval["linear_corr"].to_numpy()
                                  + X_eval["expected_offset"].to_numpy()),
        "сглаживание + смещение (anchor)": X_eval["anchor"].to_numpy(),
        "модель": predicted,
    }
    rows = []
    for name, values in methods.items():
        rows.extend(score_rows(name, truth, values, group_of, groups))
    return rows, predicted


def unseen_polygon_transfer(X_eval, truth, group_of, groups, pool, n_folds=5):
    """Тот же набор контрольных строк, но полигон исключён из обучения целиком.

    Отличие от протокола подачи ровно одно — состав обучающих полигонов,
    поэтому разница между числами измеряет именно переносимость.
    """
    unique = np.array(sorted(set(group_of)))
    rng = np.random.default_rng(0)
    rng.shuffle(unique)
    folds = np.array_split(unique, n_folds)

    predicted = np.full(len(truth), np.nan)
    for fold in folds:
        fold = set(fold)
        selected = np.isin(pool["poly"], list(fold))
        model = M.fit_ensemble(pool["X"][~selected], pool["y"][~selected], seeds=(0, 1))
        target = np.isin(group_of, list(fold))
        if not target.any():
            continue
        block = X_eval[target]
        pred = model.predict(block)
        predicted[target] = np.where(np.isfinite(pred), pred, M.fallback_prediction(block))
    return score_rows("модель", truth, predicted, group_of, groups)


def slice_report(y, pred, X, group_of, groups):
    """Разрезы ошибки: где именно модель ошибается сильнее среднего."""
    rows = []

    def add(kind, label, selected):
        if selected.sum() >= 30:
            rows.append({"разрез": kind, "значение": label, "n": int(selected.sum()),
                         "rmse": round(metrics.rmse(y[selected], pred[selected]), 5)})

    span = X["span"].to_numpy()
    for lo, hi in [(0, 3), (3, 6), (6, 12), (12, 30), (30, 1e9)]:
        add("расстояние между соседями, дн.",
            f"{lo}-{int(hi)}" if hi < 1e9 else f">{lo}", (span > lo) & (span <= hi))

    context = X["n_obs_30"].to_numpy()
    for lo, hi in [(0, 4), (4, 8), (8, 14), (14, 1e9)]:
        add("наблюдений в окне ±30 дн.",
            f"{lo}-{int(hi)}" if hi < 1e9 else f">{lo}", (context > lo) & (context <= hi))

    doy = X["doy"].to_numpy()
    for lo, hi, label in [(90, 150, "апрель-май"), (150, 220, "июнь-июль"),
                          (220, 275, "август-сентябрь"), (275, 310, "октябрь")]:
        add("фаза сезона", label, (doy >= lo) & (doy < hi))

    probs = X[["p_src_s2", "p_src_landsat", "p_src_modis"]].to_numpy()
    likely = np.nanargmax(np.nan_to_num(probs, nan=-1.0), axis=1)
    for code, name in enumerate(["s2", "landsat", "modis"]):
        add("вероятный источник", name, likely == code)

    for label, members in groups.items():
        add("группа полигонов", label, np.isin(group_of, list(members)))
    return rows


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", default=paths.TRAIN_CSV)
    ap.add_argument("--targets", nargs="*",
                    default=[paths.TEST_CSV, paths.PRIVATE_CSV])
    ap.add_argument("--out", default=str(paths.REPORTS_DIR / "experiments.csv"))
    args = ap.parse_args()

    started = time.time()
    present = [p for p in args.targets if Path(p).exists()]
    for path in args.targets:
        if path not in present:
            print(f"файл пропущен, его нет: {path}", file=sys.stderr)
    train_df, targets = load_frames(args.train, present)

    frames = [train_df, *targets.values()]
    grids = pipeline.fit_grids(frames)
    ctx = pipeline.sort_context(panel.concat_context(*frames))
    y = ctx[S.TARGET].to_numpy(dtype=float)
    polys = ctx["anon_polygon_id"].to_numpy()
    groups = polygon_groups(train_df, targets)
    print(f"контекст: {len(ctx)} строк, {ctx.anon_polygon_id.nunique()} полигонов, "
          f"источников {len(frames)}")

    evaluation, X_eval, truth, group_of, pool = build_evaluation(ctx, grids, y, polys)
    print(f"контрольных строк протокола {int(evaluation.sum())}, "
          f"обучающих примеров {len(pool['X'])}, {round(time.time() - started, 1)} с")

    rows = []
    main_rows, predicted = as_submitted(X_eval, truth, group_of, groups, pool)
    for row in main_rows:
        row["протокол"] = "как при подаче"
        rows.append(row)
    print(f"основной протокол готов, {round(time.time() - started, 1)} с")

    for row in unseen_polygon_transfer(X_eval, truth, group_of, groups, pool):
        row["протокол"] = "полигон не встречался в обучении"
        rows.append(row)

    table = pd.DataFrame(rows)[["протокол", "набор", "name", "n", "rmse", "gap_score"]]
    table.columns = ["протокол", "набор", "метод", "n", "rmse", "gap_score"]
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    table.to_csv(args.out, index=False, encoding="utf-8", lineterminator="\n")
    print(table.to_string(index=False))

    slices = pd.DataFrame(slice_report(truth, predicted, X_eval, group_of, groups))
    slice_path = Path(args.out).with_name("error_slices.csv")
    slices.to_csv(slice_path, index=False, encoding="utf-8", lineterminator="\n")
    print()
    print(slices.to_string(index=False))
    print(f"\nзаписано в {args.out} и {slice_path}, всего {round(time.time() - started, 1)} с")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
