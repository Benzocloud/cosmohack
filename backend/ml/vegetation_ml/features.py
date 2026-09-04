"""Построение признаков для восстановления primary_ndvi.

Всё считается только по тому, что доступно на контрольной строке: дате,
полигоне, культуре и соседних строках того же полигона. Целевые строки в
панели уже обнулены, поэтому признак не может подсмотреть индекс или погоду
того же дня.

Группы признаков: соседние наблюдения, гауссово сглаживание ряда, сетки
пролётов и ожидаемое смещение сенсора, сезонная норма полигона, агрегаты
погоды и эффект даты — общий сдвиг наблюдений остальных полигонов в этот день.
"""

from __future__ import annotations

import numpy as np
import pandas as pd

from . import schema as S
from .panel import OverpassGrids

BANDWIDTHS = (4.0, 8.0, 16.0)

SMOOTH_RADIUS = 45

N_NEIGHBOURS = 3

CLIM_WINDOW = 10

OFFSET_SHRINK = 20.0

def _gauss_at(query_e, known_e, known_v, bandwidths, self_pos=None):
    """Гауссовы взвешенные средние в точках query_e по известным наблюдениям.

    self_pos исключает саму строку из её же оценки. Без этого сглаженная
    кривая притягивается к строке и оценка смещения сенсора занижается."""
    n_q, n_b = len(query_e), len(bandwidths)
    out = np.full((n_q, n_b), np.nan)
    cnt = np.zeros(n_q, dtype=np.int32)
    if len(known_e) == 0:
        return out, cnt
    lo = np.searchsorted(known_e, query_e - SMOOTH_RADIUS, side="left")
    hi = np.searchsorted(known_e, query_e + SMOOTH_RADIUS, side="right")
    for j in range(n_q):
        a, b = lo[j], hi[j]
        if b <= a:
            continue
        d = known_e[a:b] - query_e[j]
        v = known_v[a:b]
        if self_pos is not None and self_pos[j] >= 0:
            k = self_pos[j] - a
            if 0 <= k < len(d):
                keep = np.ones(len(d), bool)
                keep[k] = False
                d, v = d[keep], v[keep]
        if len(d) == 0:
            continue
        cnt[j] = len(d)
        for bi, h in enumerate(bandwidths):
            w = np.exp(-0.5 * (d / h) ** 2)
            sw = w.sum()
            if sw > 1e-12:
                out[j, bi] = float((w * v).sum() / sw)
    return out, cnt

def _neighbour_block(query_e, known_e, known_v, known_s, k):
    """Ближайшие k известных наблюдений слева и справа от каждой точки."""
    n = len(query_e)
    dl = np.full((n, k), np.nan); vl = np.full((n, k), np.nan); sl = np.full((n, k), -1.0)
    dr = np.full((n, k), np.nan); vr = np.full((n, k), np.nan); sr = np.full((n, k), -1.0)
    if len(known_e) == 0:
        return dl, vl, sl, dr, vr, sr
    pos = np.searchsorted(known_e, query_e, side="left")
    for j in range(n):
        p = pos[j]
        for i in range(k):
            q = p - 1 - i
            if q < 0:
                break
            dl[j, i] = query_e[j] - known_e[q]; vl[j, i] = known_v[q]; sl[j, i] = known_s[q]

        q = p
        while q < len(known_e) and known_e[q] <= query_e[j]:
            q += 1
        for i in range(k):
            if q + i >= len(known_e):
                break
            dr[j, i] = known_e[q + i] - query_e[j]; vr[j, i] = known_v[q + i]; sr[j, i] = known_s[q + i]
    return dl, vl, sl, dr, vr, sr

