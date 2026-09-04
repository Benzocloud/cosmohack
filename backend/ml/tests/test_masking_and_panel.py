from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from vegetation_ml import features, masking, panel, pipeline, schema as S


def test_source_priority_matches_declared_order(synthetic_frame):
    src = panel.observed_source(synthetic_frame)
    known = synthetic_frame[S.TARGET].notna()
    assert (src[~known] == -1).all()
    s2 = synthetic_frame["s2_ndvi"].notna()
    assert (src[s2 & known] == 0).all()
    only_ls = ~s2 & synthetic_frame["landsat_ndvi"].notna()
    assert (src[only_ls & known] == 1).all()


def test_grids_recover_overpass_residues(synthetic_frame):
    grids = panel.fit_overpass_grids([synthetic_frame])
    assert grids.residues[("AOI-1", "s2_ndvi")] == {2}
    assert grids.residues[("AOI-1", "landsat_ndvi")] == {0, 8}
    assert grids.residues[("AOI-1", "modis_ndvi")] == {3}


def test_panel_hides_every_masked_column(synthetic_frame):
    df = pipeline.sort_context(synthetic_frame)
    mask = masking.simulate_gaps(df, fraction=0.15, seed=1)
    p = panel.build_panel(df, mask)
    hidden = [c for c in S.MASKED_ON_GAP if c in p.columns]
    assert p.loc[p.is_target, hidden].isna().all().all()
    assert (p.loc[p.is_target, "src"] == -1).all()
    assert p.loc[~p.is_target, S.TARGET].notna().sum() > 0


def test_simulated_mask_follows_organiser_profile(synthetic_frame):
    df = pipeline.sort_context(synthetic_frame)
    mask = masking.simulate_gaps(df, fraction=0.15, seed=3)
    observed = df[S.TARGET].notna().to_numpy()
    assert mask.sum() > 0
    assert (mask & ~observed).sum() == 0
    share = mask.sum() / observed.sum()
    assert 0.10 < share < 0.20

    runs = []
    for _, g in df.assign(m=mask, o=observed).groupby("anon_polygon_id", sort=False):
        f = g.loc[g.o, "m"].to_numpy()
        i = 0
        while i < len(f):
            if f[i]:
                j = i
                while j < len(f) and f[j]:
                    j += 1
                runs.append(j - i)
                i = j
            else:
                i += 1
    assert max(runs) <= 5
    assert np.mean([r == 1 for r in runs]) > 0.6


def test_masked_rows_keep_both_neighbours(synthetic_frame):
    df = pipeline.sort_context(synthetic_frame)
    mask = masking.simulate_gaps(df, fraction=0.15, seed=5)
    observed = df[S.TARGET].notna().to_numpy()
    for _, g in df.assign(m=mask, o=observed).groupby("anon_polygon_id", sort=False):
        f = g.loc[g.o, "m"].to_numpy()
        assert not f[0] and not f[-1]
