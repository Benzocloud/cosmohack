from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from vegetation_ml import features, masking, metrics, model as M, panel, pipeline, schema as S


@pytest.fixture
def prepared(synthetic_frame):
    df = pipeline.sort_context(synthetic_frame)
    grids = pipeline.fit_grids([df])
    mask = masking.simulate_gaps(df, fraction=0.15, seed=2)
    p, X, offsets, probs = pipeline.build_matrices(df, mask, grids)
    return df, grids, mask, p, X, offsets, probs


def test_features_do_not_leak_target(prepared):
    """Признаки скрытой строки не должны меняться от её собственного значения."""
    df, grids, mask, _, X, _, _ = prepared
    spoiled = df.copy()
    idx = np.where(mask)[0]
    spoiled.loc[spoiled.index[idx], S.TARGET] += 5.0
    for col in ("s2_ndvi", "landsat_ndvi", "modis_ndvi"):
        spoiled.loc[spoiled.index[idx], col] += 5.0
    _, X2, _, _ = pipeline.build_matrices(spoiled, mask, grids)
    a = X.loc[mask].to_numpy(dtype=float)
    b = X2.loc[mask].to_numpy(dtype=float)
    assert np.allclose(np.nan_to_num(a), np.nan_to_num(b), atol=1e-9)


def test_sensor_offsets_recover_injected_bias(prepared):
    """В синтетике landsat смещён на +0.04, modis на +0.09 относительно s2."""
    _, _, _, p, _, offsets, _ = prepared
    d_ls = offsets[("AOI-1", 1)] - offsets[("AOI-1", 0)]
    d_md = offsets[("AOI-1", 2)] - offsets[("AOI-1", 0)]
    assert 0.02 < d_ls < 0.06
    assert 0.06 < d_md < 0.12


def test_source_probabilities_are_normalised(prepared):
    _, _, _, _, _, _, probs = prepared
    assert list(probs.index) == list(range(8))
    assert np.allclose(probs.sum(axis=1).to_numpy(), 1.0)


def test_anchor_beats_naive_baseline(prepared):
    df, _, mask, _, X, _, _ = prepared
    y = df[S.TARGET].to_numpy(dtype=float)
    naive = (X["val_left_1"] + X["val_right_1"]).to_numpy() / 2
    assert metrics.rmse(y[mask], X["anchor"].to_numpy()[mask]) <= metrics.rmse(y[mask], naive[mask])


def test_model_trains_and_predicts_in_range(prepared):
    df, _, mask, _, X, _, _ = prepared
    y = df[S.TARGET].to_numpy(dtype=float)
    mdl = M.fit(X[mask], y[mask], params={"max_iter": 60})
    pred = mdl.predict(X[mask])
    assert pred.shape == (mask.sum(),)
    assert np.isfinite(pred).all()
    assert (pred >= M.CLIP_LOW).all() and (pred <= M.CLIP_HIGH).all()


def test_fallback_never_returns_nan_when_context_exists(prepared):
    _, _, mask, _, X, _, _ = prepared
    fb = M.fallback_prediction(X[mask])
    assert np.isfinite(fb).mean() > 0.99


def test_gap_score_formula():
    assert metrics.gap_score(0.0) == 30.0
    assert metrics.gap_score(0.10) == 0.0
    assert metrics.gap_score(0.15) == 0.0
    assert metrics.gap_score(0.05) == 15.0
    assert metrics.gap_score(0.0574) == round(30 * (1 - 0.574), 2)