def estimate_sensor_offsets(panel: pd.DataFrame) -> tuple[dict, np.ndarray]:
    """Оценивает калибровочное смещение и шум каждого сенсора.

    primary_ndvi собран из трёх сенсоров с разной калибровкой: на train средняя
    разница s2 − landsat равна −0.037, s2 − modis −0.082. Именно она даёт
    большую часть скачков ряда, а не изменение вегетации.

    Смещение считается как средний остаток относительно сглаженной кривой,
    построенной без самой строки. Оценка ведётся по всем строкам, где сенсор
    доступен, а не только по тем, где он оказался основным. Редкие сочетания
    полигон-сенсор подтягиваются к общему смещению сенсора.

    Возвращает словарь смещений и разброс остатка по каждому сенсору: он нужен
    как вес при слиянии нескольких сенсоров одного дня.
    """
    known = panel[S.TARGET].notna().to_numpy()
    smooth = np.full(len(panel), np.nan)
    for _, g in panel.groupby("anon_polygon_id", sort=False):
        idx = g.index.to_numpy()
        m = known[idx]
        if m.sum() < 5:
            continue
        ke = g["epoch"].to_numpy()[m]
        kv = g[S.TARGET].to_numpy(dtype=float)[m]
        sm, _ = _gauss_at(ke, ke, kv, (10.0,), self_pos=np.arange(len(ke)))
        smooth[idx[m]] = sm[:, 0]

    polys = panel["anon_polygon_id"].to_numpy()
    parts = []
    for si, col in enumerate(S.SENSOR_PRIORITY):
        v = panel[col].to_numpy(dtype=float)
        r = v - smooth
        ok = np.isfinite(r)
        parts.append(pd.DataFrame({"poly": polys[ok], "src": si, "r": r[ok]}))
    df = pd.concat(parts, ignore_index=True)

    glob = df.groupby("src").r.mean()
    noise = df.groupby("src").r.std()
    per = df.groupby(["poly", "src"]).r.agg(["mean", "size"])
    offsets: dict = {}
    for (poly, src), row in per.iterrows():
        g0 = float(glob.get(src, 0.0))
        w = row["size"] / (row["size"] + OFFSET_SHRINK)
        offsets[(poly, int(src))] = float(w * row["mean"] + (1 - w) * g0)
    for src, g0 in glob.items():
        offsets[("__global__", int(src))] = float(g0)
    sigma = np.array([float(noise.get(s, 0.06)) for s in range(3)])
    return offsets, sigma

def fit_source_probabilities(panel: pd.DataFrame, grids: OverpassGrids) -> pd.DataFrame:
    """P(источник | сочетание сеток пролётов) по известным строкам.

    На контрольной строке сенсор неизвестен, но известно, витки каких
    спутников проходят в этот день над этим полигоном. Этого хватает для
    оценки ожидаемого смещения: сочетание «только modis» в 93% случаев
    действительно даёт modis, «только s2» — в 98% s2."""
    rows = []
    for poly, g in panel.groupby("anon_polygon_id", sort=False):
        f = grids.flags(poly, g["epoch"].to_numpy())
        pat = (f["s2_ndvi"].astype(int) * 4 + f["landsat_ndvi"].astype(int) * 2
               + f["modis_ndvi"].astype(int))
        rows.append(pd.DataFrame({"pat": pat, "src": g["src"].to_numpy()}))
    d = pd.concat(rows, ignore_index=True)
    d = d[d.src >= 0]
    tab = pd.crosstab(d.pat, d.src, normalize="index")
    for s in range(3):
        if s not in tab.columns:
            tab[s] = 0.0
    tab = tab[[0, 1, 2]]

    prior = d.src.value_counts(normalize=True).reindex([0, 1, 2]).fillna(0.0).to_numpy()
    for pat in range(8):
        if pat not in tab.index:
            tab.loc[pat] = prior
    return tab.sort_index()

