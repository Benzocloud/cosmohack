from __future__ import annotations

import copy
import json
from datetime import date, timedelta

import pytest
from fastapi.testclient import TestClient

from vegetation_ml import contracts, service, web_analysis


@pytest.fixture
def client():
    with TestClient(service.app) as c:
        yield c


def _sources():
    return [
        {"id": "sat-1", "kind": "satellite", "provider": "cdse", "dataset": "s2-l2a",
         "mapping": "mean по полигону", "retrieved_at": "2026-09-01T00:00:00Z",
         "license": "CC-BY-4.0"},
        {"id": "wx-1", "kind": "weather", "provider": "open-meteo", "dataset": "era5",
         "mapping": "ячейка реанализа", "retrieved_at": "2026-09-01T00:00:00Z",
         "license": None},
    ]


def _point(d: date, ndvi, quality="usable", reason=None, temp=18.0, precip=1.0):
    numeric = ndvi is not None
    return {
        "date": d.isoformat(),
        "primary_ndvi": ndvi,
        "quality": quality,
        "ndvi_source_id": "sat-1" if numeric else None,
        "interval": {"from": d.isoformat(), "to": d.isoformat()} if numeric else None,
        "valid_fraction": 0.9 if numeric else None,
        "missing_reason": reason,
        "weather": {"source_id": "wx-1", "temperature_mean_c": temp,
                    "precipitation_sum_mm": precip},
        "reference": None,
    }


def _request(points, start=None, end=None):
    return {
        "schema_version": "1.0",
        "request_id": "job-1",
        "area_id": "area-1",
        "input_revision": "rev-1",
        "mode": "retrospective",
        "feature_profile": "ndvi-weather-v1",
        "analysis_period": {
            "from": start or points[0]["date"],
            "to": end or points[-1]["date"],
        },
        "sources": _sources(),
        "observations": points,
    }


def _healthy_series(n=30, gap_at=(10, 11)):
    d0 = date(2026, 6, 1)
    pts = []
    for i in range(n):
        d = d0 + timedelta(days=i)
        if i in gap_at:
            pts.append(_point(d, None, quality="missing", reason="cloud"))
        else:
            pts.append(_point(d, 0.60 + 0.001 * i))
    return pts


def test_readyz_reports_contract_and_model(client):
    r = client.get("/readyz")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ready"
    assert body["schema_version"] == "1.0"
    assert body["feature_profiles"] == ["ndvi-weather-v1"]
    assert body["model_version"] == web_analysis.MODEL_VERSION


def test_analyze_echoes_request_and_restores_gaps(client):
    r = client.post("/v1/analyze", json=_request(_healthy_series()))
    assert r.status_code == 200
    body = r.json()
    for field in ("request_id", "area_id", "input_revision", "mode", "feature_profile"):
        assert body[field] is not None
    assert body["request_id"] == "job-1"
    assert body["schema_version"] == "1.0"
    assert len(body["series"]) == 30
    states = [p["state"] for p in body["series"]]
    assert states[10] == "imputed" and states[11] == "imputed"
    assert states[0] == "observed"
    assert body["series"][10]["method"] is not None
    assert body["series"][10]["primary_ndvi"] is None


def test_observed_points_keep_their_original_value(client):
    pts = _healthy_series()
    r = client.post("/v1/analyze", json=_request(pts))
    body = r.json()
    for src, out in zip(pts, body["series"]):
        if src["quality"] == "usable":
            assert out["state"] == "observed"
            assert out["value"] == pytest.approx(src["primary_ndvi"])
            assert out["primary_ndvi"] == pytest.approx(src["primary_ndvi"])


def test_unusable_point_is_not_treated_as_observation(client):
    pts = _healthy_series(gap_at=())
    pts[10] = _point(date.fromisoformat(pts[10]["date"]), 0.05,
                     quality="unusable", reason="cloud_shadow")
    r = client.post("/v1/analyze", json=_request(pts))
    body = r.json()
    point = body["series"][10]
    assert point["state"] == "imputed"
    assert point["primary_ndvi"] == pytest.approx(0.05)
    assert point["value"] != pytest.approx(0.05)


def test_series_covers_only_analysis_period(client):
    pts = _healthy_series(n=40, gap_at=(20,))
    req = _request(pts, start=pts[10]["date"], end=pts[29]["date"])
    body = client.post("/v1/analyze", json=req).json()
    assert len(body["series"]) == 20
    assert body["series"][0]["date"] == pts[10]["date"]
    assert body["series"][-1]["date"] == pts[29]["date"]


def test_no_observations_gives_insufficient_data_not_error(client):
    d = date(2026, 6, 1)
    req = _request([_point(d, None, quality="missing", reason="no_usable_observation")])
    req["sources"] = []
    req["observations"][0]["weather"] = None
    r = client.post("/v1/analyze", json=req)
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "insufficient_data"
    assert body["severity"] is None
    assert body["method"] == web_analysis.METHOD_NONE
    assert body["series"][0]["state"] == "missing"
    assert body["series"][0]["value"] is None
    assert body["limitations"]


def test_unknown_field_is_rejected(client):
    req = _request(_healthy_series())
    req["unexpected"] = 1
    r = client.post("/v1/analyze", json=req)
    assert r.status_code == 422
    assert r.json()["error"]["code"] == "invalid_input"


def test_wrong_schema_version_is_unsupported_contract(client):
    req = _request(_healthy_series())
    req["schema_version"] = "2.0"
    r = client.post("/v1/analyze", json=req)
    assert r.status_code == 422
    body = r.json()
    assert body["error"]["code"] == "unsupported_contract"
    assert body["error"]["retryable"] is False
    assert body["request_id"] == "job-1"


