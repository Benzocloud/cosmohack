from __future__ import annotations

import copy

import numpy as np
import pandas as pd
import pytest
from fastapi.testclient import TestClient

from vegetation_ml import (
    contracts,
    masking,
    pipeline,
    service,
    web_analysis,
    web_features,
)
from vegetation_ml import model as M
from vegetation_ml import schema as S

HIDDEN_ROWS = 5


@pytest.fixture(scope="module")
def trained_model(request):
    """Небольшая модель, обученная на синтетическом наборе.

    Веб-путь расширенного профиля обязан вызывать ровно ту же модель, что и
    пакетный режим, поэтому в тестах она и обучается пакетным конвейером.
    """
    from conftest import synthetic_frame
    frame = synthetic_frame.__wrapped__()
    df = pipeline.sort_context(frame)
    grids = pipeline.fit_grids([df])
    parts_x, parts_y = [], []
    y = df[S.TARGET].to_numpy(dtype=float)
    for seed in (1, 2):
        mask = masking.simulate_gaps(df, seed=seed)
        _, X, _, _ = pipeline.build_matrices(df, mask, grids)
        parts_x.append(X[mask])
        parts_y.append(y[mask])
    return M.fit(pd.concat(parts_x, ignore_index=True), np.concatenate(parts_y),
                 params={"max_iter": 40})


@pytest.fixture
def model_client(monkeypatch, tmp_path, trained_model):
    """Сервис с загруженным артефактом: доступен расширенный профиль."""
    monkeypatch.setenv(service.MODEL_PATH_ENV, str(tmp_path / "absent.pkl"))
    with TestClient(service.app) as client:
        service._model["artifact"] = trained_model
        yield client
    service._model["artifact"] = None


@pytest.fixture
def bare_client(monkeypatch, tmp_path):
    """Сервис без артефакта: расширенный профиль не объявлен и не принимается."""
    monkeypatch.setenv(service.MODEL_PATH_ENV, str(tmp_path / "absent.pkl"))
    with TestClient(service.app) as client:
        yield client


@pytest.fixture(scope="module")
def synthetic(request):
    from conftest import synthetic_frame
    return pipeline.sort_context(synthetic_frame.__wrapped__())


def _clean(value):
    return None if value is None or (isinstance(value, float) and not np.isfinite(value)) else float(value)


def _indices(row):
    return {name: _clean(row.get(name)) for name in contracts.SENSOR_INDEX_FIELDS}


def _observation(row, hide: bool):
    """Точка расширенного профиля. hide превращает наблюдение в пропуск."""
    date_str = pd.Timestamp(row["date"]).date().isoformat()
    value = _clean(row[S.TARGET])
    if hide or value is None:
        return {
            "date": date_str, "primary_ndvi": None, "quality": "missing",
            "ndvi_source_id": None, "interval": None, "valid_fraction": None,
            "missing_reason": "cloud", "weather": _weather(row),
            "reference": None, "indices": None,
        }
    return {
        "date": date_str, "primary_ndvi": value, "quality": "usable",
        "ndvi_source_id": "sat-1",
        "interval": {"from": date_str, "to": date_str},
        "valid_fraction": 0.9, "missing_reason": None,
        "weather": _weather(row), "reference": None, "indices": _indices(row),
    }


def _weather(row):
    return {"source_id": "wx-1",
            "temperature_mean_c": _clean(row.get("era5_temp_c")),
            "precipitation_sum_mm": abs(_clean(row.get("era5_precip_mm")) or 0.0)}


def _peer(frame, area_id):
    rows = []
    for _, row in frame.iterrows():
        value = _clean(row[S.TARGET])
        rows.append({
            "date": pd.Timestamp(row["date"]).date().isoformat(),
            "primary_ndvi": value,
            "quality": "usable" if value is not None else "missing",
            "indices": _indices(row) if value is not None else None,
        })
    return {"area_id": area_id, "observations": rows}


def _sources():
    return [
        {"id": "sat-1", "kind": "satellite", "provider": "cdse", "dataset": "s2-l2a",
         "mapping": "mean по полигону", "retrieved_at": "2026-09-01T00:00:00Z",
         "license": None},
        {"id": "wx-1", "kind": "weather", "provider": "open-meteo", "dataset": "era5",
         "mapping": "ячейка реанализа", "retrieved_at": "2026-09-01T00:00:00Z",
         "license": None},
    ]


def build_multisensor_request(synthetic, with_peers=True, seasons=(2020, 2021, 2022)):
    """Запрос расширенного профиля и список скрытых дат с истинными значениями."""
    own = synthetic[(synthetic.anon_polygon_id == "AOI-1")
                    & synthetic.season.isin(seasons)].reset_index(drop=True)
    observed = own.index[own[S.TARGET].notna() & (own.season == max(seasons))]
    hidden = list(observed[10:10 + HIDDEN_ROWS])
    truth = {pd.Timestamp(own.loc[i, "date"]).date().isoformat(): float(own.loc[i, S.TARGET])
             for i in hidden}

    observations = [_observation(row, hide=i in hidden) for i, row in own.iterrows()]
    peers = None
    if with_peers:
        peers = [_peer(synthetic[(synthetic.anon_polygon_id == pid)
                                 & synthetic.season.isin(seasons)], pid)
                 for pid in ("AOI-2", "AOI-3", "AOI-4")]

    season_rows = own[own.season == max(seasons)]
    request = {
        "schema_version": "1.1",
        "request_id": "job-ms-1",
        "area_id": "AOI-1",
        "input_revision": "rev-1",
        "mode": "retrospective",
        "feature_profile": "ndvi-multisensor-v1",
        "analysis_period": {
            "from": pd.Timestamp(season_rows.date.iloc[0]).date().isoformat(),
            "to": pd.Timestamp(season_rows.date.iloc[-1]).date().isoformat(),
        },
        "sources": _sources(),
        "observations": observations,
        "area_context": {"crop_type": "кукуруза"},
        "peers": peers,
    }
    return request, truth


