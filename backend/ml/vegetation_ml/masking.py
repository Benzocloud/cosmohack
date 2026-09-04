from __future__ import annotations

import numpy as np
import pandas as pd

from . import schema as S

RUN_LENGTHS = np.array([1, 2, 3, 4, 5])
RUN_WEIGHTS = np.array([2252, 340, 41, 8, 5], dtype=float)
RUN_WEIGHTS = RUN_WEIGHTS / RUN_WEIGHTS.sum()

MASK_FRACTION = 0.15

def simulate_gaps(df: pd.DataFrame, fraction: float = MASK_FRACTION,
                  seed: int = 0, seasons: set | None = None) -> np.ndarray:
    """Помечает наблюдения, которые будут скрыты и станут целями проверки.

    Профиль повторяет маску организаторов, измеренную по private_features:
    3112 скрытых наблюдений в 2646 сериях — 2252 длины 1, 340 длины 2, далее
    единицы длиной до 5; края рядов почти не затронуты. Совпадение профиля
    важно: на серии длины 1 известны обе стороны, на длинных появляется
    настоящая экстраполяция.

    seasons ограничивает маскирование сезонами и так воспроизводит режим
    «известный полигон, новый сезон»."""
    rng = np.random.default_rng(seed)
    mask = np.zeros(len(df), dtype=bool)
    observed = df[S.TARGET].notna().to_numpy()
    if seasons is not None:
        eligible_season = df["season"].isin(seasons).to_numpy()
    else:
        eligible_season = np.ones(len(df), dtype=bool)

    for _, g in df.groupby("anon_polygon_id", sort=False):
        idx = g.index.to_numpy()
        obs_idx = idx[observed[idx] & eligible_season[idx]]
        if len(obs_idx) < 10:
            continue
        target = int(round(fraction * len(obs_idx)))
        taken = 0
        attempts = 0
        limit = len(obs_idx) * 5
        while taken < target and attempts < limit:
            attempts += 1
            run = int(rng.choice(RUN_LENGTHS, p=RUN_WEIGHTS))
            start = int(rng.integers(1, max(2, len(obs_idx) - run)))
            if start + run >= len(obs_idx):
                continue
            block = obs_idx[start:start + run]
            if mask[block].any():
                continue

            if mask[obs_idx[start - 1]] or mask[obs_idx[start + run]]:
                continue
            mask[block] = True
            taken += run
    return mask
