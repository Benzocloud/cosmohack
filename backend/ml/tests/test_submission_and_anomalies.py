from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from vegetation_ml import anomalies, pipeline
from vegetation_ml import schema as S


@pytest.fixture
def private_like(synthetic_frame):
    df = pipeline.sort_context(synthetic_frame).copy()
    observed = df[S.TARGET].notna().to_numpy()
    flag = np.zeros(len(df), dtype=bool)
    flag[np.where(observed)[0][5:25]] = True
    df[S.GAP_FLAG] = flag
    hide = [c for c in S.MASKED_ON_GAP if c in df.columns]
    df.loc[flag, hide] = np.nan
    return df


def _good_submission(private_like):
    g = private_like.loc[private_like[S.GAP_FLAG], ["anon_polygon_id", "date"]].copy()
    g["date"] = pd.to_datetime(g["date"]).dt.strftime("%Y-%m-%d")
    g["primary_ndvi_pred"] = 0.4
    return g.reset_index(drop=True)


def test_valid_submission_passes(private_like):
    pipeline.validate_submission(_good_submission(private_like), private_like)


def test_missing_key_is_rejected(private_like):
    sub = _good_submission(private_like).iloc[1:].reset_index(drop=True)
    with pytest.raises(ValueError, match="ключей не совпадает"):
        pipeline.validate_submission(sub, private_like)


def test_extra_key_is_rejected(private_like):
    sub = _good_submission(private_like)
    extra = sub.iloc[[0]].copy()
    extra["date"] = "1999-01-01"
    with pytest.raises(ValueError, match="ключей не совпадает"):
        pipeline.validate_submission(pd.concat([sub, extra], ignore_index=True), private_like)


def test_duplicate_key_is_rejected(private_like):
    sub = _good_submission(private_like)
    with pytest.raises(ValueError, match="дублирующиеся"):
        pipeline.validate_submission(pd.concat([sub, sub.iloc[[0]]], ignore_index=True),
                                     private_like)


def test_nan_prediction_is_rejected(private_like):
    sub = _good_submission(private_like)
    sub.loc[0, "primary_ndvi_pred"] = np.nan
    with pytest.raises(ValueError, match="NaN"):
        pipeline.validate_submission(sub, private_like)


def test_wrong_header_is_rejected(private_like):
    sub = _good_submission(private_like).rename(columns={"primary_ndvi_pred": "pred"})
    with pytest.raises(ValueError, match="заголовок"):
        pipeline.validate_submission(sub, private_like)


def _series(values, observed=None, clim_mean=0.6, clim_std=0.05, clim_n=10):
    n = len(values)
    return pd.DataFrame({
        "date": pd.date_range("2022-05-01", periods=n, freq="D"),
        "value": values,
        "is_observed": np.ones(n, bool) if observed is None else observed,
        "clim_mean": np.full(n, clim_mean), "clim_std": np.full(n, clim_std),
        "clim_n": np.full(n, clim_n),
        "precip_prev30": np.full(n, 30.0), "temp_prev30": np.full(n, 20.0),
    })


def test_normal_series_has_no_periods():
    r = anomalies.analyse_polygon(_series(np.full(20, 0.6)))
    assert r["status"] == "normal"
    assert r["periods"] == []


def test_deep_drop_is_confirmed_and_critical():
    v = np.full(20, 0.6)
    v[5:14] = 0.40
    r = anomalies.analyse_polygon(_series(v))
    assert r["status"] == "confirmed"
    assert r["periods"][0]["severity"] == "critical"
    assert r["periods"][0]["days"] >= anomalies.MIN_PERIOD_DAYS


def test_imputed_only_period_is_candidate_not_confirmed():
    v = np.full(20, 0.6)
    v[5:14] = 0.40
    obs = np.ones(20, bool)
    obs[5:14] = False
    r = anomalies.analyse_polygon(_series(v, observed=obs))
    assert r["periods"][0]["status"] == "candidate"
    assert any("восстановлена моделью" in x for x in r["periods"][0]["limitations"])


def test_short_history_yields_insufficient_data():
    v = np.full(20, 0.6)
    v[5:14] = 0.40
    r = anomalies.analyse_polygon(_series(v, clim_n=1))
    assert r["periods"][0]["status"] == "insufficient_data"


def test_single_noisy_day_is_not_a_period():
    v = np.full(20, 0.6)
    v[7] = 0.30
    r = anomalies.analyse_polygon(_series(v))
    assert r["periods"] == []


def test_dry_window_adds_unconfirmed_hypothesis():
    v = np.full(20, 0.6)
    v[5:14] = 0.40
    s = _series(v)
    s.loc[5:13, "precip_prev30"] = 5.0
    r = anomalies.analyse_polygon(s)
    hyp = r["periods"][0]["hypotheses"]
    assert hyp and hyp[0]["confidence"] == "не подтверждена"
