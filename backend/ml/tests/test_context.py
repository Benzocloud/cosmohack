from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from vegetation_ml import panel, pipeline, schema as S


def _target_frame(frame, seasons, gap_rows=10):
    """Файл организаторов: часть сезонов и несколько контрольных строк."""
    out = frame[frame.season.isin(seasons)].copy().reset_index(drop=True)
    flag = np.zeros(len(out), dtype=bool)
    observed = np.where(out[S.TARGET].notna().to_numpy())[0]
    flag[observed[5:5 + gap_rows]] = True
    out[S.GAP_FLAG] = flag
    out.loc[flag, [c for c in S.MASKED_ON_GAP if c in out.columns]] = np.nan
    return out


def test_context_merges_disjoint_files(synthetic_frame):
    early = _target_frame(synthetic_frame, {2020, 2021})
    late = _target_frame(synthetic_frame, {2022})
    merged = panel.concat_context(early, late)
    assert len(merged) == len(early) + len(late)
    assert not merged.duplicated(["anon_polygon_id", "date"]).any()
    assert set(merged.season.unique()) == {2020, 2021, 2022}


def test_overlapping_files_are_rejected(synthetic_frame):
    """Совпадение ключа означает дубль или раскрытие скрытого значения."""
    a = _target_frame(synthetic_frame, {2020, 2021})
    b = _target_frame(synthetic_frame, {2021, 2022})
    with pytest.raises(ValueError, match="пересекаются по ключу"):
        panel.concat_context(a, b)


def test_missing_required_column_is_rejected(synthetic_frame):
    broken = _target_frame(synthetic_frame, {2020}).drop(columns=["era5_temp_c"])
    with pytest.raises(ValueError, match="нет колонок"):
        panel.concat_context(broken)


def test_empty_input_is_rejected():
    with pytest.raises(ValueError, match="ни одного источника"):
        panel.concat_context()


def test_schema_without_climatology_columns_works(synthetic_frame, tmp_path):
    """Новый test_features.csv не содержит колонок климатологии организаторов.

    Пайплайн считает сезонную норму сам, поэтому их отсутствие не должно ничего
    ломать: проверяем весь путь до матрицы признаков.
    """
    frame = _target_frame(synthetic_frame, {2020, 2021, 2022})
    frame = frame.drop(columns=["ndvi_climatology_mean", "ndvi_climatology_std"])

    path = tmp_path / "test_features.csv"
    frame.drop(columns=["epoch", "year_", "doy_", "season"]).to_csv(path, index=False)
    loaded = panel.load_private(path)
    assert int(loaded[S.GAP_FLAG].sum()) == int(frame[S.GAP_FLAG].sum())

    context = pipeline.sort_context(panel.concat_context(loaded))
    grids = pipeline.fit_grids([loaded])
    key = pd.MultiIndex.from_arrays([context.anon_polygon_id, context.date])
    gaps = loaded.loc[loaded[S.GAP_FLAG], ["anon_polygon_id", "date"]]
    mask = np.asarray(key.isin(pd.MultiIndex.from_arrays([gaps.anon_polygon_id, gaps.date])))
    assert mask.sum() == int(loaded[S.GAP_FLAG].sum())

    _, X, _, _ = pipeline.build_matrices(context, mask, grids)
    assert "clim_mean" in X.columns
    assert np.isfinite(X.loc[mask, "anchor"].to_numpy()).any()


def test_control_rows_stay_hidden_in_context(synthetic_frame):
    """Контрольные строки не должны становиться наблюдениями после склейки."""
    frame = _target_frame(synthetic_frame, {2020, 2021})
    merged = panel.concat_context(frame)
    hidden = frame[S.GAP_FLAG].to_numpy()
    keys = set(zip(frame.loc[hidden, "anon_polygon_id"], frame.loc[hidden, "date"]))
    rows = merged[[k in keys for k in zip(merged.anon_polygon_id, merged.date)]]
    assert len(rows) == int(hidden.sum())
    assert rows[S.TARGET].isna().all()
    assert rows[S.SPECTRAL_COLUMNS].isna().all().all()