def fuse_sensors(panel: pd.DataFrame, offsets: dict, sigma: np.ndarray) -> np.ndarray:
    """Сливает доступные в один день сенсоры в одно значение.

    Примерно на 13% строк снимок дали два сенсора сразу. После снятия
    калибровочного смещения это два независимых измерения одной величины, и
    их среднее с весом по обратной дисперсии шумит заметно меньше, чем любое
    из них по отдельности.
    """
    polys = panel["anon_polygon_id"].to_numpy()
    num = np.zeros(len(panel))
    den = np.zeros(len(panel))
    for si, col in enumerate(S.SENSOR_PRIORITY):
        v = panel[col].to_numpy(dtype=float)
        ok = np.isfinite(v)
        if not ok.any():
            continue
        off = np.array([offsets.get((p, si), offsets.get(("__global__", si), 0.0))
                        for p in polys[ok]])
        w = 1.0 / max(sigma[si] ** 2, 1e-6)
        num[ok] += w * (v[ok] - off)
        den[ok] += w
    return np.where(den > 0, num / np.maximum(den, 1e-12), np.nan)


def build_features(panel: pd.DataFrame, grids: OverpassGrids,
                   offsets: dict, src_probs: pd.DataFrame,
                   sigma: np.ndarray | None = None) -> pd.DataFrame:
    """Возвращает матрицу признаков по всем строкам панели."""
    n = len(panel)
    e_all = panel["epoch"].to_numpy()
    v_all = panel[S.TARGET].to_numpy(dtype=float)
    s_all = panel["src"].to_numpy()
    polys = panel["anon_polygon_id"].to_numpy()
    known = np.isfinite(v_all)
    if sigma is None:
        sigma = np.full(3, 0.06)

    v_corr = fuse_sensors(panel, offsets, sigma)
    v_corr = np.where(known, v_corr, np.nan)

    nb = len(BANDWIDTHS)
    raw_sm = np.full((n, nb), np.nan)
    cor_sm = np.full((n, nb), np.nan)
    n_ctx = np.zeros(n)
    dlB = np.full((n, N_NEIGHBOURS), np.nan); vlB = np.full((n, N_NEIGHBOURS), np.nan)
    slB = np.full((n, N_NEIGHBOURS), -1.0)
    drB = np.full((n, N_NEIGHBOURS), np.nan); vrB = np.full((n, N_NEIGHBOURS), np.nan)
    srB = np.full((n, N_NEIGHBOURS), -1.0)
    rlB = np.full((n, N_NEIGHBOURS), np.nan); rrB = np.full((n, N_NEIGHBOURS), np.nan)
    clim_m = np.full(n, np.nan); clim_s = np.full(n, np.nan); clim_n = np.zeros(n)
    loo_resid = np.full(n, np.nan)
    g_s2 = np.zeros(n); g_ls = np.zeros(n); g_md = np.zeros(n)
    p_src = np.full((n, 3), np.nan)
    exp_off = np.zeros(n)
    n7 = np.zeros(n); n15 = np.zeros(n); n30 = np.zeros(n)
    off_by_src = np.zeros((n, 3))

    probs_np = src_probs.to_numpy()
    for poly, g in panel.groupby("anon_polygon_id", sort=False):
        idx = g.index.to_numpy()
        e = e_all[idx]
        m = known[idx]
        ke = e[m]
        kv = v_all[idx][m]
        kvc = v_corr[idx][m]
        ks = s_all[idx][m].astype(float)
        self_pos = np.full(len(idx), -1)
        self_pos[m] = np.arange(len(ke))

        raw_sm[idx], cnt = _gauss_at(e, ke, kv, BANDWIDTHS, self_pos=self_pos)
        cor_sm[idx], _ = _gauss_at(e, ke, kvc, BANDWIDTHS, self_pos=self_pos)
        n_ctx[idx] = cnt
        loo_resid[idx[m]] = kvc - cor_sm[idx][m, 1]

        a, b, c, d_, f_, h_ = _neighbour_block(e, ke, kvc, ks, N_NEIGHBOURS)
        dlB[idx], vlB[idx], slB[idx] = a, b, c
        drB[idx], vrB[idx], srB[idx] = d_, f_, h_

        kr = loo_resid[idx[m]]
        _, rl, _, _, rr, _ = _neighbour_block(e, ke, kr, ks, N_NEIGHBOURS)
        rlB[idx], rrB[idx] = rl, rr

        if len(ke):
            for arr, rad in ((n7, 7), (n15, 15), (n30, 30)):
                lo = np.searchsorted(ke, e - rad, "left")
                hi = np.searchsorted(ke, e + rad, "right")
                arr[idx] = hi - lo

        doy = panel["doy_"].to_numpy()[idx]
        season = panel["season"].to_numpy()[idx]
        kdoy, kseason = doy[m], season[m]
        order = np.argsort(kdoy, kind="stable")
        sd, sv, ss = kdoy[order], kvc[order], kseason[order]
        lo = np.searchsorted(sd, doy - CLIM_WINDOW, "left")
        hi = np.searchsorted(sd, doy + CLIM_WINDOW, "right")
        for j in range(len(idx)):
            vv = sv[lo[j]:hi[j]][ss[lo[j]:hi[j]] != season[j]]
            if len(vv) >= 3:
                clim_m[idx[j]] = vv.mean()
                clim_s[idx[j]] = vv.std()
                clim_n[idx[j]] = len(vv)

        fl = grids.flags(poly, e)
        g_s2[idx] = fl["s2_ndvi"]; g_ls[idx] = fl["landsat_ndvi"]; g_md[idx] = fl["modis_ndvi"]
        pat = (fl["s2_ndvi"].astype(int) * 4 + fl["landsat_ndvi"].astype(int) * 2
               + fl["modis_ndvi"].astype(int))
        pr = probs_np[pat]
        p_src[idx] = pr
        o = np.array([offsets.get((poly, s), offsets.get(("__global__", s), 0.0)) for s in range(3)])
        off_by_src[idx] = o
        exp_off[idx] = pr @ o

    date_effect, date_effect_n = _leave_one_polygon_date_effect(e_all, loo_resid, polys)
    de_by_src = np.full((n, 3), np.nan)
    for s in range(3):
        r_s = np.where(s_all == s, loo_resid, np.nan)
        de_by_src[:, s], _ = _leave_one_polygon_date_effect(e_all, r_s, polys)
    de_expected = np.nansum(np.nan_to_num(de_by_src) * p_src, axis=1)
    de_expected = np.where(np.isfinite(de_by_src).any(axis=1), de_expected, np.nan)
    de_knn8, de_top1 = _correlated_date_effect(e_all, loo_resid, polys, k=8)
    de_knn3, _ = _correlated_date_effect(e_all, loo_resid, polys, k=3)

    w = _weather_windows(panel)

    F = pd.DataFrame(index=panel.index)
    for bi, h in enumerate(BANDWIDTHS):
        F[f"smooth_raw_h{int(h)}"] = raw_sm[:, bi]
        F[f"smooth_corr_h{int(h)}"] = cor_sm[:, bi]
    for i in range(N_NEIGHBOURS):
        F[f"dt_left_{i + 1}"] = dlB[:, i]
        F[f"val_left_{i + 1}"] = vlB[:, i]
        F[f"src_left_{i + 1}"] = slB[:, i]
        F[f"dt_right_{i + 1}"] = drB[:, i]
        F[f"val_right_{i + 1}"] = vrB[:, i]
        F[f"src_right_{i + 1}"] = srB[:, i]

    dl1, dr1 = dlB[:, 0], drB[:, 0]
    vl1, vr1 = vlB[:, 0], vrB[:, 0]
    both = np.isfinite(vl1) & np.isfinite(vr1)
    F["baseline_corr"] = np.where(both, (vl1 + vr1) / 2, np.where(np.isfinite(vl1), vl1, vr1))
    span = dl1 + dr1
    F["linear_corr"] = np.where(both & (span > 0),
                                vl1 + (vr1 - vl1) * dl1 / np.where(span > 0, span, 1.0),
                                F["baseline_corr"])
    F["span"] = span
    F["n_ctx"] = n_ctx
    F["n_obs_7"] = n7
    F["n_obs_15"] = n15
    F["n_obs_30"] = n30
    F["clim_mean"] = clim_m
    F["clim_std"] = clim_s
    F["clim_n"] = clim_n
    F["clim_dev"] = cor_sm[:, 1] - clim_m
    F["on_grid_s2"] = g_s2
    F["on_grid_landsat"] = g_ls
    F["on_grid_modis"] = g_md
    F["p_src_s2"] = p_src[:, 0]
    F["p_src_landsat"] = p_src[:, 1]
    F["p_src_modis"] = p_src[:, 2]
    F["offset_s2"] = off_by_src[:, 0]
    F["offset_landsat"] = off_by_src[:, 1]
    F["offset_modis"] = off_by_src[:, 2]
    F["expected_offset"] = exp_off
    F["date_effect"] = date_effect
    F["date_effect_n"] = date_effect_n
    F["date_effect_s2"] = de_by_src[:, 0]
    F["date_effect_landsat"] = de_by_src[:, 1]
    F["date_effect_modis"] = de_by_src[:, 2]
    F["date_effect_expected"] = de_expected
    F["date_effect_knn8"] = de_knn8
    F["date_effect_knn3"] = de_knn3
    F["date_effect_top1"] = de_top1
    for i in range(N_NEIGHBOURS):
        F[f"resid_left_{i + 1}"] = rlB[:, i]
        F[f"resid_right_{i + 1}"] = rrB[:, i]
    F["doy"] = panel["doy_"].to_numpy()
    F["year"] = panel["year_"].to_numpy()
    for k in (1, 2):
        F[f"sin{k}"] = np.sin(2 * np.pi * k * F["doy"] / 365.25)
        F[f"cos{k}"] = np.cos(2 * np.pi * k * F["doy"] / 365.25)
    F["crop_type"] = pd.Categorical(panel["crop_type"]).codes
    for c, v in w.items():
        F[c] = v

    anchor = F["smooth_corr_h8"] + F["expected_offset"]
    anchor = anchor.fillna(F["baseline_corr"] + F["expected_offset"])
    F["anchor"] = anchor
    return F

