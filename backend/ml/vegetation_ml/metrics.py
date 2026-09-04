from __future__ import annotations

import numpy as np

from . import schema as S

def rmse(y_true, y_pred) -> float:
    """RMSE по всем контрольным точкам вместе, а не среднее RMSE по полигонам:
    на неравномерных данных это разные числа."""
    y_true = np.asarray(y_true, dtype=float)
    y_pred = np.asarray(y_pred, dtype=float)
    ok = np.isfinite(y_true) & np.isfinite(y_pred)
    if not ok.any():
        return float("nan")
    return float(np.sqrt(((y_true[ok] - y_pred[ok]) ** 2).mean()))

def gap_score(rmse_value: float) -> float:
    """round(30 * max(0, 1 - RMSE / 0.10), 2) — формула из регламента."""
    if not np.isfinite(rmse_value):
        return 0.0
    return round(S.GAP_SCORE_MAX * max(0.0, 1.0 - rmse_value / S.GAP_SCORE_RMSE_LIMIT), 2)

def report(y_true, y_pred, name: str = "") -> dict:
    r = rmse(y_true, y_pred)
    return {"name": name, "n": int(np.isfinite(np.asarray(y_true, dtype=float)).sum()),
            "rmse": round(r, 5), "gap_score": gap_score(r)}