def test_readyz_advertises_extended_profile(model_client):
    body = model_client.get("/readyz").json()
    assert body["feature_profiles"] == ["ndvi-weather-v1", "ndvi-multisensor-v1"]
    assert body["model_version"] == web_analysis.MODEL_VERSION_TRAINED
    assert body["schema_versions"] == ["1.0", "1.1"]


def test_multisensor_request_uses_trained_model(model_client, synthetic):
    req, truth = build_multisensor_request(synthetic)
    response = model_client.post("/v1/analyze", json=req)
    assert response.status_code == 200, response.json()
    body = response.json()
    assert body["feature_profile"] == "ndvi-multisensor-v1"
    assert body["schema_version"] == "1.1"
    assert body["method"] == web_features.METHOD_MODEL
    assert body["model_version"] == web_analysis.MODEL_VERSION_TRAINED

    restored = {p["date"]: p for p in body["series"]
                if p["state"] == "imputed" and p["date"] in truth}
    assert len(restored) == len(truth)
    for point in restored.values():
        assert point["method"] == web_features.METHOD_MODEL
        assert -0.2 <= point["value"] <= 1.0


def test_restored_values_are_close_to_hidden_truth(model_client, synthetic):
    req, truth = build_multisensor_request(synthetic)
    body = model_client.post("/v1/analyze", json=req).json()
    errors = [abs(p["value"] - truth[p["date"]]) for p in body["series"]
              if p["date"] in truth and p["value"] is not None]
    assert len(errors) == len(truth)
    assert np.mean(errors) < 0.10


def test_missing_peers_are_reported_as_limitation(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic, with_peers=False)
    body = model_client.post("/v1/analyze", json=req).json()
    assert body["method"] == web_features.METHOD_MODEL
    assert any("Соседние участки не переданы" in x for x in body["limitations"])


def test_short_history_falls_back_and_says_so(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic, with_peers=False, seasons=(2022,))
    body = model_client.post("/v1/analyze", json=req).json()
    assert body["method"] != web_features.METHOD_MODEL
    assert any("резервный метод" in x for x in body["limitations"])


def test_unknown_crop_is_reported(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    req["area_context"]["crop_type"] = "марсианский картофель"
    body = model_client.post("/v1/analyze", json=req).json()
    assert any("не встречалась" in x for x in body["limitations"])


def test_extended_profile_requires_schema_11(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    req["schema_version"] = "1.0"
    body = model_client.post("/v1/analyze", json=req).json()
    assert body["error"]["code"] == "unsupported_contract"


def test_extended_profile_rejected_without_artifact(bare_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    body = bare_client.post("/v1/analyze", json=req).json()
    assert body["error"]["code"] == "unsupported_contract"


def test_indices_forbidden_in_base_profile(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    req["feature_profile"] = "ndvi-weather-v1"
    req["area_context"] = None
    req["peers"] = None
    response = model_client.post("/v1/analyze", json=req)
    assert response.status_code == 422
    assert "indices" in response.json()["error"]["message"]


def test_indices_must_match_primary_ndvi(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    for point in req["observations"]:
        if point["indices"] is not None:
            point["primary_ndvi"] = (point["primary_ndvi"] or 0.0) + 0.2
            break
    response = model_client.post("/v1/analyze", json=req)
    assert response.status_code == 422
    assert "не совпадает" in response.json()["error"]["message"]


def test_peer_cannot_be_the_area_itself(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    req["peers"][0]["area_id"] = req["area_id"]
    response = model_client.post("/v1/analyze", json=req)
    assert response.status_code == 422
    assert "сам участок" in response.json()["error"]["message"]


def test_duplicate_peer_ids_are_rejected(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    req["peers"][1]["area_id"] = req["peers"][0]["area_id"]
    response = model_client.post("/v1/analyze", json=req)
    assert response.status_code == 422
    assert "уникальны" in response.json()["error"]["message"]


def test_too_many_peers_are_rejected(model_client, synthetic):
    req, _ = build_multisensor_request(synthetic)
    peer = req["peers"][0]
    req["peers"] = []
    for i in range(contracts.MAX_PEERS + 1):
        clone = copy.deepcopy(peer)
        clone["area_id"] = f"peer-{i}"
        req["peers"].append(clone)
    assert model_client.post("/v1/analyze", json=req).status_code == 422


def test_base_profile_still_works_with_artifact_loaded(model_client):
    from test_service import _healthy_series, _request
    response = model_client.post("/v1/analyze", json=_request(_healthy_series()))
    assert response.status_code == 200
    body = response.json()
    assert body["feature_profile"] == "ndvi-weather-v1"
    assert body["method"] != web_features.METHOD_MODEL
    assert any("не содержит признаков по отдельным" in x for x in body["limitations"])
