"""Схема данных соревнования: ключ, целевая переменная, состав колонок.

Порядок SENSOR_PRIORITY и периоды SENSOR_PERIOD не назначены произвольно, а
проверены на train: primary_ndvi всегда точно равен первому доступному сенсору
в этом порядке, а вне своей сетки остатков сенсор не появляется.
"""

from __future__ import annotations

KEY = ["anon_polygon_id", "date"]

TARGET = "primary_ndvi"

SENSOR_PRIORITY = ["s2_ndvi", "landsat_ndvi", "modis_ndvi"]
SENSOR_NAMES = ["s2", "landsat", "modis"]

SENSOR_PERIOD = {"s2_ndvi": 5, "landsat_ndvi": 16, "modis_ndvi": 8}

SPECTRAL_COLUMNS = [
    "s2_ndvi", "s2_evi", "s2_ndwi",
    "landsat_ndvi", "landsat_evi", "landsat_ndwi",
    "modis_ndvi", "modis_evi",
]
WEATHER_COLUMNS = ["era5_temp_c", "era5_precip_mm"]

DERIVED_COLUMNS = [
    "year", "doy", "ndvi_climatology_mean", "ndvi_climatology_std",
    "n_reference_years",
]

TRAIN_ONLY_COLUMNS = ["ndvi_zscore", "status"]

GAP_FLAG = "is_synthetic_gap"

MASKED_ON_GAP = SPECTRAL_COLUMNS + WEATHER_COLUMNS + DERIVED_COLUMNS + [TARGET]

EPOCH_ORIGIN = "2010-01-01"

SUBMISSION_COLUMNS = ["anon_polygon_id", "date", "primary_ndvi_pred"]

GAP_SCORE_RMSE_LIMIT = 0.10
GAP_SCORE_MAX = 30
