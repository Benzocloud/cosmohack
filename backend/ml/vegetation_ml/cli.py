from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import numpy as np
import pandas as pd

from . import __version__, anomalies, model as M, panel, paths, pipeline, schema as S


def cmd_predict(args) -> int:
    """Пакетный инференс: private_features.csv на входе, submission.csv на выходе."""
    sub, mdl = pipeline.predict_private(
        args.train, args.private, model_path=args.model, out_path=args.out,
        seeds=tuple(range(11, 11 + args.mask_seeds)),
        model_seeds=tuple(range(args.model_seeds)))
    print(f"строк в ответе: {len(sub)}")
    print(f"диапазон предсказаний: {sub.primary_ndvi_pred.min():.4f} .. "
          f"{sub.primary_ndvi_pred.max():.4f}")
    print(f"записано: {args.out}")
    return 0


def cmd_train(args) -> int:
    """Обучение модели и сохранение артефакта с манифестом."""
    tr = panel.load_train(args.train)
    pv = panel.load_private(args.private)
    grids = pipeline.fit_grids([tr, pv])
    mdl = pipeline.train_on_simulated(
        tr, grids, seeds=tuple(range(11, 11 + args.mask_seeds)),
        model_seeds=tuple(range(args.model_seeds)))
    mdl.meta["package_version"] = __version__
    mdl.save(args.model)
    print(f"модель сохранена: {args.model}")
    print(json.dumps(mdl.meta, ensure_ascii=False, indent=2)[:600])
    return 0


def cmd_validate(args) -> int:
    """Проверка готового submission.csv на точное совпадение ключей."""
    sub = pd.read_csv(args.submission)
    pv = panel.load_private(args.private)
    pipeline.validate_submission(sub, pv)
    print(f"ответ корректен: {len(sub)} строк, ключи совпадают с контрольными")
    return 0


def cmd_analyze(args) -> int:
    """Разбор одного полигона: восстановленный ряд и негативные периоды."""
    tr = panel.load_train(args.train)
    pv = panel.load_private(args.private)
    grids = pipeline.fit_grids([tr, pv])
    ctx = pipeline.sort_context(panel.concat_context(tr, pv))
    sel = ctx["anon_polygon_id"] == args.polygon
    if not sel.any():
        print(f"полигон {args.polygon} не найден", file=sys.stderr)
        return 2
    if args.season:
        sel &= ctx["season"] == args.season

    mask = np.zeros(len(ctx), dtype=bool)
    p, X, _, _ = pipeline.build_matrices(ctx, mask, grids)
    mdl = M.RestorationModel.load(args.model) if args.model and Path(args.model).exists() else None
    value = p[S.TARGET].to_numpy(dtype=float)
    filled = M.fallback_prediction(X) if mdl is None else mdl.predict(X)
    filled = np.where(np.isfinite(filled), filled, M.fallback_prediction(X))
    series = pd.DataFrame({
        "date": p["date"], "value": np.where(np.isfinite(value), value, filled),
        "is_observed": np.isfinite(value),
        "clim_mean": X["clim_mean"], "clim_std": X["clim_std"], "clim_n": X["clim_n"],
        "precip_prev30": X["precip_prev30"], "temp_prev30": X["temp_prev30"],
    })[sel.to_numpy()].reset_index(drop=True)

    result = anomalies.analyse_polygon(series)
    result["polygon"] = args.polygon
    result["model_version"] = __version__
    print(json.dumps({k: v for k, v in result.items() if k != "zscore"},
                     ensure_ascii=False, indent=2))
    if args.series_out:
        series.assign(zscore=result["zscore"]).to_csv(args.series_out, index=False,
                                                      encoding="utf-8")
        print(f"ряд записан: {args.series_out}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    ap = argparse.ArgumentParser(prog="vegetation-ml", description=
                                 "восстановление NDVI и поиск негативных периодов вегетации")
    ap.add_argument("--train", default=paths.TRAIN_CSV)
    ap.add_argument("--private", default=paths.PRIVATE_CSV)
    sub = ap.add_subparsers(dest="command", required=True)

    p = sub.add_parser("predict", help="построить submission.csv")
    p.add_argument("--out", default=paths.SUBMISSION_CSV)
    p.add_argument("--model", default=paths.MODEL_ARTIFACT)
    p.add_argument("--mask-seeds", type=int, default=8)
    p.add_argument("--model-seeds", type=int, default=3)
    p.set_defaults(func=cmd_predict)

    p = sub.add_parser("train", help="обучить и сохранить модель")
    p.add_argument("--model", default=paths.MODEL_ARTIFACT)
    p.add_argument("--mask-seeds", type=int, default=8)
    p.add_argument("--model-seeds", type=int, default=3)
    p.set_defaults(func=cmd_train)

    p = sub.add_parser("validate", help="проверить формат и ключи ответа")
    p.add_argument("--submission", default=paths.SUBMISSION_CSV)
    p.set_defaults(func=cmd_validate)

    p = sub.add_parser("analyze", help="разобрать один полигон")
    p.add_argument("--polygon", required=True)
    p.add_argument("--season", type=int)
    p.add_argument("--model", default=paths.MODEL_ARTIFACT)
    p.add_argument("--series-out")
    p.set_defaults(func=cmd_analyze)
    return ap


def main(argv=None) -> int:
    args = build_parser().parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