def _correlated_date_effect(epoch, resid, polys, k=8, min_periods=60):
    """Эффект даты, взвешенный по схожести полигонов.

    Полигоны анонимны, но их остатки выдают географическую близость: средняя
    корреляция между парами 0.15, а у ближайших пар доходит до 0.74. Поэтому
    вместо среднего по всем полигонам берём взвешенное среднее по k наиболее
    похожим. Свой полигон из выборки исключён.
    """
    n = len(epoch)
    d = pd.DataFrame({"e": epoch, "r": resid, "p": polys}).dropna()
    if d["p"].nunique() < 3:
        return np.full(n, np.nan), np.full(n, np.nan)
    W = d.pivot_table(index="e", columns="p", values="r")
    C = np.array(W.corr(min_periods=min_periods).to_numpy(), dtype=float, copy=True)
    np.fill_diagonal(C, np.nan)
    R = W.to_numpy()
    M = np.isfinite(R)
    Rz = np.where(M, R, 0.0)

    n_poly = R.shape[1]
    eff = np.full(R.shape, np.nan)
    top = np.full(R.shape, np.nan)
    for j in range(n_poly):
        c = C[:, j]
        ok = np.isfinite(c) & (c > 0)
        if not ok.any():
            continue
        idx = np.argsort(np.where(ok, c, -np.inf))[::-1][:k]
        idx = idx[np.isfinite(c[idx]) & (c[idx] > 0)]
        if len(idx) == 0:
            continue
        w = np.zeros(n_poly)
        w[idx] = c[idx]
        num = Rz @ w
        den = M.astype(float) @ w
        eff[:, j] = np.where(den > 0, num / den, np.nan)
        best = idx[0]
        top[:, j] = np.where(M[:, best], R[:, best], np.nan)

    date_pos = W.index.get_indexer(epoch)
    poly_pos = W.columns.get_indexer(polys)
    valid = (date_pos >= 0) & (poly_pos >= 0)
    out_eff = np.full(n, np.nan)
    out_top = np.full(n, np.nan)
    out_eff[valid] = eff[date_pos[valid], poly_pos[valid]]
    out_top[valid] = top[date_pos[valid], poly_pos[valid]]
    return out_eff, out_top


