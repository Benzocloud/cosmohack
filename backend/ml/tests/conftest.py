from __future__ import annotations

import sys
from pathlib import Path

import numpy as np
import pandas as pd
import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from vegetation_ml import panel


def _season_days(year: int) -> pd.DatetimeIndex:
    return pd.date_range(f"{year}-04-01", f"{year}-10-30", freq="D")


@pytest.fixture
def synthetic_frame() -> pd.DataFrame:
    """Синтетический набор с известной сезонной кривой и тремя сенсорами.

    Сенсоры появляются строго на своих сетках витков и имеют заданные
    калибровочные смещения, поэтому по такому набору можно проверять и
    маскирование, и восстановление сеток, и оценку смещений.
    """
    rng = np.random.default_rng(0)
    rows = []
    for poly, phase in (("AOI-1", 0.0), ("AOI-2", 0.15), ("AOI-3", -0.08), ("AOI-4", 0.05)):
        for year in (2020, 2021, 2022):
            for d in _season_days(year):
                epoch = (d - pd.Timestamp("2010-01-01")).days
                doy = d.dayofyear
                truth = 0.25 + 0.45 * np.exp(-((doy - 180) / 45.0) ** 2) + phase
                row = {
                    "anon_polygon_id": poly, "date": d, "crop_type": "кукуруза",
                    "s2_ndvi": np.nan, "s2_evi": np.nan, "s2_ndwi": np.nan,
                    "landsat_ndvi": np.nan, "landsat_evi": np.nan, "landsat_ndwi": np.nan,
                    "modis_ndvi": np.nan, "modis_evi": np.nan,
                    "era5_temp_c": 15.0 + 8 * np.sin(doy / 58.0),
                    "era5_precip_mm": float(rng.gamma(1.0, 1.5)),
                    "year": year, "doy": doy,
                    "ndvi_climatology_mean": np.nan, "ndvi_climatology_std": np.nan,
                    "n_reference_years": 0,
                }
                if epoch % 5 == 2 and rng.random() < 0.5:
                    row["s2_ndvi"] = truth + rng.normal(0, 0.02)
                if epoch % 16 in (0, 8) and rng.random() < 0.7:
                    row["landsat_ndvi"] = truth + 0.04 + rng.normal(0, 0.02)
                if epoch % 8 == 3 and rng.random() < 0.6:
                    row["modis_ndvi"] = truth + 0.09 + rng.normal(0, 0.03)
                for col in ("s2_ndvi", "landsat_ndvi", "modis_ndvi"):
                    if pd.notna(row[col]):
                        row["primary_ndvi"] = row[col]
                        break
                else:
                    row["primary_ndvi"] = np.nan
                rows.append(row)
    df = pd.DataFrame(rows)
    return panel._add_time_keys(df)