def test_wrong_feature_profile_is_unsupported_contract(client):
    req = _request(_healthy_series())
    req["feature_profile"] = "ndvi-only-v9"
    assert client.post("/v1/analyze", json=req).json()["error"]["code"] == "unsupported_contract"


def test_duplicate_dates_are_rejected(client):
    pts = _healthy_series()
    pts[5] = copy.deepcopy(pts[4])
    r = client.post("/v1/analyze", json=_request(pts))
    assert r.status_code == 422
    assert "уникальны" in r.json()["error"]["message"]


def test_unsorted_dates_are_rejected(client):
    pts = _healthy_series()
    pts[4], pts[5] = pts[5], pts[4]
    r = client.post("/v1/analyze", json=_request(pts))
    assert r.status_code == 422
    assert r.json()["error"]["code"] == "invalid_input"


def test_unknown_source_reference_is_rejected(client):
    pts = _healthy_series()
    pts[3]["ndvi_source_id"] = "sat-missing"
    r = client.post("/v1/analyze", json=_request(pts))
    assert r.status_code == 422
    assert "satellite" in r.json()["error"]["message"]


def test_weather_source_must_be_weather_kind(client):
    pts = _healthy_series()
    pts[3]["weather"]["source_id"] = "sat-1"
    r = client.post("/v1/analyze", json=_request(pts))
    assert r.status_code == 422


def test_missing_point_requires_reason(client):
    pts = _healthy_series()
    pts[10]["missing_reason"] = None
    r = client.post("/v1/analyze", json=_request(pts))
    assert r.status_code == 422
    assert r.json()["error"]["code"] == "invalid_input"


def test_date_outside_its_interval_is_rejected(client):
    pts = _healthy_series()
    pts[3]["interval"] = {"from": "2026-01-01", "to": "2026-01-02"}
    r = client.post("/v1/analyze", json=_request(pts))
    assert r.status_code == 422
    assert "интервал" in r.json()["error"]["message"]


def test_negative_precipitation_is_rejected(client):
    pts = _healthy_series()
    pts[3]["weather"]["precipitation_sum_mm"] = -1.0
    assert client.post("/v1/analyze", json=_request(pts)).status_code == 422


def test_nan_literal_is_rejected(client):
    body = json.dumps(_request(_healthy_series())).replace('"primary_ndvi": 0.6,',
                                                           '"primary_ndvi": NaN,', 1)
    r = client.post("/v1/analyze", content=body.encode(),
                    headers={"content-type": "application/json"})
    assert r.status_code == 422
    assert r.json()["error"]["code"] == "invalid_input"


def test_invalid_json_is_400(client):
    r = client.post("/v1/analyze", content=b"{not json",
                    headers={"content-type": "application/json"})
    assert r.status_code == 400
    assert r.json()["error"]["code"] == "invalid_json"


def test_wrong_content_type_is_415(client):
    r = client.post("/v1/analyze", content=b"{}", headers={"content-type": "text/plain"})
    assert r.status_code == 415
    assert r.json()["error"]["code"] == "unsupported_media_type"


def test_oversized_body_is_413(client):
    big = b'{"padding": "' + b"x" * (service.MAX_REQUEST_BYTES + 10) + b'"}'
    r = client.post("/v1/analyze", content=big, headers={"content-type": "application/json"})
    assert r.status_code == 413
    assert r.json()["error"]["code"] == "payload_too_large"


def test_busy_slot_returns_429_and_is_retryable(client):
    assert service._slot.acquire(blocking=False)
    try:
        r = client.post("/v1/analyze", json=_request(_healthy_series()))
    finally:
        service._slot.release()
    assert r.status_code == 429
    body = r.json()
    assert body["error"]["code"] == "busy"
    assert body["error"]["retryable"] is True
    assert body["request_id"] == "job-1"


def test_slot_is_released_after_successful_request(client):
    client.post("/v1/analyze", json=_request(_healthy_series()))
    assert service._slot.acquire(blocking=False)
    service._slot.release()


def test_error_body_has_contract_shape(client):
    r = client.post("/v1/analyze", content=b"{not json",
                    headers={"content-type": "application/json"})
    body = r.json()
    assert set(body) == {"schema_version", "request_id", "error"}
    assert set(body["error"]) == {"code", "message", "retryable"}
    assert "Traceback" not in body["error"]["message"]


def test_declared_anomaly_period_is_reported_with_evidence(client):
    d0 = date(2026, 6, 1)
    pts = []
    for i in range(40):
        d = d0 + timedelta(days=i)
        value = 0.60 if not (12 <= i <= 22) else 0.30
        p = _point(d, value, precip=0.0 if 12 <= i <= 22 else 5.0)
        p["reference"] = {"source_id": "ref-1", "mean": 0.60, "std": 0.05,
                          "n_reference_years": 10, "target_year_excluded": True}
        pts.append(p)
    req = _request(pts)
    req["sources"].append({"id": "ref-1", "kind": "reference", "provider": "team",
                           "dataset": "own-climatology", "mapping": "по дню года",
                           "retrieved_at": "2026-09-01T00:00:00Z", "license": None})
    body = client.post("/v1/analyze", json=req).json()
    assert body["status"] == "confirmed"
    assert body["severity"] == "high"
    assert len(body["events"]) == 1
    ev = body["events"][0]
    assert ev["status"] == "confirmed"
    assert ev["evidence_dates"]
    assert ev["min_z_score"] < -2
    assert any("z-score" in f for f in ev["facts"])
    assert ev["limitations"]