def _leave_one_polygon_date_effect(epoch, resid, polys):
    """Средний остаток остальных полигонов в тот же день.

    Ловит общее для съёмки состояние атмосферы: в один и тот же день все
    полигоны отклоняются от собственных сглаженных кривых согласованно.
    Свой полигон исключается, поэтому признак не подсматривает цель.
    """
    d = pd.DataFrame({"e": epoch, "r": resid, "p": polys}).dropna()
    if not len(d):
        return np.full(len(epoch), np.nan), np.zeros(len(epoch))
    agg = d.groupby("e").r.agg(["sum", "size"])
    own = d.groupby(["p", "e"]).r.agg(["sum", "size"])
    tot_s = np.nan_to_num(agg["sum"].reindex(epoch).to_numpy())
    tot_n = np.nan_to_num(agg["size"].reindex(epoch).to_numpy())
    key = pd.MultiIndex.from_arrays([polys, epoch])
    own_s = np.nan_to_num(own["sum"].reindex(key).to_numpy())
    own_n = np.nan_to_num(own["size"].reindex(key).to_numpy())
    other_n = tot_n - own_n
    eff = np.where(other_n > 0, (tot_s - own_s) / np.maximum(other_n, 1), np.nan)
    return eff, other_n


def _weather_windows(panel: pd.DataFrame) -> dict:
    """Температура и осадки по окнам до и вокруг даты.

    Окна не включают текущий день: на контрольной строке погода скрыта."""
    n = len(panel)
    names = ("temp_prev10", "temp_prev30", "temp_around7",
             "precip_prev10", "precip_prev30")
    out = {k: np.full(n, np.nan) for k in names}
    for _, g in panel.groupby("anon_polygon_id", sort=False):
        idx = g.index.to_numpy()
        e = g["epoch"].to_numpy()
        t = g["era5_temp_c"].to_numpy(dtype=float)
        p = g["era5_precip_mm"].to_numpy(dtype=float)
        ok_t = np.isfinite(t)
        ok_p = np.isfinite(p)
        et, vt = e[ok_t], t[ok_t]
        ep_, vp = e[ok_p], p[ok_p]
        ct = np.concatenate([[0.0], np.cumsum(vt)])
        cp = np.concatenate([[0.0], np.cumsum(vp)])

        def mean_t(lo_d, hi_d):
            a = np.searchsorted(et, e + lo_d, "left")
            b = np.searchsorted(et, e + hi_d, "right")
            cnt = b - a
            return np.where(cnt > 0, (ct[b] - ct[a]) / np.maximum(cnt, 1), np.nan)

        def sum_p(lo_d, hi_d):
            a = np.searchsorted(ep_, e + lo_d, "left")
            b = np.searchsorted(ep_, e + hi_d, "right")
            return np.where(b > a, cp[b] - cp[a], np.nan)

        out["temp_prev10"][idx] = mean_t(-10, -1)
        out["temp_prev30"][idx] = mean_t(-30, -1)
        out["temp_around7"][idx] = mean_t(-7, 7)
        out["precip_prev10"][idx] = sum_p(-10, -1)
        out["precip_prev30"][idx] = sum_p(-30, -1)
    return out
